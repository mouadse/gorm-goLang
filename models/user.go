package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a platform user with fitness profile data.
type User struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Email         string         `gorm:"type:varchar(255);uniqueIndex:idx_users_email,where:deleted_at IS NULL;not null" json:"email"`
	PasswordHash  string         `gorm:"type:varchar(255);not null" json:"-"`
	Name          string         `gorm:"type:varchar(255);not null" json:"name"`
	Avatar        string         `gorm:"type:varchar(512)" json:"avatar"`
	Age           int            `gorm:"type:int" json:"age"` // Deprecated: Use DateOfBirth instead
	DateOfBirth   *time.Time     `gorm:"type:date" json:"date_of_birth"`
	Weight        float64        `gorm:"type:decimal(7,2)" json:"weight"`
	Height        float64        `gorm:"type:decimal(5,2)" json:"height"`
	Goal          string         `gorm:"type:varchar(100)" json:"goal"`
	ActivityLevel string         `gorm:"type:varchar(50)" json:"activity_level"`
	TDEE          int            `gorm:"type:int" json:"tdee"`
	Role          string         `gorm:"type:varchar(50);default:'user';not null" json:"role"`
	AuthVersion   uint           `gorm:"type:integer;default:0" json:"-"`
	BannedAt      *time.Time     `gorm:"type:timestamp" json:"banned_at"`
	BanReason     string         `gorm:"type:text" json:"ban_reason"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Has-many relationships
	Workouts      []Workout     `gorm:"foreignKey:UserID" json:"workouts,omitempty"`
	Meals         []Meal        `gorm:"foreignKey:UserID" json:"meals,omitempty"`
	WeightEntries []WeightEntry `gorm:"foreignKey:UserID" json:"weight_entries,omitempty"`
}

// CalculateTDEE estimates the Total Daily Energy Expenditure using the Mifflin-St Jeor Equation.
// It uses a simplified approach for activity levels.
func (u *User) CalculateTDEE() int {
	if u.Weight == 0 || u.Height == 0 || u.DateOfBirth == nil {
		return u.TDEE
	}

	// Calculate Age
	now := time.Now().UTC()
	age := now.Year() - u.DateOfBirth.Year()
	if now.Month() < u.DateOfBirth.Month() || (now.Month() == u.DateOfBirth.Month() && now.Day() < u.DateOfBirth.Day()) {
		age--
	}

	// BMR = 10 * weight (kg) + 6.25 * height (cm) - 5 * age (y) + s
	// s is +5 for males and -161 for females.
	// Since we don't have gender in the model, we'll use a neutral average or assume one.
	// Let's assume +5 for now or just average.
	bmr := 10*u.Weight + 6.25*u.Height - 5*float64(age) + 5

	// Activity multipliers
	multipliers := map[string]float64{
		"sedentary":         1.2,
		"lightly_active":    1.375,
		"moderately_active": 1.55,
		"active":            1.725,
		"very_active":       1.9,
	}

	multiplier, ok := multipliers[strings.ToLower(u.ActivityLevel)]
	if !ok {
		multiplier = 1.2 // default to sedentary
	}

	return int(bmr * multiplier)
}

// BeforeCreate sets a new UUID before inserting.
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
