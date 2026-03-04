package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Friendship represents a connection between two users.
type Friendship struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_friend,where:deleted_at IS NULL" json:"user_id"`
	FriendID    uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_friend,where:deleted_at IS NULL;check:friend_id <> user_id" json:"friend_id"`
	RequesterID uuid.UUID      `gorm:"type:uuid;not null;index;check:requester_id IN (user_id, friend_id)" json:"requester_id"`
	Status      string         `gorm:"type:varchar(50);not null;default:'pending'" json:"status"` // pending/accepted/blocked
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	User      User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Friend    User `gorm:"foreignKey:FriendID" json:"friend,omitempty"`
	Requester User `gorm:"foreignKey:RequesterID" json:"requester,omitempty"`
}

// BeforeCreate sets a new UUID before inserting and validates no self-friendship.
func (f *Friendship) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.UserID == uuid.Nil || f.FriendID == uuid.Nil {
		return errors.New("user_id and friend_id must be set")
	}
	if f.UserID == f.FriendID {
		return errors.New("user cannot be friends with themselves")
	}
	if f.RequesterID == uuid.Nil {
		f.RequesterID = f.UserID
	}
	if f.RequesterID != f.UserID && f.RequesterID != f.FriendID {
		return errors.New("requester_id must match user_id or friend_id")
	}

	// Canonicalize pair ordering so (A,B) and (B,A) collapse to one unique pair.
	if f.UserID.String() > f.FriendID.String() {
		f.UserID, f.FriendID = f.FriendID, f.UserID
	}

	return nil
}
