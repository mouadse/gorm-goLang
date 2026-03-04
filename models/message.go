package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Message represents a direct message between two users.
type Message struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SenderID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"sender_id"`
	ReceiverID uuid.UUID      `gorm:"type:uuid;not null;index" json:"receiver_id"`
	Content    string         `gorm:"type:text;not null" json:"content"`
	Timestamp  time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"timestamp"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	Sender   User `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
	Receiver User `gorm:"foreignKey:ReceiverID" json:"receiver,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
