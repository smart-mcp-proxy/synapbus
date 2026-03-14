package agents

import (
	"context"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/synapbus/synapbus/internal/channels"
	"github.com/synapbus/synapbus/internal/messaging"
	"github.com/synapbus/synapbus/internal/trace"
)

func newTestService(t *testing.T) *AgentService {
	t.Helper()
	db := newTestDB(t)
	store := NewSQLiteAgentStore(db)
	tracer := trace.NewTracer(db)
	t.Cleanup(func() { tracer.Close() })

	return NewAgentService(store, tracer)
}

func TestAgentService_Register(t *testing.T) {
	tests := []struct {
		name         string
		agentName    string
		displayName  string
		agentType    string
		capabilities json.RawMessage
		ownerID      int64
		wantErr      bool
	}{
		{
			name:         "successful registration",
			agentName:    "test-bot",
			displayName:  "Test Bot",
			agentType:    "ai",
			capabilities: json.RawMessage(`{"skills":["testing"]}`),
			ownerID:      1,
		},
		{
			name:    "empty name fails",
			agentName: "",
			wantErr:   true,
		},
		{
			name:      "invalid type",
			agentName: "invalid-type",
			agentType: "robot",
			ownerID:   1,
			wantErr:   true,
		},
		{
			name:         "invalid capabilities JSON",
			agentName:    "bad-caps",
			agentType:    "ai",
			capabilities: json.RawMessage("not json"),
			ownerID:      1,
			wantErr:      true,
		},
		{
			name:      "default type is ai",
			agentName: "default-type",
			ownerID:   1,
		},
		{
			name:      "human type is valid",
			agentName: "human-agent",
			agentType: "human",
			ownerID:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t)
			ctx := context.Background()

			agent, apiKey, err := svc.Register(ctx, tt.agentName, tt.displayName, tt.agentType, tt.capabilities, tt.ownerID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if agent.ID == 0 {
				t.Error("agent ID should not be 0")
			}
			if apiKey == "" {
				t.Error("API key should not be empty")
			}
			if len(apiKey) < 32 {
				t.Errorf("API key too short: %d chars", len(apiKey))
			}
			if agent.Status != AgentStatusActive {
				t.Errorf("status = %s, want %s", agent.Status, AgentStatusActive)
			}
		})
	}
}

