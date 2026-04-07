package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type LLMMessage struct {
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
	Name       string      `json:"name,omitempty"` // For tool role
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []*ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // always "function"
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type ToolDef struct {
	Type     string      `json:"type"` // "function"
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // e.g. JSON schema map
}

type LLMRequest struct {
	Model      string       `json:"model"`
	Messages   []LLMMessage `json:"messages"`
	Tools      []ToolDef    `json:"tools,omitempty"`
	ToolChoice string       `json:"tool_choice,omitempty"`
}

type LLMResponse struct {
	Choices []struct {
		Message LLMMessage `json:"message"`
	} `json:"choices"`
}

type LLMClient interface {
	Chat(messages []LLMMessage, tools []ToolDef) (*LLMMessage, error)
}

type OpenRouterClient struct {
	APIKey string
	Model  string
	URL    string
	Client *http.Client
}

func NewOpenRouterClient(apiKey, model string) *OpenRouterClient {
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if model == "" {
		model = os.Getenv("LLM_MODEL")
		if model == "" {
			model = "google/gemini-2.0-flash-001"
		}
	}
	return &OpenRouterClient{
		APIKey: apiKey,
		Model:  model,
		URL:    "https://openrouter.ai/api/v1/chat/completions",
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *OpenRouterClient) Chat(messages []LLMMessage, tools []ToolDef) (*LLMMessage, error) {
	reqBody := LLMRequest{
		Model:    c.Model,
		Messages: messages,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
		reqBody.ToolChoice = "auto"
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	// OpenRouter specific headers
	req.Header.Set("HTTP-Referer", "http://localhost:8080")
	req.Header.Set("X-Title", "Fitness Tracker AI")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	var llmResp LLMResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from LLM")
	}

	return &llmResp.Choices[0].Message, nil
}
