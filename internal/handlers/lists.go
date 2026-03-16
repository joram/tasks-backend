package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-tracker-api/internal/models"
)

type ListsHandler struct {
	db *gorm.DB
}

func NewListsHandler(db *gorm.DB) *ListsHandler {
	return &ListsHandler{db: db}
}

// listWithRole is TaskList plus role for API response.
type listWithRole struct {
	models.TaskList
	Role string `json:"role"`
}

func (h *ListsHandler) List(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	// Owned lists
	var owned []models.TaskList
	if err := h.db.Where("user_id = ?", userID).Order("created_at asc").Find(&owned).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch lists"})
		return
	}
	out := make([]listWithRole, 0, len(owned))
	for _, l := range owned {
		out = append(out, listWithRole{TaskList: l, Role: models.ListRoleOwner})
	}
	// Shared lists (where user is member)
	var memberRows []models.ListMember
	if err := h.db.Where("user_id = ?", userID).Find(&memberRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch lists"})
		return
	}
	for _, m := range memberRows {
		var list models.TaskList
		if err := h.db.Where("id = ?", m.ListID).First(&list).Error; err != nil {
			continue
		}
		out = append(out, listWithRole{TaskList: list, Role: models.ListRoleMember})
	}
	c.JSON(http.StatusOK, out)
}

func (h *ListsHandler) Create(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	list := models.TaskList{
		UserID: userID,
		Name:   body.Name,
	}
	if err := h.db.Create(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create list"})
		return
	}
	c.JSON(http.StatusCreated, listWithRole{TaskList: list, Role: models.ListRoleOwner})
}

func (h *ListsHandler) Update(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	listID := c.Param("id")
	canAccess, isOwner := UserCanAccessList(h.db, listID, userID)
	if !canAccess {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}
	if !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the list owner can update it"})
		return
	}
	var list models.TaskList
	if err := h.db.Where("id = ?", listID).First(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch list"})
		return
	}
	var body struct {
		Name *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Name != nil {
		list.Name = *body.Name
		if err := h.db.Model(&list).Update("name", list.Name).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update list"})
			return
		}
	}
	c.JSON(http.StatusOK, list)
}

func (h *ListsHandler) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	listID := c.Param("id")
	canAccess, isOwner := UserCanAccessList(h.db, listID, userID)
	if !canAccess {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}
	if !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the list owner can delete it"})
		return
	}
	// Remove members first, then list
	h.db.Where("list_id = ?", listID).Delete(&models.ListMember{})
	res := h.db.Where("id = ?", listID).Delete(&models.TaskList{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete list"})
		return
	}
	c.Status(http.StatusNoContent)
}

type addListMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *ListsHandler) AddMember(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	listID := c.Param("id")
	canAccess, isOwner := UserCanAccessList(h.db, listID, userID)
	if !canAccess {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}
	if !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the list owner can add members"})
		return
	}
	var body addListMemberRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))
	var target models.User
	if err := h.db.Where("email = ?", email).First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "no user with that email"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to look up user"})
		}
		return
	}
	if target.ID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "you are already the owner"})
		return
	}
	var existing models.ListMember
	if err := h.db.Where("list_id = ? AND user_id = ?", listID, target.ID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "user already has access to this list"})
		return
	}
	member := models.ListMember{ListID: listID, UserID: target.ID}
	if err := h.db.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add member"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user_id": target.ID, "email": target.Email})
}

func (h *ListsHandler) ListMembers(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	listID := c.Param("id")
	canAccess, isOwner := UserCanAccessList(h.db, listID, userID)
	if !canAccess {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}
	if !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the list owner can view members"})
		return
	}
	var members []models.ListMember
	if err := h.db.Where("list_id = ?", listID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch members"})
		return
	}
	type memberInfo struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
	}
	infos := make([]memberInfo, 0, len(members))
	for _, m := range members {
		var u models.User
		if err := h.db.Where("id = ?", m.UserID).First(&u).Error; err != nil {
			continue
		}
		infos = append(infos, memberInfo{UserID: u.ID, Email: u.Email})
	}
	c.JSON(http.StatusOK, infos)
}

func (h *ListsHandler) RemoveMember(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	listID := c.Param("id")
	memberUserID := c.Param("user_id")
	canAccess, isOwner := UserCanAccessList(h.db, listID, userID)
	if !canAccess {
		c.JSON(http.StatusNotFound, gin.H{"error": "list not found"})
		return
	}
	if !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the list owner can remove members"})
		return
	}
	res := h.db.Where("list_id = ? AND user_id = ?", listID, memberUserID).Delete(&models.ListMember{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
