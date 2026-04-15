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
	"gorm.io/gorm/clause"
)

type ChatRequest struct {
	Message        string    `json:"message"`
	ConversationID uuid.UUID `json:"conversation_id"`
}

type ChatResponse struct {
	Message        string    `json:"message"`
	ConversationID uuid.UUID `json:"conversation_id"`
}

const maxCoachToolRounds = 4

func coachSystemPrompt() string {
	return `You are a helpful AI fitness coach for a fitness tracker application. Answer the user's questions using their real data and the available tools. Keep answers concise, specific, and actionable.

All user-scoped tools refer to the authenticated user in this chat.

Use profile and logging tools whenever the user asks about personal account or tracking data:
- get_user for profile questions such as name, goal, activity level, weight, height, or TDEE
- get_user_workouts to inspect workout history, especially for questions about the last workout they logged
- get_workout to inspect one specific workout's exercises and sets
- get_user_meals for meal history and nutrition logging questions
- get_user_weight_entries for bodyweight progress questions

Use coach data tools whenever the user asks about progress, trends, nutrition intake, recovery, adherence, streaks, records, or recommendations:
- get_daily_summary for daily nutrition context
- get_weekly_summary for weekly nutrition and workout context
- get_user_streaks for consistency and adherence
- get_user_records for personal records and best lifts
- get_user_workout_stats for workout trends and volume
- get_recommendations for personalized coaching recommendations
- get_notifications and get_unread_notification_count only when the user asks about notifications

Use exercise library tools when the user:
- asks about exercises for specific muscles, goals, equipment, or movement patterns (search_exercises)
- wants a workout program or training plan (generate_program)
- needs to know available equipment profiles, levels, or metadata (get_exercise_library_meta)

When answering:
- fetch the missing data you need before answering
- for questions like "what is my name", "what is my TDEE", or "what was my last workout", do not guess; call the relevant tool first
- combine tool results into one natural response
- be specific with numbers and comparisons when relevant
- never show raw JSON, function names, tool names, or internal IDs
- never mention that you used tools or APIs`
}

func marshalToolResult(result interface{}, err error) string {
	payload := result
	if err != nil {
		payload = map[string]string{"error": err.Error()}
	}

	b, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		fallback, _ := json.Marshal(map[string]string{"error": "failed to serialize tool result"})
		return string(fallback)
	}

	return string(b)
}

func toolCallSignature(tc *services.ToolCall) string {
	if tc == nil {
		return ""
	}
	return tc.Function.Name + ":" + tc.Function.Arguments
}

func orderedConversationMessages(db *gorm.DB) *gorm.DB {
	return db.
		Order("conversation_messages.sequence asc").
		Order("conversation_messages.created_at asc").
		Order("conversation_messages.id asc")
}

func legacyOrderedConversationMessages(db *gorm.DB) *gorm.DB {
	return db.
		Order("conversation_messages.created_at asc").
		Order("conversation_messages.id asc")
}

func nextConversationMessageTime(existing []models.ConversationMessage) func() time.Time {
	next := time.Now().UTC()
	for _, msg := range existing {
		if msg.CreatedAt.After(next) {
			next = msg.CreatedAt
		}
	}

	return func() time.Time {
		next = next.Add(time.Microsecond)
		return next
	}
}

func conversationLockingQuery(tx *gorm.DB) *gorm.DB {
	switch tx.Dialector.Name() {
	case "postgres", "mysql":
		return tx.Clauses(clause.Locking{Strength: "UPDATE"})
	default:
		return tx
	}
}

func ensureConversationMessageSequences(tx *gorm.DB, conversationID uuid.UUID) ([]models.ConversationMessage, error) {
	var unsequencedCount int64
	if err := tx.Model(&models.ConversationMessage{}).
		Where("conversation_id = ? AND sequence = 0", conversationID).
		Count(&unsequencedCount).Error; err != nil {
		return nil, err
	}

	if unsequencedCount > 0 {
		var legacyMessages []models.ConversationMessage
		if err := tx.
			Where("conversation_id = ?", conversationID).
			Scopes(legacyOrderedConversationMessages).
			Find(&legacyMessages).Error; err != nil {
			return nil, err
		}

		for i, msg := range legacyMessages {
			if err := tx.Model(&models.ConversationMessage{}).
				Where("id = ?", msg.ID).
				Update("sequence", int64(i+1)).Error; err != nil {
				return nil, err
			}
		}
	}

	var orderedMessages []models.ConversationMessage
	if err := tx.
		Where("conversation_id = ?", conversationID).
		Scopes(orderedConversationMessages).
		Find(&orderedMessages).Error; err != nil {
		return nil, err
	}

	return orderedMessages, nil
}

