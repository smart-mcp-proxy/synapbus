package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/synapbus/synapbus/internal/agents"
	"github.com/synapbus/synapbus/internal/channels"
	"github.com/synapbus/synapbus/internal/onboarding"
)

// OnboardingHandler handles REST API requests for agent onboarding.
type OnboardingHandler struct {
	agentService   *agents.AgentService
	channelService *channels.Service
	baseURL        string
	logger         *slog.Logger
}

// NewOnboardingHandler creates a new onboarding handler.
func NewOnboardingHandler(agentService *agents.AgentService, channelService *channels.Service, baseURL string) *OnboardingHandler {
	return &OnboardingHandler{
		agentService:   agentService,
		channelService: channelService,
		baseURL:        baseURL,
		logger:         slog.Default().With("component", "api.onboarding"),
	}
}

// GetCLAUDEMD handles GET /api/agents/{name}/claude-md?archetype=researcher
// Returns a rendered CLAUDE.md for the given agent and archetype.
func (h *OnboardingHandler) GetCLAUDEMD(w http.ResponseWriter, r *http.Request) {
	agentName := chi.URLParam(r, "name")
	archetype := r.URL.Query().Get("archetype")
	if archetype == "" {
		archetype = "custom"
	}

	// Look up the agent to get display info
	ownerName := "owner"
	agent, err := h.agentService.GetAgent(r.Context(), agentName)
	if err != nil {
		// Agent doesn't need to exist -- we generate for any name
		h.logger.Debug("agent not found, using defaults", "name", agentName, "error", err)
	} else {
		if agent.DisplayName != "" {
			agentName = agent.DisplayName
		}
	}

	// Collect channel info
	var channelInfos []onboarding.ChannelInfo
	if h.channelService != nil {
		// List all channels (use empty agent name to get all public channels)
		chList, err := h.channelService.ListChannels(r.Context(), "")
		if err != nil {
			h.logger.Warn("failed to list channels", "error", err)
		} else {
			for _, ch := range chList {
				if ch.IsSystem {
					continue
				}
				channelInfos = append(channelInfos, onboarding.ChannelInfo{
					Name:        ch.Name,
					Description: ch.Description,
				})
			}
		}
	}

	config := onboarding.GeneratorConfig{
		AgentName:   agentName,
		Archetype:   archetype,
		OwnerName:   ownerName,
		SynapBusURL: h.baseURL,
		Channels:    channelInfos,
	}

	md, err := onboarding.GenerateCLAUDEMD(config)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("invalid_archetype", err.Error()))
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(md))
}

// GetMCPConfig handles GET /api/agents/{name}/mcp-config
// Returns a JSON MCP config snippet for Claude Code settings.
func (h *OnboardingHandler) GetMCPConfig(w http.ResponseWriter, r *http.Request) {
	agentName := chi.URLParam(r, "name")

	// Verify the agent exists
	_, err := h.agentService.GetAgent(r.Context(), agentName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorBody("not_found", "Agent not found: "+agentName))
		return
	}

	// Use a placeholder for the API key since we cannot recover the raw key
	apiKeyPlaceholder := "<YOUR_API_KEY>"

	config := onboarding.GenerateMCPConfig(h.baseURL, apiKeyPlaceholder)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(config))
}

// ListArchetypes handles GET /api/archetypes
// Returns the list of available agent archetypes.
func (h *OnboardingHandler) ListArchetypes(w http.ResponseWriter, r *http.Request) {
	archetypes := onboarding.ListArchetypes()
	writeJSON(w, http.StatusOK, map[string]any{
		"archetypes": archetypes,
	})
}

// ListSkills handles GET /api/skills
// Returns the list of available agent skills.
func (h *OnboardingHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := onboarding.ListSkills()
	if err != nil {
		h.logger.Error("failed to list skills", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("server_error", "Failed to list skills"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"skills": skills,
	})
}

// GetSkill handles GET /api/skills/{name}
// Returns the markdown content of a skill.
func (h *OnboardingHandler) GetSkill(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	content, err := onboarding.GetSkill(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorBody("not_found", err.Error()))
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content))
}
