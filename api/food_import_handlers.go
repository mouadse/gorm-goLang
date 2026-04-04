package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
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

	stats, err := s.importSvc.ImportFromFile(ctx, filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, importUSDAResponse{
		FoodCount:   stats.FoodCount,
		NewFoods:    stats.NewFoods,
		NutrientRow: stats.NutrientRow,
		Duration:    stats.Duration.Round(time.Millisecond).String(),
	})
}
