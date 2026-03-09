package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const dateLayout = "2006-01-02"

func ensureSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func parseDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("date must be %s", dateLayout)
	}

	parsed, err := time.Parse(dateLayout, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("date must be %s", dateLayout)
	}

	return parsed.UTC(), nil
}

func parseDateOrDefault(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Now().UTC().Truncate(24 * time.Hour), nil
	}
	return parseDate(raw)
}

func parseOptionalUUID(field, raw string) (*uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid uuid for %s", field)
	}

	return &parsed, nil
}

func requireNonBlank(field, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	return value, nil
}

func recordExists(db *gorm.DB, model any, id uuid.UUID) (bool, error) {
	err := db.Select("id").First(model, "id = ?", id).Error
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func resolveScopedUUID(r *http.Request, pathField, bodyField, raw string) (uuid.UUID, error) {
	pathValue := strings.TrimSpace(r.PathValue(pathField))
	bodyValue := strings.TrimSpace(raw)

	if pathValue == "" {
		return parseRequiredUUID(bodyField, bodyValue)
	}

	pathID, err := parseRequiredUUID(bodyField, pathValue)
	if err != nil {
		return uuid.Nil, err
	}

	if bodyValue == "" {
		return pathID, nil
	}

	bodyID, err := parseRequiredUUID(bodyField, bodyValue)
	if err != nil {
		return uuid.Nil, err
	}

	if bodyID != pathID {
		return uuid.Nil, fmt.Errorf("%s in path and body must match", bodyField)
	}

	return pathID, nil
}
