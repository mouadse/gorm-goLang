package api

import (
	"errors"
	"net/http"

	"fitness-tracker/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type createFriendRequestRequest struct {
	RequesterID string `json:"requester_id"`
	AddresseeID string `json:"addressee_id"`
}

type acceptFriendRequestRequest struct {
	ActorUserID string `json:"actor_user_id"`
}

func (s *Server) handleCreateFriendRequest(w http.ResponseWriter, r *http.Request) {
	var req createFriendRequestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	requesterID, err := parseRequiredUUID("requester_id", req.RequesterID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	addresseeID, err := parseRequiredUUID("addressee_id", req.AddresseeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	leftID, rightID := canonicalizeUserPair(requesterID, addresseeID)

	var existing models.Friendship
	err = s.db.Where("user_id = ? AND friend_id = ?", leftID, rightID).First(&existing).Error
	if err == nil {
		writeError(w, http.StatusConflict, errors.New("friend request already exists for this pair"))
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	friendship := models.Friendship{
		UserID:      requesterID,
		FriendID:    addresseeID,
		RequesterID: requesterID,
		Status:      "pending",
	}
	if err := s.db.Create(&friendship).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.db.Preload("Requester").Preload("User").Preload("Friend").First(&friendship, "id = ?", friendship.ID).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, friendship)
}

func (s *Server) handleListIncomingFriendRequests(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var friendships []models.Friendship
	if err := s.db.
		Where("status = ?", "pending").
		Where("requester_id <> ?", userID).
		Where("user_id = ? OR friend_id = ?", userID, userID).
		Preload("Requester").
		Preload("User").
		Preload("Friend").
		Order("created_at desc").
		Find(&friendships).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, friendships)
}

func (s *Server) handleListOutgoingFriendRequests(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var friendships []models.Friendship
	if err := s.db.
		Where("status = ?", "pending").
		Where("requester_id = ?", userID).
		Preload("Requester").
		Preload("User").
		Preload("Friend").
		Order("created_at desc").
		Find(&friendships).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, friendships)
}

func (s *Server) handleListFriends(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var friendships []models.Friendship
	if err := s.db.
		Where("status = ?", "accepted").
		Where("user_id = ? OR friend_id = ?", userID, userID).
		Preload("Requester").
		Preload("User").
		Preload("Friend").
		Order("updated_at desc").
		Find(&friendships).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, friendships)
}

func (s *Server) handleAcceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	friendshipID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req acceptFriendRequestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	actorUserID, err := parseRequiredUUID("actor_user_id", req.ActorUserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var friendship models.Friendship
	if err := s.db.First(&friendship, "id = ?", friendshipID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("friend request not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if actorUserID != friendship.UserID && actorUserID != friendship.FriendID {
		writeError(w, http.StatusForbidden, errors.New("actor must be part of this friendship"))
		return
	}
	if actorUserID == friendship.RequesterID {
		writeError(w, http.StatusForbidden, errors.New("requester cannot accept their own request"))
		return
	}

	friendship.Status = "accepted"
	if err := s.db.Save(&friendship).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, friendship)
}

func (s *Server) handleDeleteFriendship(w http.ResponseWriter, r *http.Request) {
	friendshipID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result := s.db.Delete(&models.Friendship{}, "id = ?", friendshipID)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("friendship not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func canonicalizeUserPair(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if a.String() <= b.String() {
		return a, b
	}
	return b, a
}
