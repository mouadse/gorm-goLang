package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FoodImportLog struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	AdminID       uuid.UUID `gorm:"type:uuid;not null;index" json:"admin_id"`
	Source        string    `gorm:"type:varchar(50);not null;default:'usda'" json:"source"`
	FdcID         *int      `gorm:"index" json:"fdc_id,omitempty"`
	Status        string    `gorm:"type:varchar(20);not null" json:"status"` // success, failed, duplicate
	ErrorMessage  string    `gorm:"type:text" json:"error_message,omitempty"`
	FoodsImported int       `gorm:"default:0" json:"foods_imported"`
	DurationMs    int64     `json:"duration_ms"`
	CreatedAt     time.Time `json:"created_at"`
}

// BeforeCreate sets a new UUID before inserting.
func (f *FoodImportLog) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}
