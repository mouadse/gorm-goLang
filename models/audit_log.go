package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID         uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	AdminID    uuid.UUID       `gorm:"type:uuid;not null;index" json:"admin_id"`
	Action     string          `gorm:"type:varchar(100);not null" json:"action"`      // create_user, delete_user, etc.
	EntityType string          `gorm:"type:varchar(100);not null" json:"entity_type"` // user, exercise, etc.
	EntityID   uuid.UUID       `gorm:"type:uuid" json:"entity_id"`
	OldValue   json.RawMessage `gorm:"type:jsonb" json:"old_value"`
	NewValue   json.RawMessage `gorm:"type:jsonb" json:"new_value"`
	IPAddress  string          `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent  string          `gorm:"type:varchar(255)" json:"user_agent"`
	CreatedAt  time.Time       `json:"created_at"`
}
