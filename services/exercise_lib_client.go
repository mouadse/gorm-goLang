package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"fitness-tracker/metrics"
)

type ExerciseLibClient struct {
	baseURL    string
	httpClient *http.Client
	metrics    *metrics.Metrics
}

type ExerciseLibAPIError struct {
	StatusCode int
	Body       string
}

func (e *ExerciseLibAPIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Body == "" {
		return fmt.Sprintf("exercise lib returned %d", e.StatusCode)
	}
	return fmt.Sprintf("exercise lib returned %d: %s", e.StatusCode, e.Body)
}

func NewExerciseLibClient(m ...*metrics.Metrics) *ExerciseLibClient {
	c := &ExerciseLibClient{
		baseURL:    getEnvOrDefault("EXERCISE_LIB_URL", "http://localhost:8000"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	if len(m) > 0 {
		c.metrics = m[0]
	}
	return c
}

// --- Request / Response types ---

type LibInitResponse struct {
	Status          string `json:"status"`
	ExercisesLoaded int    `json:"exercises_loaded"`
}

type LibMetaResponse struct {
	LibrarySize       int                 `json:"library_size"`
	Levels            []string            `json:"levels"`
	Categories        []string            `json:"categories"`
	Equipment         []string            `json:"equipment"`
	Muscles           []string            `json:"muscles"`
	EquipmentProfiles []map[string]string `json:"equipment_profiles"`
	SampleQueries     []string            `json:"sample_queries"`
}

type LibCatalogExercise struct {
	ExerciseID       string   `json:"exercise_id"`
	Name             string   `json:"name"`
	Force            string   `json:"force"`
	Level            string   `json:"level"`
	Mechanic         string   `json:"mechanic"`
	Equipment        string   `json:"equipment"`
	Category         string   `json:"category"`
	PrimaryMuscles   []string `json:"primary_muscles"`
	SecondaryMuscles []string `json:"secondary_muscles"`
	Instructions     []string `json:"instructions"`
	ImageURL         *string  `json:"image_url"`
	AltImageURL      *string  `json:"alt_image_url"`
}

type LibCatalogExercisesResponse struct {
	Total     int                  `json:"total"`
	Exercises []LibCatalogExercise `json:"exercises"`
}

type LibSearchRequest struct {
	Query     string  `json:"query"`
	TopK      int     `json:"top_k"`
	Level     *string `json:"level,omitempty"`
	Equipment *string `json:"equipment,omitempty"`
	Category  *string `json:"category,omitempty"`
	Muscle    *string `json:"muscle,omitempty"`
}

type LibExerciseResult struct {
	ExerciseID       string   `json:"exercise_id"`
	Score            float64  `json:"score"`
	Name             string   `json:"name"`
	Force            string   `json:"force"`
	Level            string   `json:"level"`
	Mechanic         string   `json:"mechanic"`
	Equipment        string   `json:"equipment"`
	Category         string   `json:"category"`
	PrimaryMuscles   []string `json:"primary_muscles"`
	SecondaryMuscles []string `json:"secondary_muscles"`
	Instructions     []string `json:"instructions"`
	ImageURL         *string  `json:"image_url"`
	AltImageURL      *string  `json:"alt_image_url"`
	MatchReasons     []string `json:"match_reasons"`
}

type LibSearchResponse struct {
	Results []LibExerciseResult `json:"results"`
}

type LibProgramRequest struct {
	Goal             string   `json:"goal"`
	DaysPerWeek      int      `json:"days_per_week"`
	SessionMinutes   int      `json:"session_minutes"`
	Level            string   `json:"level"`
	EquipmentProfile string   `json:"equipment_profile"`
	Focus            []string `json:"focus"`
	Notes            string   `json:"notes"`
}

type LibProgramExercise struct {
	ExerciseID       string   `json:"exercise_id"`
	Name             string   `json:"name"`
	ImageURL         *string  `json:"image_url"`
	AltImageURL      *string  `json:"alt_image_url"`
	Category         string   `json:"category"`
	Mechanic         string   `json:"mechanic"`
	Equipment        string   `json:"equipment"`
	PrimaryMuscles   []string `json:"primary_muscles"`
	SecondaryMuscles []string `json:"secondary_muscles"`
	Prescription     string   `json:"prescription"`
	Reason           string   `json:"reason"`
	Instructions     []string `json:"instructions"`
}

type LibProgramDay struct {
	Day       int                  `json:"day"`
	Title     string               `json:"title"`
	Focus     string               `json:"focus"`
	Duration  string               `json:"duration_label"`
	Exercises []LibProgramExercise `json:"exercises"`
}

type LibProgramResponse struct {
	Summary      string          `json:"summary"`
	RecoveryNote string          `json:"recovery_note"`
	Warmup       []string        `json:"warmup"`
	Days         []LibProgramDay `json:"days"`
}

func (c *ExerciseLibClient) BaseURL() string {
	return c.baseURL
}

// --- Methods ---

func (c *ExerciseLibClient) Init() (*LibInitResponse, error) {
	var resp LibInitResponse
	err := c.do("POST", "/init", nil, &resp)
	return &resp, err
}

func (c *ExerciseLibClient) GetMeta() (*LibMetaResponse, error) {
	var resp LibMetaResponse
	err := c.do("GET", "/catalog/meta", nil, &resp)
	return &resp, err
}

func (c *ExerciseLibClient) GetAllExercises() ([]LibCatalogExercise, error) {
	var all []LibCatalogExercise
	offset := 0
	for {
		path := fmt.Sprintf("/catalog/exercises?limit=500&offset=%d", offset)
		var resp LibCatalogExercisesResponse
		if err := c.do("GET", path, nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Exercises...)
		if offset+len(resp.Exercises) >= resp.Total {
			break
		}
		offset += len(resp.Exercises)
	}
	return all, nil
}

func (c *ExerciseLibClient) Search(req LibSearchRequest) (*LibSearchResponse, error) {
	var resp LibSearchResponse
	err := c.do("POST", "/search", req, &resp)
	return &resp, err
}

func (c *ExerciseLibClient) GetProgram(req LibProgramRequest) (*LibProgramResponse, error) {
	var resp LibProgramResponse
	err := c.do("POST", "/program", req, &resp)
	return &resp, err
}

// --- Internal ---

func (c *ExerciseLibClient) do(method, path string, body any, target any) error {
	start := time.Now()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.metrics != nil {
			c.metrics.ExtServiceErrors.WithLabelValues("exercise_lib", "connection").Inc()
		}
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	status := fmt.Sprintf("%d", resp.StatusCode)
	if c.metrics != nil {
		c.metrics.ExtServiceRequests.WithLabelValues("exercise_lib", method, status).Inc()
		c.metrics.ExtServiceDuration.WithLabelValues("exercise_lib").Observe(time.Since(start).Seconds())
	}

	if resp.StatusCode >= 400 {
		if c.metrics != nil {
			c.metrics.ExtServiceErrors.WithLabelValues("exercise_lib", status).Inc()
		}
		b, _ := io.ReadAll(resp.Body)
		return &ExerciseLibAPIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(b)),
		}
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
