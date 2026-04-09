package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	PillarTraining    = "training"
	PillarNutrition   = "nutrition"
	PillarConsistency = "consistency"
)

// UserPointsLog records gamification points earned by a user.
type UserPointsLog struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null;index:idx_user_points_user_date,priority:1;index:idx_user_points_user_pillar,priority:1" json:"user_id"`
	Points         int            `gorm:"type:int;not null" json:"points"`
	Reason         string         `gorm:"type:varchar(255);not null" json:"reason"` // E.g., "Workout logged (>=15m)"
	ReasonCode     string         `gorm:"type:varchar(50);not null" json:"reason_code"` // E.g., "T1", "N1"
	Pillar         string         `gorm:"type:varchar(50);not null;index:idx_user_points_user_pillar,priority:2" json:"pillar"` // training, nutrition, consistency
	SourceEntityID *uuid.UUID     `gorm:"type:uuid;index" json:"source_entity_id"` // Optional link to Workout/Meal/etc
	EarnedAt       time.Time      `gorm:"type:timestamp;not null;index:idx_user_points_user_date,priority:2" json:"earned_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

// BeforeCreate sets a new UUID before inserting.
func (u *UserPointsLog) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.EarnedAt.IsZero() {
		u.EarnedAt = time.Now()
	}
	return nil
}
