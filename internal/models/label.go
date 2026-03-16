package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Label struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string    `gorm:"type:uuid;index;uniqueIndex:idx_user_label_slug" json:"user_id"` // nullable for migration; seed backfills to admin
	Slug      string    `gorm:"not null;uniqueIndex:idx_user_label_slug" json:"slug"`
	Name      string    `gorm:"not null" json:"name"`
	Color     string    `gorm:"not null" json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (l *Label) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
