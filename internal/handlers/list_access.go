package handlers

import (
	"gorm.io/gorm"

	"task-tracker-api/internal/models"
)

// UserCanAccessList returns whether the user can access the list (owner or member), and whether they are the owner.
func UserCanAccessList(db *gorm.DB, listID, userID string) (ok bool, isOwner bool) {
	var list models.TaskList
	if err := db.Where("id = ?", listID).First(&list).Error; err != nil {
		return false, false
	}
	if list.UserID == userID {
		return true, true
	}
	var member models.ListMember
	if err := db.Where("list_id = ? AND user_id = ?", listID, userID).First(&member).Error; err != nil {
		return false, false
	}
	return true, false
}
