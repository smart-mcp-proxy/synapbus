package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/synapbus/synapbus/internal/agents"
	"github.com/synapbus/synapbus/internal/harness/runs"
	"github.com/synapbus/synapbus/internal/messaging"
	"github.com/synapbus/synapbus/internal/reactor"
)

// RunsHandler handles REST API requests for reactive runs.
type RunsHandler struct {
	store          *reactor.Store
	reactor        *reactor.Reactor
	agentStore     agents.AgentStore
	harnessRuns    *runs.Store
	msgService     *messaging.MessagingService
	db             *sql.DB
}

// NewRunsHandler creates a new runs handler.
func NewRunsHandler(
	store *reactor.Store,
	r *reactor.Reactor,
	agentStore agents.AgentStore,
	harnessRuns *runs.Store,
	msgService *messaging.MessagingService,
	db *sql.DB,
) *RunsHandler {
	return &RunsHandler{
		store:       store,
		reactor:     r,
		agentStore:  agentStore,
		harnessRuns: harnessRuns,
		msgService:  msgService,
		db:          db,
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

// GetRun returns a composite view of a reactive run: the reactive_runs
// row itself, the linked harness_runs row (with captured prompt /
// response / usage), the triggering message, the outgoing message the
// agent produced (if any), and a snapshot of the agent's current
// harness config. Everything the Web UI needs to render "what happened
// on this run" in a single request.
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

	resp := map[string]any{
		"run": run,
	}

	// Linked harness_run (may be nil for K8s path which still uses
	// the legacy reactive_runs-only flow).
	if h.harnessRuns != nil {
		hr, _ := h.harnessRuns.GetByReactiveRunID(r.Context(), id)
		if hr != nil {
			resp["harness_run"] = hr
		}
	}

	// Triggering message body (what the sender wrote).
	if run.TriggerMessageID != nil && h.msgService != nil {
		if msg, err := h.msgService.GetMessageByID(r.Context(), *run.TriggerMessageID); err == nil && msg != nil {
			resp["trigger_message"] = msg
		}
	}

	// Agent snapshot — current harness config so the UI can show
	// the gemini_md / claude_md the agent is currently running with.
	if agent, err := h.agentStore.GetAgentByName(r.Context(), run.AgentName); err == nil && agent != nil {
		resp["agent"] = map[string]any{
			"name":                agent.Name,
			"display_name":        agent.DisplayName,
			"type":                agent.Type,
			"harness_name":        agent.HarnessName,
			"local_command":       agent.LocalCommand,
			"harness_config_json": agent.HarnessConfigJSON,
			"trigger_mode":        agent.TriggerMode,
			"cooldown_seconds":    agent.CooldownSeconds,
			"daily_trigger_budget": agent.DailyTriggerBudget,
			"max_trigger_depth":   agent.MaxTriggerDepth,
		}
	}

	// Outgoing message — the first DM this agent produced after
	// the run started. We find it by querying messages where
	// from_agent = this run's agent AND created_at >= run.StartedAt,
	// ordered by id. Works for both success and failure cases.
	if h.db != nil && run.StartedAt != nil {
		var (
			msgID     int64
			toAgent   sql.NullString
			body      string
			status    string
			createdAt string
		)
		// Wrap both sides in datetime() so SQLite parses and compares
		// canonically — the messages table stores created_at as
		// 'YYYY-MM-DD HH:MM:SS' (space separator) while Go emits
		// RFC3339 with 'T'. A raw string comparison fails silently.
		err := h.db.QueryRowContext(r.Context(),
			`SELECT id, to_agent, body, status, created_at
			   FROM messages
			  WHERE from_agent = ?
			    AND datetime(created_at) >= datetime(?)
			  ORDER BY id ASC LIMIT 1`,
			run.AgentName,
			run.StartedAt.UTC().Format(time.RFC3339),
		).Scan(&msgID, &toAgent, &body, &status, &createdAt)
		if err == nil {
			resp["outgoing_message"] = map[string]any{
				"id":         msgID,
				"to_agent":   toAgent.String,
				"body":       body,
				"status":     status,
				"created_at": createdAt,
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
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
