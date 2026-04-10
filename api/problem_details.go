package api

import (
	"errors"
	"net/http"
	"sort"
	"strings"
)

const (
	validationProblemType  = "https://api.fitnesstracker.local/problems/validation"
	validationProblemTitle = "Validation failed"
)

type ProblemDetails struct {
	Type   string            `json:"type,omitempty"`
	Title  string            `json:"title"`
	Status int               `json:"status"`
	Detail string            `json:"detail,omitempty"`
	Error  string            `json:"error,omitempty"`
	Errors map[string]string `json:"errors,omitempty"`
}

type ValidationErrors struct {
	fields  map[string]string
	summary string
}

func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		fields: make(map[string]string),
	}
}

func (v *ValidationErrors) Add(field, message string) {
	if v == nil || strings.TrimSpace(field) == "" || strings.TrimSpace(message) == "" {
		return
	}
	if v.fields == nil {
		v.fields = make(map[string]string)
	}
	if _, exists := v.fields[field]; exists {
		return
	}
	v.fields[field] = message
}

func (v *ValidationErrors) Any() bool {
	return v != nil && len(v.fields) > 0
}

func (v *ValidationErrors) SetSummary(summary string) {
	if v == nil {
		return
	}
	v.summary = strings.TrimSpace(summary)
}

func (v *ValidationErrors) Error() string {
	if v == nil {
		return ""
	}
	return v.Summary()
}

func (v *ValidationErrors) Summary() string {
	if v == nil || len(v.fields) == 0 {
		return ""
	}
	if v.summary != "" {
		return v.summary
	}
	if len(v.fields) == 1 {
		for _, message := range v.fields {
			return message
		}
	}
	return "one or more fields are invalid"
}

func (v *ValidationErrors) Clone() map[string]string {
	if v == nil || len(v.fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(v.fields))
	for field, message := range v.fields {
		out[field] = message
	}
	return out
}

func (v *ValidationErrors) Fields() []string {
	if v == nil || len(v.fields) == 0 {
		return nil
	}
	fields := make([]string, 0, len(v.fields))
	for field := range v.fields {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func validationProblem(status int, err error) ProblemDetails {
	title := http.StatusText(status)
	if title == "" {
		title = "Error"
	}

	problem := ProblemDetails{
		Type:   "about:blank",
		Title:  title,
		Status: status,
	}

	if err == nil {
		return problem
	}

	detail := err.Error()
	problem.Detail = detail
	problem.Error = detail

	var validationErr *ValidationErrors
	if errors.As(err, &validationErr) && validationErr.Any() {
		problem.Type = validationProblemType
		problem.Title = validationProblemTitle
		problem.Detail = validationErr.Summary()
		problem.Error = validationErr.Summary()
		problem.Errors = validationErr.Clone()
	}

	return problem
}

func singleFieldError(field, message string) *ValidationErrors {
	errs := NewValidationErrors()
	errs.Add(field, message)
	return errs
}
