package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Food represents a food item in the database with its nutritional information.
type Food struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Name          string         `gorm:"type:varchar(255);not null;index" json:"name"`
	Brand         string         `gorm:"type:varchar(255)" json:"brand"`
	ServingSize   float64        `gorm:"type:decimal(10,2);not null" json:"serving_size"`   // e.g. 100
	ServingUnit   string         `gorm:"type:varchar(50);not null" json:"serving_unit"`   // e.g. g, ml, oz
	Calories      float64        `gorm:"type:decimal(10,2);not null" json:"calories"`      // per serving
	Protein       float64        `gorm:"type:decimal(10,2);not null" json:"protein"`       // per serving
	Carbohydrates float64        `gorm:"type:decimal(10,2);not null" json:"carbohydrates"` // per serving
	Fat           float64        `gorm:"type:decimal(10,2);not null" json:"fat"`           // per serving
	Fiber         float64        `gorm:"type:decimal(10,2)" json:"fiber"`                  // optional
	Sugar         float64        `gorm:"type:decimal(10,2)" json:"sugar"`                  // optional
	Sodium        float64        `gorm:"type:decimal(10,2)" json:"sodium"`                 // optional
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate sets a new UUID before inserting.
func (f *Food) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}
