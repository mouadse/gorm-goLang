package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"fitness-tracker/services"

	"github.com/google/uuid"
)

type createExportJobRequest struct {
	Format string `json:"format"`
}

func (s *Server) handleCreateExportJob(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var req createExportJobRequest
	if err := decodeJSON(r, &req); err != nil {
		// Default to JSON only for empty body (EOF), reject malformed JSON
		if errors.Is(err, io.EOF) {
			req.Format = string(services.ExportJSON)
		} else {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
			return
		}
	}

	format := services.ExportJSON
	if req.Format == string(services.ExportCSV) {
		format = services.ExportCSV
	} else if req.Format != string(services.ExportJSON) && req.Format != "" {
		writeError(w, http.StatusBadRequest, errors.New("format must be json or csv"))
		return
	}

	job, err := s.exportSvc.CreateExportJob(userID, format)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("create export job: %w", err))
		return
	}

	// Test and in-memory SQLite setups run without the external worker process.
	if s.db.Dialector.Name() == "sqlite" {
		if err := s.exportSvc.ProcessExportJob(job.ID); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("process export job: %w", err))
			return
		}
		job, err = s.exportSvc.GetExportJob(userID, job.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("reload export job: %w", err))
			return
		}
	}

	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleGetExportJob(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	jobIDStr := r.PathValue("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid job ID"))
		return
	}

	job, err := s.exportSvc.GetExportJob(userID, jobID)
	if err != nil {
		if err.Error() == "export job not found" {
			writeError(w, http.StatusNotFound, err)
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("get export job: %w", err))
		}
		return
	}

	if r.URL.Query().Get("download") == "true" {
		if job.Status != services.ExportCompleted {
			writeError(w, http.StatusConflict, errors.New("export job not completed yet"))
			return
		}

		data, contentType, err := s.exportSvc.GetExportData(userID, job.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("get export data: %w", err))
			return
		}

		w.Header().Set("Content-Type", contentType)
		filename := "export.json"
		if job.Format == services.ExportCSV {
			filename = "export.csv"
		}
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleCreateDeletionRequest(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	req, err := s.exportSvc.CreateDeletionRequest(userID)
	if err != nil {
		if err.Error() == "deletion request already pending" {
			writeError(w, http.StatusConflict, err)
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("create deletion request: %w", err))
		}
		return
	}

	// For MVP, process immediately
	err = s.exportSvc.ProcessDeletionRequest(userID)
	if err != nil {
		// Roll back the deletion request so the user can retry
		_ = s.exportSvc.CancelDeletionRequest(userID)
		writeError(w, http.StatusInternalServerError, fmt.Errorf("process deletion request: %w", err))
		return
	}

	// Update status so the response reflects reality
	req.Status = "processed"

	writeJSON(w, http.StatusAccepted, req)
}
