package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Conversation struct {
	ID        uuid.UUID             `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID             `gorm:"type:uuid;not null;index:idx_conversations_user_updated,priority:1" json:"user_id"`
	User      User                  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Title     string                `gorm:"type:varchar(255)" json:"title"`
	Messages  []ConversationMessage `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE" json:"messages"`
	CreatedAt time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time             `gorm:"autoUpdateTime;index:idx_conversations_user_updated,priority:2,sort:desc" json:"updated_at"`
}

func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

type ConversationMessage struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null;index;index:idx_conversation_messages_conversation_sequence,priority:1" json:"conversation_id"`
	Role           string    `gorm:"type:varchar(50);not null" json:"role"` // "user", "assistant", "system", "tool"
	Content        string    `gorm:"type:text" json:"content"`
	ToolCalls      *string   `gorm:"type:text" json:"tool_calls,omitempty"`           // JSON serialized array of services.ToolCall
	ToolCallID     *string   `gorm:"type:varchar(255)" json:"tool_call_id,omitempty"` // For "tool" role and "assistant" tool calls
	ToolName       *string   `gorm:"type:varchar(255)" json:"tool_name,omitempty"`    // For "tool" role
	ToolArgs       *string   `gorm:"type:text" json:"tool_args,omitempty"`            // Raw JSON args if assistant called a tool
	Feedback       *int      `gorm:"type:smallint" json:"feedback,omitempty"`         // 1 for thumbs up, -1 for down
	Sequence       int64     `gorm:"not null;default:0;index:idx_conversation_messages_conversation_sequence,priority:2" json:"-"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (cm *ConversationMessage) BeforeCreate(tx *gorm.DB) error {
	if cm.ID == uuid.Nil {
		cm.ID = uuid.New()
	}
	return nil
}