func TestAgentService_Authenticate(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, apiKey, err := svc.Register(ctx, "auth-bot", "Auth Bot", "ai", nil, 1)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	t.Run("valid key", func(t *testing.T) {
		agent, err := svc.Authenticate(ctx, apiKey)
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if agent.Name != "auth-bot" {
			t.Errorf("Name = %s, want auth-bot", agent.Name)
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		_, err := svc.Authenticate(ctx, "invalid-key")
		if err == nil {
			t.Error("expected error for invalid key")
		}
	})
}

func TestAgentService_GetAgent(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.Register(ctx, "get-bot", "Get Bot", "ai", nil, 1)

	t.Run("existing agent", func(t *testing.T) {
		agent, err := svc.GetAgent(ctx, "get-bot")
		if err != nil {
			t.Fatalf("GetAgent: %v", err)
		}
		if agent.Name != "get-bot" {
			t.Errorf("Name = %s, want get-bot", agent.Name)
		}
	})

	t.Run("non-existing agent", func(t *testing.T) {
		_, err := svc.GetAgent(ctx, "ghost")
		if err == nil {
			t.Error("expected error for non-existing agent")
		}
	})
}

func TestAgentService_UpdateAgent(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.Register(ctx, "update-bot", "Update Bot", "ai", json.RawMessage(`{"skills":["v1"]}`), 1)

	updated, err := svc.UpdateAgent(ctx, "update-bot", "Updated Bot", json.RawMessage(`{"skills":["v1","v2"]}`))
	if err != nil {
		t.Fatalf("UpdateAgent: %v", err)
	}
	if updated.DisplayName != "Updated Bot" {
		t.Errorf("DisplayName = %s, want Updated Bot", updated.DisplayName)
	}
}

func TestAgentService_Deregister(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.Register(ctx, "dereg-bot", "Dereg Bot", "ai", nil, 1)

	t.Run("owner can deregister", func(t *testing.T) {
		err := svc.Deregister(ctx, "dereg-bot", 1)
		if err != nil {
			t.Fatalf("Deregister: %v", err)
		}

		_, err = svc.GetAgent(ctx, "dereg-bot")
		if err == nil {
			t.Error("expected error for deregistered agent")
		}
	})

	t.Run("wrong owner cannot deregister", func(t *testing.T) {
		svc.Register(ctx, "other-bot", "Other Bot", "ai", nil, 1)
		err := svc.Deregister(ctx, "other-bot", 999)
		if err == nil {
			t.Error("expected error for wrong owner")
		}
	})
}

func TestAgentService_DiscoverAgents(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.Register(ctx, "search-bot", "Search Bot", "ai", json.RawMessage(`{"skills":["web-search"]}`), 1)
	svc.Register(ctx, "analyze-bot", "Analyze Bot", "ai", json.RawMessage(`{"skills":["sentiment"]}`), 1)

	t.Run("find by capability", func(t *testing.T) {
		agents, err := svc.DiscoverAgents(ctx, "web-search")
		if err != nil {
			t.Fatalf("DiscoverAgents: %v", err)
		}
		if len(agents) != 1 {
			t.Errorf("got %d agents, want 1", len(agents))
		}
	})

	t.Run("empty query returns all", func(t *testing.T) {
		agents, err := svc.DiscoverAgents(ctx, "")
		if err != nil {
			t.Fatalf("DiscoverAgents: %v", err)
		}
		if len(agents) != 2 {
			t.Errorf("got %d agents, want 2", len(agents))
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		agents, err := svc.DiscoverAgents(ctx, "quantum")
		if err != nil {
			t.Fatalf("DiscoverAgents: %v", err)
		}
		if len(agents) != 0 {
			t.Errorf("got %d agents, want 0", len(agents))
		}
	})
}

func TestAgentService_ListAgents(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.Register(ctx, "list-a", "Bot A", "ai", nil, 1)
	svc.Register(ctx, "list-b", "Bot B", "ai", nil, 1)

	agents, err := svc.ListAgents(ctx, 1)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 2 {
		t.Errorf("got %d agents, want 2", len(agents))
	}
}

func TestAgentService_DeregisterCapturesDeadLetters(t *testing.T) {
	db := newTestDB(t)
	agentStore := NewSQLiteAgentStore(db)
	tracer := trace.NewTracer(db)
	t.Cleanup(func() { tracer.Close() })

	agentSvc := NewAgentService(agentStore, tracer)

	// Wire up dead letter store
	dls := messaging.NewDeadLetterStore(db)
	agentSvc.SetDeadLetterStore(dls)

	ctx := context.Background()

	// Register sender and target agents
	agentSvc.Register(ctx, "dl-sender", "DL Sender", "ai", nil, 1)
	agentSvc.Register(ctx, "dl-target", "DL Target", "ai", nil, 1)

	// Insert pending messages to dl-target
	db.Exec(`INSERT INTO conversations (id, subject, created_by, created_at, updated_at) VALUES (100, 'test', 'dl-sender', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	db.Exec(`INSERT INTO messages (conversation_id, from_agent, to_agent, body, priority, status, metadata, created_at, updated_at) VALUES (100, 'dl-sender', 'dl-target', 'Pending message 1', 5, 'pending', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	db.Exec(`INSERT INTO messages (conversation_id, from_agent, to_agent, body, priority, status, metadata, created_at, updated_at) VALUES (100, 'dl-sender', 'dl-target', 'Pending message 2', 8, 'pending', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	// Insert a done message (should NOT be captured)
	db.Exec(`INSERT INTO messages (conversation_id, from_agent, to_agent, body, priority, status, metadata, created_at, updated_at) VALUES (100, 'dl-sender', 'dl-target', 'Done message', 5, 'done', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)

	// Deregister the target agent
	err := agentSvc.Deregister(ctx, "dl-target", 1)
	if err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	// Verify dead letters were captured
	letters, total, err := dls.ListDeadLetters(ctx, 1, false, 50)
	if err != nil {
		t.Fatalf("ListDeadLetters: %v", err)
	}
	if total != 2 {
		t.Errorf("total unacknowledged = %d, want 2", total)
	}
	if len(letters) != 2 {
		t.Fatalf("len(letters) = %d, want 2", len(letters))
	}

	// Verify data correctness
	for _, dl := range letters {
		if dl.ToAgent != "dl-target" {
			t.Errorf("to_agent = %q, want dl-target", dl.ToAgent)
		}
		if dl.FromAgent != "dl-sender" {
			t.Errorf("from_agent = %q, want dl-sender", dl.FromAgent)
		}
		if dl.OwnerID != 1 {
			t.Errorf("owner_id = %d, want 1", dl.OwnerID)
		}
	}
}

func TestAgentService_RegisterAndJoinMyAgents(t *testing.T) {
	// Integration test: register an agent, then join it to the my-agents channel.
	// This simulates what the API handler does after registration.
	db := newTestDB(t)
	agentStore := NewSQLiteAgentStore(db)
	tracer := trace.NewTracer(db)
	t.Cleanup(func() { tracer.Close() })

	agentSvc := NewAgentService(agentStore, tracer)

	// Create channel service
	channelStore := channels.NewSQLiteChannelStore(db)
	msgStore := messaging.NewSQLiteMessageStore(db)
	msgSvc := messaging.NewMessagingService(msgStore, tracer)
	channelSvc := channels.NewService(channelStore, msgSvc, tracer)

	ctx := context.Background()

	// Step 1: Ensure human agent (simulates login)
	humanAgent, err := agentSvc.EnsureHumanAgent(ctx, "testowner", "Test Owner", 1)
	if err != nil {
		t.Fatalf("EnsureHumanAgent: %v", err)
	}
	if humanAgent == nil {
		t.Fatal("expected human agent, got nil")
	}

	// Step 2: Ensure my-agents channel (simulates login)
	err = channelSvc.EnsureMyAgentsChannel(ctx, "testowner", humanAgent.Name)
	if err != nil {
		t.Fatalf("EnsureMyAgentsChannel: %v", err)
	}

	// Step 3: Register an AI agent
	newAgent, _, err := agentSvc.Register(ctx, "my-bot", "My Bot", "ai", nil, 1)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Step 4: Join agent to my-agents channel (simulates API handler post-registration)
	err = channelSvc.JoinMyAgentsChannel(ctx, "testowner", newAgent.Name)
	if err != nil {
		t.Fatalf("JoinMyAgentsChannel: %v", err)
	}

	// Verify the agent is a member of the my-agents channel
	ch, err := channelSvc.GetChannelByName(ctx, "my-agents-testowner")
	if err != nil {
		t.Fatalf("GetChannelByName: %v", err)
	}

	members, err := channelSvc.GetMembers(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetMembers: %v", err)
	}

	// Should have 2 members: human agent (owner) + my-bot (member)
	if len(members) != 2 {
		t.Errorf("got %d members, want 2", len(members))
	}

	// Verify my-bot is a member
	found := false
	for _, m := range members {
		if m.AgentName == "my-bot" {
			found = true
			if m.Role != channels.RoleMember {
				t.Errorf("my-bot role = %s, want member", m.Role)
			}
		}
	}
	if !found {
		t.Error("my-bot should be a member of my-agents channel")
	}
}
