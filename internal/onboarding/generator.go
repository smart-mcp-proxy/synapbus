package onboarding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// GeneratorConfig holds the parameters for generating a CLAUDE.md file.
type GeneratorConfig struct {
	AgentName   string
	Archetype   string
	OwnerName   string
	SynapBusURL string
	APIKey      string
	Channels    []ChannelInfo
}

// ChannelInfo describes a channel for the template.
type ChannelInfo struct {
	Name        string
	Description string
}

// ArchetypeInfo describes an available archetype.
type ArchetypeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// archetypeDescriptions maps archetype names to human-readable descriptions.
var archetypeDescriptions = map[string]string{
	"researcher": "research and discovery",
	"writer":     "content creation and publishing",
	"commenter":  "community engagement",
	"monitor":    "monitoring and alerting",
	"operator":   "deployment and operations",
	"custom":     "general purpose",
}

// archetypeTemplates maps archetype names to their specific template sections.
var archetypeTemplates = map[string]string{
	"researcher": researcherTemplate,
	"writer":     writerTemplate,
	"commenter":  commenterTemplate,
	"monitor":    monitorTemplate,
	"operator":   operatorTemplate,
	"custom":     customTemplate,
}

// templateData is the data passed to templates during rendering.
type templateData struct {
	AgentName            string
	Archetype            string
	ArchetypeDescription string
	OwnerName            string
	SynapBusURL          string
	Channels             []ChannelInfo
}

// GenerateCLAUDEMD renders the CLAUDE.md template for the given archetype.
func GenerateCLAUDEMD(config GeneratorConfig) (string, error) {
	archetype := strings.ToLower(config.Archetype)
	if archetype == "" {
		archetype = "custom"
	}

	description, ok := archetypeDescriptions[archetype]
	if !ok {
		return "", fmt.Errorf("unknown archetype: %s", config.Archetype)
	}

	archetypeSection, ok := archetypeTemplates[archetype]
	if !ok {
		return "", fmt.Errorf("no template for archetype: %s", config.Archetype)
	}

	// Combine common + archetype-specific template
	fullTemplate := commonTemplate + archetypeSection

	tmpl, err := template.New("claude-md").Parse(fullTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	data := templateData{
		AgentName:            config.AgentName,
		Archetype:            archetype,
		ArchetypeDescription: description,
		OwnerName:            config.OwnerName,
		SynapBusURL:          config.SynapBusURL,
		Channels:             config.Channels,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// GenerateMCPConfig returns a JSON snippet for Claude Code MCP settings.
func GenerateMCPConfig(synapbusURL, apiKey string) string {
	config := map[string]any{
		"mcpServers": map[string]any{
			"synapbus": map[string]any{
				"type": "streamable-http",
				"url":  strings.TrimRight(synapbusURL, "/") + "/mcp",
				"headers": map[string]string{
					"Authorization": "Bearer " + apiKey,
				},
			},
		},
	}

	b, _ := json.MarshalIndent(config, "", "  ")
	return string(b)
}

// ListArchetypes returns all available archetypes with their descriptions.
func ListArchetypes() []ArchetypeInfo {
	return []ArchetypeInfo{
		{Name: "researcher", Description: "Web search, platform discovery, finding deduplication, news channel posting"},
		{Name: "writer", Description: "Content creation, blog publishing, editing, draft-review-publish pipeline"},
		{Name: "commenter", Description: "Community engagement, comment drafting, tone guidelines, approval workflow"},
		{Name: "monitor", Description: "Diff checking, alert thresholds, audit skills, change detection"},
		{Name: "operator", Description: "Deployment, incident response, system commands, infrastructure tasks"},
		{Name: "custom", Description: "Minimal template with common sections only -- user fills in the rest"},
	}
}
