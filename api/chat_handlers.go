package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ChatRequest struct {
	Message        string    `json:"message"`
	ConversationID uuid.UUID `json:"conversation_id"`
}

type ChatResponse struct {
	Message        string    `json:"message"`
	ConversationID uuid.UUID `json:"conversation_id"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errors.New("Unauthorized"))
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("Invalid request payload"))
		return
	}

	if req.Message == "" {
		writeError(w, http.StatusBadRequest, errors.New("Message cannot be empty"))
		return
	}

	var conv models.Conversation
	if req.ConversationID == uuid.Nil {
		conv = models.Conversation{
			UserID: userID,
			Title:  "New Conversation",
		}
		if err := s.db.Create(&conv).Error; err != nil {
			writeError(w, http.StatusInternalServerError, errors.New("Failed to create conversation"))
			return
		}
		req.ConversationID = conv.ID
	} else {
		if err := s.db.Where("id = ? AND user_id = ?", req.ConversationID, userID).Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at asc")
		}).First(&conv).Error; err != nil {
			writeError(w, http.StatusNotFound, errors.New("Conversation not found"))
			return
		}
	}

	// Save user message
	userMsg := models.ConversationMessage{
		ConversationID: conv.ID,
		Role:           "user",
		Content:        req.Message,
	}
	s.db.Create(&userMsg)

	// Prepare messages for LLM
	var llmMessages []services.LLMMessage

	// Add system prompt
	systemPrompt := `You are a helpful AI fitness coach. Answer questions based on the user's data using tools. Keep answers concise.

You have access to exercise library tools. Use them when the user:
- Asks about exercises for specific muscles or goals (use search_exercises)
- Wants a workout program or training plan (use generate_program)
- Asks what equipment, levels, or muscle groups are available (use get_exercise_library_meta)

When generating a program, try to infer the user's level and goals from their workout stats and records before calling generate_program. If unsure about available equipment profiles or options, call get_exercise_library_meta first.

When searching exercises, use natural language queries like "chest compound exercises" or "back and biceps isolation moves". You can combine search results with the user's personal records to give tailored advice.`
	llmMessages = append(llmMessages, services.LLMMessage{Role: "system", Content: systemPrompt})

	for _, m := range conv.Messages {
		msg := services.LLMMessage{
			Role:       m.Role,
			Content:    m.Content,
			Name:       getStringPtrOrEmpty(m.ToolName),
			ToolCallID: getStringPtrOrEmpty(m.ToolCallID),
		}

		if m.Role == "assistant" && m.ToolCalls != nil {
			var toolCalls []*services.ToolCall
			if err := json.Unmarshal([]byte(*m.ToolCalls), &toolCalls); err == nil {
				msg.ToolCalls = toolCalls
			}
		} else if m.Role == "assistant" && m.ToolCallID != nil {
			// Backwards compatibility for old tool calls
			toolArgs := getStringPtrOrEmpty(m.ToolArgs)
			msg.ToolCalls = []*services.ToolCall{
				{
					ID:   *m.ToolCallID,
					Type: "function",
					Function: services.ToolFunction{
						Name:      getStringPtrOrEmpty(m.ToolName),
						Arguments: toolArgs,
					},
				},
			}
		}

		llmMessages = append(llmMessages, msg)
	}
	llmMessages = append(llmMessages, services.LLMMessage{Role: "user", Content: req.Message})

	tools := s.coachSvc.GetTools()

	// Call LLM
	start := time.Now()
	llmResp, err := s.llmClient.Chat(llmMessages, tools)
	if err != nil {
		s.metrics.CoachRequestsTotal.WithLabelValues("error").Inc()
		s.metrics.CoachRequestDuration.WithLabelValues("").Observe(time.Since(start).Seconds())
		writeError(w, http.StatusInternalServerError, errors.New("Failed to communicate with AI"))
		return
	}
	s.metrics.CoachRequestsTotal.WithLabelValues("success").Inc()
	s.metrics.CoachRequestDuration.WithLabelValues("").Observe(time.Since(start).Seconds())
	s.metrics.ChatMessagesTotal.Inc()

	// Handle tool calls
	if len(llmResp.ToolCalls) > 0 {
		llmMessages = append(llmMessages, *llmResp)

		// Save one assistant message with all tool calls
		toolCallsJSON, _ := json.Marshal(llmResp.ToolCalls)
		toolCallsStr := string(toolCallsJSON)
		astMsg := models.ConversationMessage{
			ConversationID: conv.ID,
			Role:           "assistant",
			ToolCalls:      &toolCallsStr,
		}
		s.db.Create(&astMsg)

		for _, tc := range llmResp.ToolCalls {
			toolResult, err := s.coachSvc.DispatchFunction(tc.Function.Name, tc.Function.Arguments, userID)
			toolResultStr := "{}"
			if err == nil {
				b, _ := json.Marshal(toolResult)
				toolResultStr = string(b)
			} else {
				toolResultStr = `{"error": "` + err.Error() + `"}`
			}

			// Save tool response message
			toolCallID := tc.ID
			toolName := tc.Function.Name
			tMsg := models.ConversationMessage{
				ConversationID: conv.ID,
				Role:           "tool",
				Content:        toolResultStr,
				ToolCallID:     &toolCallID,
				ToolName:       &toolName,
			}
			s.db.Create(&tMsg)

			llmMessages = append(llmMessages, services.LLMMessage{
				Role:       "tool",
				Content:    toolResultStr,
				Name:       tc.Function.Name,
				ToolCallID: tc.ID,
			})
		}

		// Second call
		start2 := time.Now()
		llmResp, err = s.llmClient.Chat(llmMessages, nil)
		if err != nil {
			s.metrics.CoachRequestsTotal.WithLabelValues("error").Inc()
			s.metrics.CoachRequestDuration.WithLabelValues("").Observe(time.Since(start2).Seconds())
			writeError(w, http.StatusInternalServerError, errors.New("Failed to communicate with AI after tool call"))
			return
		}
		s.metrics.CoachRequestsTotal.WithLabelValues("success").Inc()
		s.metrics.CoachRequestDuration.WithLabelValues("").Observe(time.Since(start2).Seconds())
	}

	// Save final assistant message
	finalAstMsg := models.ConversationMessage{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        llmResp.Content,
	}
	s.db.Create(&finalAstMsg)

	writeJSON(w, http.StatusOK, ChatResponse{
		Message:        llmResp.Content,
		ConversationID: conv.ID,
	})
}

func getStringPtrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Server) handleChatHistory(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errors.New("Unauthorized"))
		return
	}

	convIDStr := r.URL.Query().Get("conversation_id")
	if convIDStr != "" {
		convID, err := uuid.Parse(convIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("invalid conversation_id"))
			return
		}

		var conv models.Conversation
		if err := s.db.Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at asc")
		}).Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeError(w, http.StatusNotFound, errors.New("conversation not found"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, conv)
		return
	}

	page, limit := parsePagination(r)
	var convs []models.Conversation
	paginated, err := paginate(s.db.Where("user_id = ?", userID).Order("updated_at desc"), page, limit, &convs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, paginated)
}

type ChatFeedbackRequest struct {
	MessageID uuid.UUID `json:"message_id"`
	Feedback  int       `json:"feedback"`
}

func (s *Server) handleChatFeedback(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errors.New("Unauthorized"))
		return
	}

	var req ChatFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("Invalid request payload"))
		return
	}

	var msg models.ConversationMessage
	if err := s.db.Joins("JOIN conversations on conversations.id = conversation_messages.conversation_id").
		Where("conversation_messages.id = ? AND conversations.user_id = ?", req.MessageID, userID).
		First(&msg).Error; err != nil {
		writeError(w, http.StatusNotFound, errors.New("Message not found"))
		return
	}

	msg.Feedback = &req.Feedback
	if err := s.db.Save(&msg).Error; err != nil {
		writeError(w, http.StatusInternalServerError, errors.New("Failed to save feedback"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Feedback received"})
}

func (s *Server) handleCoachSummary(w http.ResponseWriter, r *http.Request) {
	targetUserID, err := parsePathUUID(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := authorizeUser(r, targetUserID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	dateStr := r.URL.Query().Get("date")
	date := time.Now()
	if dateStr != "" {
		if d, err := time.Parse("2006-01-02", dateStr); err == nil {
			date = d
		} else {
			writeError(w, http.StatusBadRequest, errors.New("Invalid date format, expected YYYY-MM-DD"))
			return
		}
	}

	summary, err := s.coachSvc.GetCoachSummary(targetUserID, date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errors.New(err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, summary)
}
