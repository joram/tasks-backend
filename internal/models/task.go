package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

// TaskLabel is the slug of a label (stored on Task); validity is checked against the Label table.
type TaskLabel string

type Task struct {
	ID          string     `gorm:"type:uuid;primaryKey" json:"id"`
	ListID      *string    `gorm:"type:uuid;index" json:"list_id"`
	ParentID    *string    `gorm:"type:uuid;index" json:"parent_id"` // subtask of this task; null = top-level
	Title       string     `gorm:"not null" json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `gorm:"type:text;not null;default:'pending'" json:"status"`
	Label       *TaskLabel `gorm:"type:text" json:"label"`
	DueDate     *time.Time `json:"due_date"`
	SortOrder   int        `gorm:"not null;default:0" json:"sort_order"`
	ArchivedAt  *time.Time `json:"archived_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Status == "" {
		t.Status = StatusPending
	}
	return nil
}
