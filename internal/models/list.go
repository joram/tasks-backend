package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskList is a list owned by a user; tasks belong to a list. Can be shared via ListMember.
type TaskList struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string    `gorm:"type:uuid;not null;index" json:"user_id"`
	Name      string    `gorm:"not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Role is the user's relationship to a list in list responses.
const (
	ListRoleOwner  = "owner"
	ListRoleMember = "member"
)

func (l *TaskList) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
