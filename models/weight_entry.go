package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WeightEntry tracks a user's body weight over time.
type WeightEntry struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_date,where:deleted_at IS NULL" json:"user_id"`
	Weight    float64        `gorm:"type:decimal(7,2);not null" json:"weight"`
	Date      time.Time      `gorm:"type:date;not null;uniqueIndex:idx_user_date,where:deleted_at IS NULL" json:"date"`
	Notes     string         `gorm:"type:text" json:"notes"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (we *WeightEntry) BeforeCreate(tx *gorm.DB) error {
	if we.ID == uuid.Nil {
		we.ID = uuid.New()
	}
	return nil
}
