package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type MockLLMClient struct {
	Responses          []*services.LLMMessage
	Err                error
	CallCount          int
	ReceivedMessages   [][]services.LLMMessage
	ReceivedToolCounts []int
}

func (m *MockLLMClient) Chat(messages []services.LLMMessage, tools []services.ToolDef) (*services.LLMMessage, error) {
	msgCopy := make([]services.LLMMessage, len(messages))
	copy(msgCopy, messages)
	m.ReceivedMessages = append(m.ReceivedMessages, msgCopy)
	m.ReceivedToolCounts = append(m.ReceivedToolCounts, len(tools))
	if m.Err != nil {
		return nil, m.Err
	}
	if len(m.Responses) == 0 {
		return nil, nil
	}

	idx := m.CallCount
	if idx >= len(m.Responses) {
		idx = len(m.Responses) - 1
	}
	m.CallCount++
	return m.Responses[idx], nil
}

func TestChatHandlers(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.User{}, &models.Conversation{}, &models.ConversationMessage{})

	userID := uuid.New()
	user := models.User{ID: userID, Email: "test@example.com", Name: "Test User"}
	db.Create(&user)

	mockLLM := &MockLLMClient{
		Responses: []*services.LLMMessage{
			{
				Role:    "assistant",
				Content: "Hello! I am your AI coach.",
			},
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

	t.Run("Handles multiple tool-calling rounds", func(t *testing.T) {
		mockLLM.Responses = []*services.LLMMessage{
			{
				Role: "assistant",
				ToolCalls: []*services.ToolCall{
					{
						ID:   "call-1",
						Type: "function",
						Function: services.ToolFunction{
							Name:      "unknown_tool_one",
							Arguments: "{}",
						},
					},
				},
			},
			{
				Role: "assistant",
				ToolCalls: []*services.ToolCall{
					{
						ID:   "call-2",
						Type: "function",
						Function: services.ToolFunction{
							Name:      "unknown_tool_two",
							Arguments: "{\"date\":\"2026-04-15\"}",
						},
					},
				},
			},
			{
				Role:    "assistant",
				Content: "Focus on consistency and keep protein high today.",
			},
		}
		mockLLM.CallCount = 0
		mockLLM.ReceivedMessages = nil
		mockLLM.ReceivedToolCounts = nil

		reqBody, _ := json.Marshal(ChatRequest{
			Message: "How am I doing today?",
		})
		req := httptest.NewRequest("POST", "/v1/chat", bytes.NewBuffer(reqBody))
		ctx := context.WithValue(req.Context(), authenticatedUserIDKey, userID)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		http.HandlerFunc(server.handleChat).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. body: %s", rr.Code, rr.Body.String())
		}

		var resp ChatResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Message != "Focus on consistency and keep protein high today." {
			t.Fatalf("expected final assistant answer, got %q", resp.Message)
		}
		if mockLLM.CallCount != 3 {
			t.Fatalf("expected 3 LLM calls, got %d", mockLLM.CallCount)
		}
		if len(mockLLM.ReceivedToolCounts) != 3 {
			t.Fatalf("expected tool counts for 3 calls, got %d", len(mockLLM.ReceivedToolCounts))
		}
		if mockLLM.ReceivedToolCounts[1] == 0 || mockLLM.ReceivedToolCounts[2] == 0 {
			t.Fatalf("expected follow-up LLM calls to keep tools enabled, got %v", mockLLM.ReceivedToolCounts)
		}

		var toolMsgCount int64
		db.Model(&models.ConversationMessage{}).Where("role = ?", "tool").Count(&toolMsgCount)
		if toolMsgCount < 2 {
			t.Fatalf("expected tool responses to be persisted, got %d", toolMsgCount)
		}
	})

	t.Run("Replays tool transcripts in saved order on later turns", func(t *testing.T) {
		mockLLM.Responses = []*services.LLMMessage{
			{
				Role: "assistant",
				ToolCalls: []*services.ToolCall{
					{
						ID:   "call-ordered",
						Type: "function",
						Function: services.ToolFunction{
							Name:      "get_user",
							Arguments: "{}",
						},
					},
				},
			},
			{
				Role:    "assistant",
				Content: "Here is your profile.",
			},
		}
		mockLLM.CallCount = 0
		mockLLM.ReceivedMessages = nil
		mockLLM.ReceivedToolCounts = nil

		firstReqBody, _ := json.Marshal(ChatRequest{Message: "What is my name?"})
		firstReq := httptest.NewRequest("POST", "/v1/chat", bytes.NewBuffer(firstReqBody))
		firstReq = firstReq.WithContext(context.WithValue(firstReq.Context(), authenticatedUserIDKey, userID))

		firstRR := httptest.NewRecorder()
		http.HandlerFunc(server.handleChat).ServeHTTP(firstRR, firstReq)
		if firstRR.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. body: %s", firstRR.Code, firstRR.Body.String())
		}

		var firstResp ChatResponse
		if err := json.NewDecoder(firstRR.Body).Decode(&firstResp); err != nil {
			t.Fatalf("decode first response: %v", err)
		}

		var savedMessages []models.ConversationMessage
		if err := db.
			Where("conversation_id = ?", firstResp.ConversationID).
			Order("created_at asc").
			Find(&savedMessages).Error; err != nil {
			t.Fatalf("load saved messages: %v", err)
		}
		if len(savedMessages) != 4 {
			t.Fatalf("expected 4 saved messages, got %d", len(savedMessages))
		}
		for i, msg := range savedMessages {
			expectedSequence := int64(i + 1)
			if msg.Sequence != expectedSequence {
				t.Fatalf("expected saved message %d to have sequence %d, got %d", i, expectedSequence, msg.Sequence)
			}
		}
		if savedMessages[1].Role != "assistant" || savedMessages[1].ToolCalls == nil {
			t.Fatalf("expected second saved message to be assistant tool call, got role=%q tool_calls_nil=%t", savedMessages[1].Role, savedMessages[1].ToolCalls == nil)
		}
		if savedMessages[2].Role != "tool" {
			t.Fatalf("expected third saved message to be tool response, got %q", savedMessages[2].Role)
		}
		if !savedMessages[1].CreatedAt.Before(savedMessages[2].CreatedAt) {
			t.Fatalf("expected assistant tool call to be saved before tool response, got %s then %s", savedMessages[1].CreatedAt, savedMessages[2].CreatedAt)
		}

		mockLLM.Responses = []*services.LLMMessage{
			{
				Role:    "assistant",
				Content: "Second turn reply.",
			},
		}
		mockLLM.CallCount = 0
		mockLLM.ReceivedMessages = nil
		mockLLM.ReceivedToolCounts = nil

		secondReqBody, _ := json.Marshal(ChatRequest{
			Message:        "And what about now?",
			ConversationID: firstResp.ConversationID,
		})
		secondReq := httptest.NewRequest("POST", "/v1/chat", bytes.NewBuffer(secondReqBody))
		secondReq = secondReq.WithContext(context.WithValue(secondReq.Context(), authenticatedUserIDKey, userID))

		secondRR := httptest.NewRecorder()
		http.HandlerFunc(server.handleChat).ServeHTTP(secondRR, secondReq)
		if secondRR.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. body: %s", secondRR.Code, secondRR.Body.String())
		}

		if len(mockLLM.ReceivedMessages) == 0 {
			t.Fatal("expected second request to reach llm")
		}
		replayedMessages := mockLLM.ReceivedMessages[0]
		if len(replayedMessages) < 6 {
			t.Fatalf("expected replayed transcript to include history and current user message, got %d messages", len(replayedMessages))
		}
		if replayedMessages[2].Role != "assistant" || len(replayedMessages[2].ToolCalls) != 1 {
			t.Fatalf("expected replayed assistant tool-call message at index 2, got role=%q tool_calls=%d", replayedMessages[2].Role, len(replayedMessages[2].ToolCalls))
		}
		if replayedMessages[3].Role != "tool" || replayedMessages[3].ToolCallID != "call-ordered" {
			t.Fatalf("expected replayed tool response after assistant tool call, got role=%q tool_call_id=%q", replayedMessages[3].Role, replayedMessages[3].ToolCallID)
		}
	})

	t.Run("Replays overlapping created_at history by persisted sequence", func(t *testing.T) {
		conv := models.Conversation{
			ID:     uuid.New(),
			UserID: userID,
			Title:  "Concurrent ordering",
		}
		if err := db.Create(&conv).Error; err != nil {
			t.Fatalf("create conversation: %v", err)
		}

		base := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
		toolCallsJSON := `[{"id":"call-sequenced","type":"function","function":{"name":"get_user","arguments":"{}"}}]`
		toolCallID := "call-sequenced"
		toolName := "get_user"

		seedMessages := []models.ConversationMessage{
			{
				ID:             uuid.New(),
				ConversationID: conv.ID,
				Role:           "user",
				Content:        "First question",
				Sequence:       1,
				CreatedAt:      base.Add(time.Microsecond),
			},
			{
				ID:             uuid.New(),
				ConversationID: conv.ID,
				Role:           "assistant",
				ToolCalls:      &toolCallsJSON,
				Sequence:       2,
				CreatedAt:      base.Add(2 * time.Microsecond),
			},
			{
				ID:             uuid.New(),
				ConversationID: conv.ID,
				Role:           "tool",
				Content:        `{"name":"Test User"}`,
				ToolCallID:     &toolCallID,
				ToolName:       &toolName,
				Sequence:       3,
				CreatedAt:      base.Add(3 * time.Microsecond),
			},
			{
				ID:             uuid.New(),
				ConversationID: conv.ID,
				Role:           "assistant",
				Content:        "First answer",
				Sequence:       4,
				CreatedAt:      base.Add(4 * time.Microsecond),
			},
			{
				ID:             uuid.New(),
				ConversationID: conv.ID,
				Role:           "user",
				Content:        "Second question",
				Sequence:       5,
				CreatedAt:      base.Add(time.Microsecond),
			},
			{
				ID:             uuid.New(),
				ConversationID: conv.ID,
				Role:           "assistant",
				Content:        "Second answer",
				Sequence:       6,
				CreatedAt:      base.Add(2 * time.Microsecond),
			},
		}
		for _, msg := range seedMessages {
			if err := db.Create(&msg).Error; err != nil {
				t.Fatalf("seed conversation message %s: %v", msg.Role, err)
			}
		}

		mockLLM.Responses = []*services.LLMMessage{
			{
				Role:    "assistant",
				Content: "Third answer.",
			},
		}
		mockLLM.CallCount = 0
		mockLLM.ReceivedMessages = nil
		mockLLM.ReceivedToolCounts = nil

		reqBody, _ := json.Marshal(ChatRequest{
			Message:        "Third question",
			ConversationID: conv.ID,
		})
		req := httptest.NewRequest("POST", "/v1/chat", bytes.NewBuffer(reqBody))
		req = req.WithContext(context.WithValue(req.Context(), authenticatedUserIDKey, userID))

		rr := httptest.NewRecorder()
		http.HandlerFunc(server.handleChat).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. body: %s", rr.Code, rr.Body.String())
		}

		if len(mockLLM.ReceivedMessages) == 0 {
			t.Fatal("expected llm call to receive replayed history")
		}

		replayedMessages := mockLLM.ReceivedMessages[0]
		var replayedContents []string
		for _, msg := range replayedMessages {
			if msg.Role == "system" {
				continue
			}
			if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
				replayedContents = append(replayedContents, "assistant_tool_call")
				continue
			}
			replayedContents = append(replayedContents, msg.Content)
		}

		got := strings.Join(replayedContents, " | ")
		want := "First question | assistant_tool_call | {\"name\":\"Test User\"} | First answer | Second question | Second answer | Third question"
		if got != want {
			t.Fatalf("expected replay order %q, got %q", want, got)
		}
	})

	t.Run("Reuses cached tool results for duplicate calls", func(t *testing.T) {
		mockLLM.Responses = []*services.LLMMessage{
			{
				Role: "assistant",
				ToolCalls: []*services.ToolCall{
					{
						ID:   "call-a",
						Type: "function",
						Function: services.ToolFunction{
							Name:      "get_user",
							Arguments: "{}",
						},
					},
					{
						ID:   "call-b",
						Type: "function",
						Function: services.ToolFunction{
							Name:      "get_user",
							Arguments: "{}",
						},
					},
				},
			},
			{
				Role:    "assistant",
				Content: "You are Test User.",
			},
		}
		mockLLM.CallCount = 0
		mockLLM.ReceivedMessages = nil
		mockLLM.ReceivedToolCounts = nil

		reqBody, _ := json.Marshal(ChatRequest{Message: "Who am I?"})
		req := httptest.NewRequest("POST", "/v1/chat", bytes.NewBuffer(reqBody))
		req = req.WithContext(context.WithValue(req.Context(), authenticatedUserIDKey, userID))

		rr := httptest.NewRecorder()
		http.HandlerFunc(server.handleChat).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. body: %s", rr.Code, rr.Body.String())
		}

		var resp ChatResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		var toolMessages []models.ConversationMessage
		if err := db.
			Where("conversation_id = ? AND role = ?", resp.ConversationID, "tool").
			Order("created_at asc").
			Find(&toolMessages).Error; err != nil {
			t.Fatalf("load tool messages: %v", err)
		}
		if len(toolMessages) != 2 {
			t.Fatalf("expected 2 tool messages, got %d", len(toolMessages))
		}
		if toolMessages[0].Content != toolMessages[1].Content {
			t.Fatalf("expected duplicate tool calls to reuse the same result, got %q and %q", toolMessages[0].Content, toolMessages[1].Content)
		}
		if bytes.Contains([]byte(toolMessages[1].Content), []byte("already fetched")) {
			t.Fatalf("expected duplicate tool call to reuse cached result, got %q", toolMessages[1].Content)
		}
	})
}
