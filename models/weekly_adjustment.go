package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WeeklyAdjustment stores AI-driven weekly TDEE adjustments for a user.
type WeeklyAdjustment struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_week,where:deleted_at IS NULL" json:"user_id"`
	WeekStart     time.Time      `gorm:"type:date;not null;uniqueIndex:idx_user_week,where:deleted_at IS NULL" json:"week_start"`
	OldTDEE       int            `gorm:"type:int" json:"old_tdee"`
	NewTDEE       int            `gorm:"type:int" json:"new_tdee"`
	WorkoutVolume float64        `gorm:"type:decimal(8,2)" json:"workout_volume"`
	AIReasoning   string         `gorm:"type:text" json:"ai_reasoning"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (wa *WeeklyAdjustment) BeforeCreate(tx *gorm.DB) error {
	if wa.ID == uuid.Nil {
		wa.ID = uuid.New()
	}
	return nil
}
