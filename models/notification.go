package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Notification represents a notification sent to a user.
type Notification struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Type       string         `gorm:"type:varchar(100);not null" json:"type"`
	Content    string         `gorm:"type:text" json:"content"`
	ReadStatus bool           `gorm:"type:boolean;default:false" json:"read_status"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}
