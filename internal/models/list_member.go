package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListMember grants a user access to a list they don't own. Only the list owner can add/remove members.
type ListMember struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	ListID    string    `gorm:"type:uuid;not null;uniqueIndex:idx_list_member" json:"list_id"`
	UserID    string    `gorm:"type:uuid;not null;uniqueIndex:idx_list_member" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (m *ListMember) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
