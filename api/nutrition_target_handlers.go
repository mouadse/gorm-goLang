package api

import (
	"errors"
	"net/http"

	"fitness-tracker/services"

	"gorm.io/gorm"
)

// handleGetUserNutritionTargets godoc
// @Summary Get user nutrition targets
// @Description Calculates or retrieves the nutrition targets for the specified user
// @Tags nutrition-targets
// @Produce json
// @Param user_id path string true "User ID" format(uuid)
// @Success 200 {object} services.NutritionTargets
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /v1/users/{user_id}/nutrition-targets [get]
// @Security BearerAuth
func (s *Server) handleGetUserNutritionTargets(w http.ResponseWriter, r *http.Request) {
	targetUserID, err := parsePathUUID(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// Authorize user (ensure they are requesting their own targets, or they have admin rights, but authorizeUser only checks ownership here)
	if err := authorizeUser(r, targetUserID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	svc := services.NewNutritionTargetService(s.db)
	targets, err := svc.GetUserNutritionTargets(targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("user not found or invalid target data"))
		} else {
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}

	writeJSON(w, http.StatusOK, targets)
}
