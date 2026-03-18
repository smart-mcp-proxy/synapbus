package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/synapbus/synapbus/internal/agents"
	"github.com/synapbus/synapbus/internal/auth"
	"github.com/synapbus/synapbus/internal/messaging"
	"github.com/synapbus/synapbus/internal/reactions"
)

// ReactionsHandler handles REST API requests for message reactions.
type ReactionsHandler struct {
	reactionService *reactions.Service
	msgService      *messaging.MessagingService
	agentService    *agents.AgentService
	logger          *slog.Logger
}

// NewReactionsHandler creates a new reactions handler.
func NewReactionsHandler(reactionService *reactions.Service, msgService *messaging.MessagingService, agentService *agents.AgentService) *ReactionsHandler {
	return &ReactionsHandler{
		reactionService: reactionService,
		msgService:      msgService,
		agentService:    agentService,
		logger:          slog.Default().With("component", "api.reactions"),
	}
}

// Toggle handles POST /api/messages/{id}/reactions.
func (h *ReactionsHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := OwnerIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorBody("unauthorized", "Authentication required"))
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("invalid_id", "Invalid message ID"))
		return
	}

	var req struct {
		Reaction string          `json:"reaction"`
		Metadata json.RawMessage `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("invalid_request", "Invalid JSON body"))
		return
	}

	if req.Reaction == "" {
		writeJSON(w, http.StatusBadRequest, errorBody("validation_error", "Reaction type is required"))
		return
	}

	// Verify the message exists
	msg, err := h.msgService.GetMessageByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorBody("not_found", "Message not found"))
		return
	}

	// Determine the acting agent name from the session
	agentName, err := h.resolveAgentName(r, ownerID, msg)
	if err != nil {
		h.logger.Error("resolve agent name failed", "error", err)
		writeJSON(w, http.StatusBadRequest, errorBody("no_agent", err.Error()))
		return
	}

	result, err := h.reactionService.Toggle(r.Context(), id, agentName, req.Reaction, req.Metadata)
	if err != nil {
		if err == reactions.ErrInvalidReaction {
			writeJSON(w, http.StatusBadRequest, errorBody("invalid_reaction", err.Error()))
			return
		}
		if err == reactions.ErrReactionLimit {
			writeJSON(w, http.StatusBadRequest, errorBody("reaction_limit", err.Error()))
			return
		}
		h.logger.Error("toggle reaction failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("server_error", "Failed to toggle reaction"))
		return
	}

	// Reload reactions and workflow state for the response
	rxs, state, err := h.reactionService.GetReactions(r.Context(), id)
	if err != nil {
		h.logger.Error("get reactions failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("server_error", "Failed to get reactions"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"action":         result.Action,
		"reaction":       result.Reaction,
		"reactions":      rxs,
		"workflow_state": state,
	})
}

// GetReactions handles GET /api/messages/{id}/reactions.
func (h *ReactionsHandler) GetReactions(w http.ResponseWriter, r *http.Request) {
	_, ok := OwnerIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorBody("unauthorized", "Authentication required"))
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("invalid_id", "Invalid message ID"))
		return
	}

	// Verify the message exists
	_, err = h.msgService.GetMessageByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorBody("not_found", "Message not found"))
		return
	}

	rxs, state, err := h.reactionService.GetReactions(r.Context(), id)
	if err != nil {
		h.logger.Error("get reactions failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("server_error", "Failed to get reactions"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"reactions":      rxs,
		"workflow_state": state,
	})
}

// Remove handles DELETE /api/messages/{id}/reactions/{reaction}.
func (h *ReactionsHandler) Remove(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := OwnerIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorBody("unauthorized", "Authentication required"))
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("invalid_id", "Invalid message ID"))
		return
	}

	reactionType := chi.URLParam(r, "reaction")
	if reactionType == "" {
		writeJSON(w, http.StatusBadRequest, errorBody("validation_error", "Reaction type is required"))
		return
	}

	// Verify the message exists
	msg, err := h.msgService.GetMessageByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorBody("not_found", "Message not found"))
		return
	}

	// Determine the acting agent name
	agentName, err := h.resolveAgentName(r, ownerID, msg)
	if err != nil {
		h.logger.Error("resolve agent name failed", "error", err)
		writeJSON(w, http.StatusBadRequest, errorBody("no_agent", err.Error()))
		return
	}

	if err := h.reactionService.Remove(r.Context(), id, agentName, reactionType); err != nil {
		if err == reactions.ErrInvalidReaction {
			writeJSON(w, http.StatusBadRequest, errorBody("invalid_reaction", err.Error()))
			return
		}
		h.logger.Error("remove reaction failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("server_error", "Failed to remove reaction"))
		return
	}

	// Reload reactions and workflow state for the response
	rxs, state, err := h.reactionService.GetReactions(r.Context(), id)
	if err != nil {
		h.logger.Error("get reactions failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("server_error", "Failed to get reactions"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "removed",
		"reactions":      rxs,
		"workflow_state": state,
	})
}

// resolveAgentName determines the agent name for the current session user.
// For session-authenticated users (Web UI), it returns the human agent.
// For API key / OAuth, it falls back to the first owned agent.
func (h *ReactionsHandler) resolveAgentName(r *http.Request, ownerID int64, msg *messaging.Message) (string, error) {
	if _, isSession := auth.SessionIDFromContext(r.Context()); isSession {
		humanAgent, err := h.agentService.GetHumanAgentForUser(r.Context(), ownerID)
		if err != nil {
			return "", err
		}
		if humanAgent == nil {
			return "", fmt.Errorf("no human agent found for this user")
		}
		return humanAgent.Name, nil
	}

	// Non-session: find an owned agent
	ownedAgents, err := h.agentService.ListAgents(r.Context(), ownerID)
	if err != nil || len(ownedAgents) == 0 {
		return "", fmt.Errorf("no agents registered")
	}

	// Prefer human-type agent
	for _, a := range ownedAgents {
		if a.Type == "human" {
			return a.Name, nil
		}
	}
	return ownedAgents[0].Name, nil
}
