package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/synapbus/synapbus/internal/agents"
	"github.com/synapbus/synapbus/internal/reactor"
)

// RunsHandler handles REST API requests for reactive runs.
type RunsHandler struct {
	store      *reactor.Store
	reactor    *reactor.Reactor
	agentStore agents.AgentStore
}

// NewRunsHandler creates a new runs handler.
func NewRunsHandler(store *reactor.Store, r *reactor.Reactor, agentStore agents.AgentStore) *RunsHandler {
	return &RunsHandler{
		store:      store,
		reactor:    r,
		agentStore: agentStore,
	}
}

// ListRuns returns reactive runs with optional filters.
func (h *RunsHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	agentName := r.URL.Query().Get("agent")
	status := r.URL.Query().Get("status")
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	runs, total, err := h.store.ListRuns(r.Context(), agentName, status, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody("internal_error", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"runs":  runs,
		"total": total,
	})
}

// GetRun returns a single run by ID.
func (h *RunsHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("bad_request", "invalid run ID"))
		return
	}

	run, err := h.store.GetRunByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorBody("not_found", "run not found"))
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// RetryRun retries a failed run.
func (h *RunsHandler) RetryRun(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("bad_request", "invalid run ID"))
		return
	}

	newRun, err := h.reactor.RetryRun(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("retry_failed", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"new_run_id": newRun.ID,
		"status":     newRun.Status,
	})
}

// ReactiveAgents returns agents with reactive trigger config and current status.
func (h *RunsHandler) ReactiveAgents(w http.ResponseWriter, r *http.Request) {
	agentsList, err := h.agentStore.ListReactiveAgents(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody("internal_error", err.Error()))
		return
	}

	type agentStatus struct {
		Name               string  `json:"name"`
		TriggerMode        string  `json:"trigger_mode"`
		CooldownSeconds    int     `json:"cooldown_seconds"`
		DailyTriggerBudget int     `json:"daily_trigger_budget"`
		MaxTriggerDepth    int     `json:"max_trigger_depth"`
		K8sImage           string  `json:"k8s_image"`
		PendingWork        bool    `json:"pending_work"`
		State              string  `json:"state"`
		TodayRuns          int     `json:"today_runs"`
		CooldownUntil      *string `json:"cooldown_until"`
	}

	result := make([]agentStatus, 0, len(agentsList))
	for _, a := range agentsList {
		as := agentStatus{
			Name:               a.Name,
			TriggerMode:        a.TriggerMode,
			CooldownSeconds:    a.CooldownSeconds,
			DailyTriggerBudget: a.DailyTriggerBudget,
			MaxTriggerDepth:    a.MaxTriggerDepth,
			K8sImage:           a.K8sImage,
			PendingWork:        a.PendingWork,
		}

		// Compute state
		todayCount, _ := h.store.CountTodayRuns(r.Context(), a.Name)
		as.TodayRuns = todayCount

		running, _ := h.store.IsAgentRunning(r.Context(), a.Name)
		if running {
			as.State = "running"
		} else if a.PendingWork {
			as.State = "queued"
		} else if todayCount >= a.DailyTriggerBudget {
			as.State = "budget_exhausted"
		} else {
			lastRun, _ := h.store.GetLastRunTime(r.Context(), a.Name)
			if lastRun != nil {
				cooldownEnd := lastRun.Add(time.Duration(a.CooldownSeconds) * time.Second)
				if time.Now().Before(cooldownEnd) {
					as.State = "cooldown"
					t := cooldownEnd.UTC().Format(time.RFC3339)
					as.CooldownUntil = &t
				} else {
					as.State = "idle"
				}
			} else {
				as.State = "idle"
			}
		}

		result = append(result, as)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"agents": result,
	})
}
