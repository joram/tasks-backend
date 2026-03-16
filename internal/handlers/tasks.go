package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-tracker-api/internal/models"
)

type TaskHandler struct {
	db *gorm.DB
}

func NewTaskHandler(db *gorm.DB) *TaskHandler {
	return &TaskHandler{db: db}
}

func (h *TaskHandler) List(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	listID := c.Query("list_id")
	if listID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "list_id is required"})
		return
	}
	if ok, _ := UserCanAccessList(h.db, listID, userID); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "list not found"})
		return
	}
	var tasks []models.Task
	q := h.db.Where("list_id = ?", listID)
	if c.Query("archived") == "true" {
		q = q.Where("archived_at IS NOT NULL").Order("archived_at desc")
	} else {
		q = q.Where("archived_at IS NULL").Order("sort_order asc, created_at asc")
	}
	if label := c.Query("label"); label != "" {
		if LabelExists(h.db, label, userID) {
			q = q.Where("label = ?", label)
		}
	}
	if err := q.Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tasks"})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *TaskHandler) Create(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var task models.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		log.Printf("tasks.Create: bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if task.ListID == nil || *task.ListID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "list_id is required"})
		return
	}
	if task.Label != nil && *task.Label != "" && !LabelExists(h.db, string(*task.Label), userID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid label"})
		return
	}
	if ok, _ := UserCanAccessList(h.db, *task.ListID, userID); !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "list not found"})
		return
	}
	if task.ParentID != nil && *task.ParentID != "" {
		if !h.validParentInList(*task.ParentID, *task.ListID, userID, "") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parent task"})
			return
		}
	}
	if task.DueDate == nil {
		d := time.Now().AddDate(0, 0, 7)
		task.DueDate = &d
	}
	task.ArchivedAt = nil
	if err := h.db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) Get(c *gin.Context) {
	task, ok := h.findByID(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Update(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	task, ok := h.findByID(c)
	if !ok {
		return
	}

	var body struct {
		Title       *string            `json:"title"`
		Description *string            `json:"description"`
		Status      *models.TaskStatus `json:"status"`
		Label       *models.TaskLabel  `json:"label"`
		DueDate     *time.Time         `json:"due_date"`
		SortOrder   *int               `json:"sort_order"`
		ParentID    *string            `json:"parent_id"`
		// Archive / unarchive: send archived_at as a timestamp to archive, null to unarchive.
		// Use the dedicated /tasks/:id/archive and /tasks/:id/unarchive endpoints for clarity.
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if body.Title != nil {
		updates["title"] = *body.Title
	}
	if body.Description != nil {
		updates["description"] = *body.Description
	}
	if body.Status != nil {
		updates["status"] = *body.Status
	}
	if body.Label != nil {
		if *body.Label != "" && !LabelExists(h.db, string(*body.Label), userID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid label"})
			return
		}
		updates["label"] = *body.Label
	}
	if body.DueDate != nil {
		updates["due_date"] = *body.DueDate
	}
	if body.SortOrder != nil {
		updates["sort_order"] = *body.SortOrder
	}
	if body.ParentID != nil {
		if *body.ParentID != "" {
			if task.ListID == nil || !h.validParentInList(*body.ParentID, *task.ListID, userID, task.ID) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parent task"})
				return
			}
			updates["parent_id"] = *body.ParentID
		} else {
			updates["parent_id"] = nil
		}
	}

	if err := h.db.Model(task).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update task"})
		return
	}

	// Re-fetch to return current DB state
	h.db.First(task, "id = ?", task.ID)
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	task, ok := h.findByID(c)
	if !ok {
		return
	}
	if err := h.db.Delete(task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete task"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) Archive(c *gin.Context) {
	task, ok := h.findByID(c)
	if !ok {
		return
	}
	now := time.Now()
	if err := h.db.Model(task).Update("archived_at", now).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to archive task"})
		return
	}
	task.ArchivedAt = &now
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Unarchive(c *gin.Context) {
	task, ok := h.findByID(c)
	if !ok {
		return
	}
	if err := h.db.Model(task).Update("archived_at", nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unarchive task"})
		return
	}
	task.ArchivedAt = nil
	c.JSON(http.StatusOK, task)
}

type reorderItem struct {
	ID        string  `json:"id" binding:"required"`
	SortOrder int     `json:"sort_order"`
	ParentID  *string `json:"parent_id"` // optional; set for indent/outdent
}

func (h *TaskHandler) Reorder(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var body []reorderItem
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for _, item := range body {
		var task models.Task
		if err := h.db.First(&task, "id = ?", item.ID).Error; err != nil || task.ListID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "task not found"})
			return
		}
		if ok, _ := UserCanAccessList(h.db, *task.ListID, userID); !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "task not found"})
			return
		}
		updates := map[string]interface{}{"sort_order": item.SortOrder}
		if item.ParentID != nil {
			if *item.ParentID == "" {
				updates["parent_id"] = nil
			} else if h.validParentInList(*item.ParentID, *task.ListID, userID, task.ID) {
				updates["parent_id"] = *item.ParentID
			}
		}
		if err := h.db.Model(&task).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder tasks"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// validParentInList returns true if parentID is a task in the same list, user has access, and parent is not self or a descendant of self (no cycles).
func (h *TaskHandler) validParentInList(parentID, listID, userID, excludeTaskID string) bool {
	if parentID == excludeTaskID {
		return false
	}
	var parent models.Task
	if err := h.db.Where("id = ? AND list_id = ?", parentID, listID).First(&parent).Error; err != nil {
		return false
	}
	if ok, _ := UserCanAccessList(h.db, listID, userID); !ok {
		return false
	}
	// Avoid cycle: parent must not be a descendant of excludeTaskID (the task we're moving)
	cur := parent.ParentID
	for cur != nil && *cur != "" {
		if *cur == excludeTaskID {
			return false
		}
		var t models.Task
		if err := h.db.Select("parent_id").Where("id = ?", *cur).First(&t).Error; err != nil {
			break
		}
		cur = t.ParentID
	}
	return true
}

func (h *TaskHandler) findByID(c *gin.Context) (*models.Task, bool) {
	userID, ok := getUserID(c)
	if !ok {
		return nil, false
	}
	var task models.Task
	err := h.db.First(&task, "id = ?", c.Param("id")).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch task"})
		}
		return nil, false
	}
	if task.ListID == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return nil, false
	}
	if ok, _ := UserCanAccessList(h.db, *task.ListID, userID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return nil, false
	}
	return &task, true
}
