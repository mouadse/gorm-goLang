package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

type PaginationMetadata struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	TotalCount int  `json:"total_count"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
}

type PaginatedResponse[T any] struct {
	Data     []T                `json:"data"`
	Metadata PaginationMetadata `json:"metadata"`
}

func parsePagination(r *http.Request) (int, int) {
	page := 1
	limit := 20

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}
	}

	return page, limit
}

func paginate[T any](query *gorm.DB, page, limit int, dest *[]T) (PaginatedResponse[T], error) {
	var totalCount int64
	var t T

	// Count using a subquery to avoid ORDER BY leaking into the count statement,
	// which causes PostgreSQL "column must appear in GROUP BY" errors.
	// The Session with NewDB=false clones the query (preserving WHERE/JOIN clauses)
	// while Count() itself resets SELECT and ORDER BY via GORM internals.
	if err := query.Session(&gorm.Session{NewDB: false}).Model(&t).Count(&totalCount).Error; err != nil {
		return PaginatedResponse[T]{}, err
	}

	offset := (page - 1) * limit
	if err := query.Model(&t).Limit(limit).Offset(offset).Find(dest).Error; err != nil {
		return PaginatedResponse[T]{}, err
	}

	totalPages := int((totalCount + int64(limit) - 1) / int64(limit))
	hasNext := page < totalPages

	return PaginatedResponse[T]{
		Data: ensureSlice(*dest),
		Metadata: PaginationMetadata{
			Page:       page,
			Limit:      limit,
			TotalCount: int(totalCount),
			TotalPages: totalPages,
			HasNext:    hasNext,
		},
	}, nil
}
