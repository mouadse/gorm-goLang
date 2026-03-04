package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a platform user with fitness profile data.
type User struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
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
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Has-many relationships
	Workouts          []Workout          `gorm:"foreignKey:UserID" json:"workouts,omitempty"`
	Meals             []Meal             `gorm:"foreignKey:UserID" json:"meals,omitempty"`
	WeightEntries     []WeightEntry      `gorm:"foreignKey:UserID" json:"weight_entries,omitempty"`
	Friendships       []Friendship       `gorm:"foreignKey:UserID" json:"friendships,omitempty"`
	SentMessages      []Message          `gorm:"foreignKey:SenderID" json:"sent_messages,omitempty"`
	ReceivedMessages  []Message          `gorm:"foreignKey:ReceiverID" json:"received_messages,omitempty"`
	Notifications     []Notification     `gorm:"foreignKey:UserID" json:"notifications,omitempty"`
	WeeklyAdjustments []WeeklyAdjustment `gorm:"foreignKey:UserID" json:"weekly_adjustments,omitempty"`
	WorkoutPrograms   []WorkoutProgram   `gorm:"foreignKey:CreatedBy" json:"workout_programs,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
