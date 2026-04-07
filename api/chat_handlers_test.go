package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fitness-tracker/models"
	"fitness-tracker/services"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type MockLLMClient struct {
	Response *services.LLMMessage
	Err      error
}

func (m *MockLLMClient) Chat(messages []services.LLMMessage, tools []services.ToolDef) (*services.LLMMessage, error) {
	return m.Response, m.Err
}

func TestChatHandlers(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.User{}, &models.Conversation{}, &models.ConversationMessage{})

	userID := uuid.New()
	user := models.User{ID: userID, Email: "test@example.com", Name: "Test User"}
	db.Create(&user)

	mockLLM := &MockLLMClient{
		Response: &services.LLMMessage{
			Role:    "assistant",
			Content: "Hello! I am your AI coach.",
		},
	}

	server := NewServer(db)
	server.llmClient = mockLLM

	t.Run("Create new conversation and chat", func(t *testing.T) {
		reqBody, _ := json.Marshal(ChatRequest{
			Message: "Hi coach!",
		})
		req := httptest.NewRequest("POST", "/v1/chat", bytes.NewBuffer(reqBody))
		
		// Add user ID to context
		ctx := context.WithValue(req.Context(), authenticatedUserIDKey, userID)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(server.handleChat)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d. body: %s", rr.Code, rr.Body.String())
		}

		var resp ChatResponse
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp.Message != "Hello! I am your AI coach." {
			t.Errorf("expected coach greeting, got %s", resp.Message)
		}
		if resp.ConversationID == uuid.Nil {
			t.Error("expected conversation ID to be returned")
		}

		// Verify conversation and message were saved
		var conv models.Conversation
		if err := db.First(&conv, "id = ?", resp.ConversationID).Error; err != nil {
			t.Errorf("conversation not found in db: %v", err)
		}
		var msgCount int64
		db.Model(&models.ConversationMessage{}).Where("conversation_id = ?", conv.ID).Count(&msgCount)
		if msgCount != 2 { // user message + assistant response
			t.Errorf("expected 2 messages in db, got %d", msgCount)
		}
	})
}