func persistConversationTurn(db *gorm.DB, conversationID, userID uuid.UUID, turnMessages []models.ConversationMessage) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var conv models.Conversation
		if err := conversationLockingQuery(tx).
			Where("id = ? AND user_id = ?", conversationID, userID).
			First(&conv).Error; err != nil {
			return err
		}

		existingMessages, err := ensureConversationMessageSequences(tx, conversationID)
		if err != nil {
			return err
		}

		nextSequence := int64(1)
		if len(existingMessages) > 0 {
			nextSequence = existingMessages[len(existingMessages)-1].Sequence + 1
		}
		nextMessageTime := nextConversationMessageTime(existingMessages)

		for i := range turnMessages {
			turnMessages[i].ConversationID = conversationID
			turnMessages[i].Sequence = nextSequence
			nextSequence++
			if turnMessages[i].CreatedAt.IsZero() {
				turnMessages[i].CreatedAt = nextMessageTime()
			}
			if err := tx.Create(&turnMessages[i]).Error; err != nil {
				return err
			}
		}

		return tx.Model(&models.Conversation{}).
			Where("id = ?", conversationID).
			UpdateColumn("updated_at", time.Now().UTC()).Error
	})
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
			return orderedConversationMessages(db)
		}).First(&conv).Error; err != nil {
			writeError(w, http.StatusNotFound, errors.New("Conversation not found"))
			return
		}
	}

	// Prepare messages for LLM
	var llmMessages []services.LLMMessage

	// Add system prompt
	llmMessages = append(llmMessages, services.LLMMessage{Role: "system", Content: coachSystemPrompt()})

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

	cachedToolResults := map[string]string{}
	turnMessages := []models.ConversationMessage{
		{
			Role:    "user",
			Content: req.Message,
		},
	}

	// Handle tool calls
	for toolRound := 0; len(llmResp.ToolCalls) > 0; toolRound++ {
		if toolRound >= maxCoachToolRounds {
			llmResp = &services.LLMMessage{
				Role:    "assistant",
				Content: "I couldn't finish gathering your coaching data in time. Please try again.",
			}
			break
		}

		llmMessages = append(llmMessages, *llmResp)

		// Save one assistant message with all tool calls
		toolCallsJSON, _ := json.Marshal(llmResp.ToolCalls)
		toolCallsStr := string(toolCallsJSON)
		turnMessages = append(turnMessages, models.ConversationMessage{
			Role:      "assistant",
			ToolCalls: &toolCallsStr,
		})

		for _, tc := range llmResp.ToolCalls {
			signature := toolCallSignature(tc)
			toolResultStr, exists := cachedToolResults[signature]
			if !exists {
				toolResult, dispatchErr := s.coachSvc.DispatchFunction(tc.Function.Name, tc.Function.Arguments, userID)
				toolResultStr = marshalToolResult(toolResult, dispatchErr)
				cachedToolResults[signature] = toolResultStr
			}

			// Save tool response message
			toolCallID := tc.ID
			toolName := tc.Function.Name
			turnMessages = append(turnMessages, models.ConversationMessage{
				Role:       "tool",
				Content:    toolResultStr,
				ToolCallID: &toolCallID,
				ToolName:   &toolName,
			})

			llmMessages = append(llmMessages, services.LLMMessage{
				Role:       "tool",
				Content:    toolResultStr,
				Name:       tc.Function.Name,
				ToolCallID: tc.ID,
			})
		}

		// Second call
		start2 := time.Now()
		llmResp, err = s.llmClient.Chat(llmMessages, tools)
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
	turnMessages = append(turnMessages, models.ConversationMessage{
		Role:    "assistant",
		Content: llmResp.Content,
	})
	if err := persistConversationTurn(s.db, conv.ID, userID, turnMessages); err != nil {
		writeError(w, http.StatusInternalServerError, errors.New("Failed to save chat transcript"))
		return
	}

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
			return orderedConversationMessages(db)
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
