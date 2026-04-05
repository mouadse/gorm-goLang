package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FavoriteFood represents a user's favorited food item.
type FavoriteFood struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_user_food_fav" json:"user_id"`
	FoodID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_user_food_fav" json:"food_id"`
	CreatedAt time.Time `json:"created_at"`

	Food Food `gorm:"foreignKey:FoodID" json:"food,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (f *FavoriteFood) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}
