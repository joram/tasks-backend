package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-tracker-api/internal/models"
)

// LabelResponse is the shape returned by GET /labels and create/update.
type LabelResponse struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// slug must be lowercase alphanumeric and underscores
var slugRegex = regexp.MustCompile(`^[a-z0-9_]+$`)

type LabelsHandler struct {
	db *gorm.DB
}

func NewLabelsHandler(db *gorm.DB) *LabelsHandler {
	return &LabelsHandler{db: db}
}

func (h *LabelsHandler) List(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var labels []models.Label
	if err := h.db.Where("user_id = ?", userID).Order("slug asc").Find(&labels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch labels"})
		return
	}
	out := make([]LabelResponse, 0, len(labels))
	for _, l := range labels {
		out = append(out, labelToResponse(l))
	}
	c.JSON(http.StatusOK, out)
}

func labelToResponse(l models.Label) LabelResponse {
	return LabelResponse{
		ID:    l.ID,
		Slug:  l.Slug,
		Name:  l.Name,
		Color:  l.Color,
		CreatedAt: l.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: l.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *LabelsHandler) Create(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var body struct {
		Slug  string `json:"slug" binding:"required"`
		Name  string `json:"name" binding:"required"`
		Color string `json:"color" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	slug := strings.TrimSpace(strings.ToLower(body.Slug))
	if !slugRegex.MatchString(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug must be lowercase letters, numbers, and underscores only"})
		return
	}
	var existing models.Label
	if err := h.db.Where("user_id = ? AND slug = ?", userID, slug).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a label with that slug already exists"})
		return
	}
	label := models.Label{
		UserID: userID,
		Slug:   slug,
		Name:   strings.TrimSpace(body.Name),
		Color:  strings.TrimSpace(body.Color),
	}
	if label.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if err := h.db.Create(&label).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create label"})
		return
	}
	c.JSON(http.StatusCreated, labelToResponse(label))
}

func (h *LabelsHandler) Update(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var label models.Label
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&label).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "label not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch label"})
		}
		return
	}
	var body struct {
		Slug  *string `json:"slug"`
		Name  *string `json:"name"`
		Color *string `json:"color"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Slug != nil {
		slug := strings.TrimSpace(strings.ToLower(*body.Slug))
		if !slugRegex.MatchString(slug) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "slug must be lowercase letters, numbers, and underscores only"})
			return
		}
		var existing models.Label
		if err := h.db.Where("user_id = ? AND slug = ? AND id != ?", userID, slug, id).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "a label with that slug already exists"})
			return
		}
		label.Slug = slug
	}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
			return
		}
		label.Name = name
	}
	if body.Color != nil {
		label.Color = strings.TrimSpace(*body.Color)
	}
	if err := h.db.Save(&label).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update label"})
		return
	}
	c.JSON(http.StatusOK, labelToResponse(label))
}

func (h *LabelsHandler) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var label models.Label
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&label).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "label not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch label"})
		}
		return
	}
	// Clear label from any tasks that use it
	if err := h.db.Model(&models.Task{}).Where("label = ?", label.Slug).Update("label", nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear label from tasks"})
		return
	}
	if err := h.db.Delete(&label).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete label"})
		return
	}
	c.Status(http.StatusNoContent)
}

// LabelExists returns true if a label with the given slug exists and is owned by the user.
func LabelExists(db *gorm.DB, slug, userID string) bool {
	if slug == "" {
		return true
	}
	var count int64
	db.Model(&models.Label{}).Where("slug = ? AND user_id = ?", slug, userID).Count(&count)
	return count > 0
}
