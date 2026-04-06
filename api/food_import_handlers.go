package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
)

const defaultUSDADatasetPath = "FoodData_Central_foundation_food_json_2025-12-18.json"

type importUSDARequest struct {
	FilePath string `json:"file_path"`
}

type importUSDAResponse struct {
	FoodCount   int    `json:"food_count"`
	NewFoods    int    `json:"new_foods"`
	NutrientRow int    `json:"nutrient_rows"`
	Duration    string `json:"duration"`
}

func (s *Server) handleImportUSDA(w http.ResponseWriter, r *http.Request) {
	var req importUSDARequest
	if err := decodeJSON(r, &req); err != nil {
		if !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
			return
		}
		// Allow empty body — use default path
		req.FilePath = ""
	}

	filePath := req.FilePath
	if filePath == "" {
		filePath = defaultUSDADatasetPath
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		writeError(w, http.StatusBadRequest, errors.New("dataset file not found: "+filePath))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	startTime := time.Now()
	stats, err := s.importSvc.ImportFromFile(ctx, filePath)
	duration := time.Since(startTime)

	adminID, _ := authenticatedUserID(r)
	importLog := models.FoodImportLog{
		AdminID:       adminID,
		Source:        "usda",
		Status:        "success",
		FoodsImported: stats.NewFoods,
		DurationMs:    duration.Milliseconds(),
		CreatedAt:     time.Now(),
	}

	if err != nil {
		importLog.Status = "failed"
		importLog.ErrorMessage = err.Error()
		s.db.Create(&importLog)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.db.Create(&importLog)
	s.logAdminAction(r, "import_usda", "food", uuid.Nil, nil, stats)

	writeJSON(w, http.StatusOK, importUSDAResponse{
		FoodCount:   stats.FoodCount,
		NewFoods:    stats.NewFoods,
		NutrientRow: stats.NutrientRow,
		Duration:    stats.Duration.Round(time.Millisecond).String(),
	})
}
