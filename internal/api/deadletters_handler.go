package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/synapbus/synapbus/internal/messaging"
)

// DeadLettersHandler handles REST API requests for the dead letter queue.
type DeadLettersHandler struct {
	deadLetterStore *messaging.DeadLetterStore
	logger          *slog.Logger
}

// NewDeadLettersHandler creates a new dead letters handler.
func NewDeadLettersHandler(dls *messaging.DeadLetterStore) *DeadLettersHandler {
	return &DeadLettersHandler{
		deadLetterStore: dls,
		logger:          slog.Default().With("component", "api.deadletters"),
	}
}

// List handles GET /api/dead-letters.
func (h *DeadLettersHandler) List(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := OwnerIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorBody("unauthorized", "Authentication required"))
		return
	}

	// Parse query params
	includeAcknowledged := r.URL.Query().Get("acknowledged") == "true"

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	letters, total, err := h.deadLetterStore.ListDeadLetters(r.Context(), ownerID, includeAcknowledged, limit)
	if err != nil {
		h.logger.Error("list dead letters failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("server_error", "Failed to list dead letters"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dead_letters": letters,
		"total":        total,
	})
}

// Acknowledge handles POST /api/dead-letters/{id}/acknowledge.
func (h *DeadLettersHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := OwnerIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorBody("unauthorized", "Authentication required"))
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("invalid_id", "Invalid dead letter ID"))
		return
	}

	if err := h.deadLetterStore.AcknowledgeDeadLetter(r.Context(), id, ownerID); err != nil {
		h.logger.Error("acknowledge dead letter failed", "error", err)
		writeJSON(w, http.StatusBadRequest, errorBody("acknowledge_failed", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"acknowledged": true})
}

// Count handles GET /api/dead-letters/count.
func (h *DeadLettersHandler) Count(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := OwnerIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorBody("unauthorized", "Authentication required"))
		return
	}

	count, err := h.deadLetterStore.CountUnacknowledged(r.Context(), ownerID)
	if err != nil {
		h.logger.Error("count dead letters failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("server_error", "Failed to count dead letters"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}
