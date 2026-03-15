// Package actions defines the catalog of available actions for the execute tool.
// Actions are searchable via BM25 and map to service method calls via the bridge.
package actions

// Param describes a single parameter for an action.
type Param struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "number", "boolean", "object"
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

// Action describes a callable operation available through the execute tool.
type Action struct {
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Params      []Param `json:"params"`
	Example     string  `json:"example"`
}

// Registry holds all registered actions.
type Registry struct {
	actions []Action
	byName  map[string]*Action
}

// NewRegistry creates a registry populated with all SynapBus actions.
func NewRegistry() *Registry {
	r := &Registry{
		byName: make(map[string]*Action),
	}
	r.registerAll()
	return r
}

// List returns all registered actions.
func (r *Registry) List() []Action {
	return r.actions
}

// Get returns an action by name, or nil if not found.
func (r *Registry) Get(name string) *Action {
	return r.byName[name]
}

func (r *Registry) add(a Action) {
	r.actions = append(r.actions, a)
	r.byName[a.Name] = &r.actions[len(r.actions)-1]
}

func (r *Registry) registerAll() {
	// --- Messaging ---
	r.add(Action{
		Name:        "read_inbox",
		Category:    "messaging",
		Description: "Check your message inbox for pending messages. Returns unread/pending direct messages addressed to you.",
		Params: []Param{
			{Name: "limit", Type: "number", Description: "Maximum number of messages to return (default 50)"},
			{Name: "status_filter", Type: "string", Description: "Filter by message status: pending, processing, done, failed"},
			{Name: "include_read", Type: "boolean", Description: "Include previously read messages (default false)"},
			{Name: "min_priority", Type: "number", Description: "Minimum priority filter (1-10)"},
			{Name: "from_agent", Type: "string", Description: "Filter by sender agent name"},
		},
		Example: `call("read_inbox", { limit: 10 })`,
	})

	r.add(Action{
		Name:        "claim_messages",
		Category:    "messaging",
		Description: "Atomically claim pending messages for processing. Claimed messages transition to 'processing' status so no other agent processes them.",
		Params: []Param{
			{Name: "limit", Type: "number", Description: "Maximum number of messages to claim (default 10)"},
		},
		Example: `call("claim_messages", { limit: 5 })`,
	})

	r.add(Action{
		Name:        "mark_done",
		Category:    "messaging",
		Description: "Mark a claimed message as done or failed.",
		Params: []Param{
			{Name: "message_id", Type: "number", Description: "ID of the message to mark", Required: true},
			{Name: "status", Type: "string", Description: "New status: 'done' or 'failed' (default 'done')"},
			{Name: "reason", Type: "string", Description: "Failure reason (only for status='failed')"},
		},
		Example: `call("mark_done", { message_id: 42, status: "done" })`,
	})

	r.add(Action{
		Name:        "search_messages",
		Category:    "messaging",
		Description: "Search for messages across your inbox and channels. Supports full-text and semantic search.",
		Params: []Param{
			{Name: "query", Type: "string", Description: "Search query string — supports natural language for semantic search"},
			{Name: "limit", Type: "number", Description: "Maximum results to return (default 10, max 100)"},
			{Name: "min_priority", Type: "number", Description: "Minimum priority filter (1-10)"},
			{Name: "from_agent", Type: "string", Description: "Filter by sender agent name"},
			{Name: "status", Type: "string", Description: "Filter by message status"},
			{Name: "search_mode", Type: "string", Description: "Search mode: 'auto' (default), 'semantic', or 'fulltext'"},
		},
		Example: `call("search_messages", { query: "deployment status", limit: 5 })`,
	})

	r.add(Action{
		Name:        "discover_agents",
		Category:    "messaging",
		Description: "Discover other agents on the bus. Find agents you can communicate with, optionally filtered by capability.",
		Params: []Param{
			{Name: "query", Type: "string", Description: "Capability keyword to search for"},
		},
		Example: `call("discover_agents", {})`,
	})

	// --- Channels ---
	r.add(Action{
		Name:        "create_channel",
		Category:    "channels",
		Description: "Create a new channel for group communication.",
		Params: []Param{
			{Name: "name", Type: "string", Description: "Unique channel name (alphanumeric, hyphens, underscores, max 64 chars)", Required: true},
			{Name: "description", Type: "string", Description: "Channel description"},
			{Name: "topic", Type: "string", Description: "Current channel topic"},
			{Name: "type", Type: "string", Description: "Channel type: 'standard', 'blackboard', or 'auction' (default 'standard')"},
			{Name: "is_private", Type: "boolean", Description: "Whether the channel is private (invite-only). Default false"},
		},
		Example: `call("create_channel", { name: "dev-ops", description: "DevOps coordination" })`,
	})

	r.add(Action{
		Name:        "join_channel",
		Category:    "channels",
		Description: "Join a channel to participate in group conversations.",
		Params: []Param{
			{Name: "channel_id", Type: "number", Description: "ID of the channel to join"},
			{Name: "channel_name", Type: "string", Description: "Name of the channel to join (alternative to channel_id)"},
		},
		Example: `call("join_channel", { channel_name: "general" })`,
	})

	r.add(Action{
		Name:        "leave_channel",
		Category:    "channels",
		Description: "Leave a channel you are a member of.",
		Params: []Param{
			{Name: "channel_id", Type: "number", Description: "ID of the channel to leave"},
			{Name: "channel_name", Type: "string", Description: "Name of the channel to leave (alternative to channel_id)"},
		},
		Example: `call("leave_channel", { channel_name: "old-project" })`,
	})

	r.add(Action{
		Name:        "list_channels",
		Category:    "channels",
		Description: "List all channels visible to you. Shows public channels and private channels you are a member of.",
		Params:      []Param{},
		Example:     `call("list_channels", {})`,
	})

	r.add(Action{
		Name:        "invite_to_channel",
		Category:    "channels",
		Description: "Invite an agent to a channel (only the channel owner can invite to private channels).",
		Params: []Param{
			{Name: "channel_id", Type: "number", Description: "ID of the channel"},
			{Name: "channel_name", Type: "string", Description: "Name of the channel (alternative to channel_id)"},
			{Name: "agent_name", Type: "string", Description: "Name of the agent to invite", Required: true},
		},
		Example: `call("invite_to_channel", { channel_name: "dev-ops", agent_name: "deploy-bot" })`,
	})

	r.add(Action{
		Name:        "kick_from_channel",
		Category:    "channels",
		Description: "Remove an agent from a channel (only the channel owner can kick).",
		Params: []Param{
			{Name: "channel_id", Type: "number", Description: "ID of the channel"},
			{Name: "channel_name", Type: "string", Description: "Name of the channel (alternative to channel_id)"},
			{Name: "agent_name", Type: "string", Description: "Name of the agent to kick", Required: true},
		},
		Example: `call("kick_from_channel", { channel_name: "dev-ops", agent_name: "old-bot" })`,
	})

	r.add(Action{
		Name:        "get_channel_messages",
		Category:    "channels",
		Description: "Get recent messages from a channel you are a member of.",
		Params: []Param{
			{Name: "channel_id", Type: "number", Description: "ID of the channel"},
			{Name: "channel_name", Type: "string", Description: "Name of the channel (alternative to channel_id)"},
			{Name: "limit", Type: "number", Description: "Max number of messages to return (default 50, max 200)"},
		},
		Example: `call("get_channel_messages", { channel_name: "general", limit: 20 })`,
	})

	r.add(Action{
		Name:        "send_channel_message",
		Category:    "channels",
		Description: "Send a message to all members of a channel. Use @agentname to mention specific agents.",
		Params: []Param{
			{Name: "channel_id", Type: "number", Description: "ID of the channel"},
			{Name: "channel_name", Type: "string", Description: "Name of the channel (alternative to channel_id)"},
			{Name: "body", Type: "string", Description: "Message body text", Required: true},
			{Name: "priority", Type: "number", Description: "Message priority (1-10, default 5)"},
			{Name: "metadata", Type: "string", Description: "JSON metadata object (optional)"},
		},
		Example: `call("send_channel_message", { channel_name: "general", body: "Hello team!" })`,
	})

	r.add(Action{
		Name:        "update_channel",
		Category:    "channels",
		Description: "Update channel topic or description (only the channel owner can update).",
		Params: []Param{
			{Name: "channel_id", Type: "number", Description: "ID of the channel"},
			{Name: "channel_name", Type: "string", Description: "Name of the channel (alternative to channel_id)"},
			{Name: "topic", Type: "string", Description: "New channel topic"},
			{Name: "description", Type: "string", Description: "New channel description"},
		},
		Example: `call("update_channel", { channel_name: "dev-ops", topic: "Sprint 42" })`,
	})

	// --- Swarm ---
	r.add(Action{
		Name:        "post_task",
		Category:    "swarm",
		Description: "Post a task to an auction channel for agents to bid on.",
		Params: []Param{
			{Name: "channel_name", Type: "string", Description: "Name of the auction channel", Required: true},
			{Name: "title", Type: "string", Description: "Task title", Required: true},
			{Name: "description", Type: "string", Description: "Task description"},
			{Name: "requirements", Type: "string", Description: "JSON object of task requirements"},
			{Name: "deadline", Type: "string", Description: "Task deadline in ISO 8601 format"},
		},
		Example: `call("post_task", { channel_name: "tasks", title: "Analyze logs", description: "Find anomalies in the last 24h" })`,
	})

	r.add(Action{
		Name:        "bid_task",
		Category:    "swarm",
		Description: "Submit a bid on an open task in an auction channel.",
		Params: []Param{
			{Name: "task_id", Type: "number", Description: "ID of the task to bid on", Required: true},
			{Name: "capabilities", Type: "string", Description: "JSON object describing your relevant capabilities"},
			{Name: "time_estimate", Type: "string", Description: "Estimated time to complete the task"},
			{Name: "message", Type: "string", Description: "Message to the task poster explaining your bid"},
		},
		Example: `call("bid_task", { task_id: 1, message: "I can do this in 10 minutes" })`,
	})

	r.add(Action{
		Name:        "accept_bid",
		Category:    "swarm",
		Description: "Accept a bid on a task you posted, assigning the task to the bidding agent.",
		Params: []Param{
			{Name: "task_id", Type: "number", Description: "ID of the task", Required: true},
			{Name: "bid_id", Type: "number", Description: "ID of the bid to accept", Required: true},
		},
		Example: `call("accept_bid", { task_id: 1, bid_id: 3 })`,
	})

	r.add(Action{
		Name:        "complete_task",
		Category:    "swarm",
		Description: "Mark a task as completed (only the assigned agent can do this).",
		Params: []Param{
			{Name: "task_id", Type: "number", Description: "ID of the task to complete", Required: true},
		},
		Example: `call("complete_task", { task_id: 1 })`,
	})

	r.add(Action{
		Name:        "list_tasks",
		Category:    "swarm",
		Description: "List tasks in an auction channel, optionally filtered by status.",
		Params: []Param{
			{Name: "channel_name", Type: "string", Description: "Name of the auction channel", Required: true},
			{Name: "status", Type: "string", Description: "Filter by task status: open, assigned, completed, cancelled"},
		},
		Example: `call("list_tasks", { channel_name: "tasks", status: "open" })`,
	})

	// --- Attachments ---
	r.add(Action{
		Name:        "upload_attachment",
		Category:    "attachments",
		Description: "Upload a file attachment. Content must be base64-encoded. Returns SHA-256 hash for retrieval. Max 50MB.",
		Params: []Param{
			{Name: "content", Type: "string", Description: "Base64-encoded file content", Required: true},
			{Name: "filename", Type: "string", Description: "Original filename (optional)"},
			{Name: "mime_type", Type: "string", Description: "MIME type override (optional, auto-detected)"},
			{Name: "message_id", Type: "number", Description: "Message ID to attach the file to (optional)"},
		},
		Example: `call("upload_attachment", { content: btoa("hello"), filename: "hello.txt" })`,
	})

	r.add(Action{
		Name:        "download_attachment",
		Category:    "attachments",
		Description: "Download an attachment by its SHA-256 hash. Returns base64-encoded content with metadata.",
		Params: []Param{
			{Name: "hash", Type: "string", Description: "SHA-256 hash of the attachment", Required: true},
		},
		Example: `call("download_attachment", { hash: "abc123..." })`,
	})
}
