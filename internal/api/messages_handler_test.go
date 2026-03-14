package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/synapbus/synapbus/internal/agents"
	"github.com/synapbus/synapbus/internal/auth"
	"github.com/synapbus/synapbus/internal/messaging"
	"github.com/synapbus/synapbus/internal/storage"
	"github.com/synapbus/synapbus/internal/trace"
)

func newMessagesTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	ctx := context.Background()
	if err := storage.RunMigrations(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return db
}

func seedUser(t *testing.T, db *sql.DB, id int64, username string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT OR IGNORE INTO users (id, username, password_hash, display_name) VALUES (?, ?, 'hash', ?)`,
		id, username, username,
	)
	if err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
}

func seedTestAgentWithType(t *testing.T, db *sql.DB, name, agentType string, ownerID int64) {
	t.Helper()
	_, err := db.Exec(
		`INSERT OR IGNORE INTO agents (name, display_name, type, capabilities, owner_id, api_key_hash, status)
		 VALUES (?, ?, ?, '{}', ?, 'testhash', 'active')`,
		name, name, agentType, ownerID,
	)
	if err != nil {
		t.Fatalf("seed agent %s: %v", name, err)
	}
}

func TestSendMessage_SessionAuth_OverridesFrom(t *testing.T) {
	db := newMessagesTestDB(t)

	// Create users and agents
	seedUser(t, db, 1, "alice")
	seedUser(t, db, 2, "bob")
	seedTestAgentWithType(t, db, "alice", "human", 1)    // Human agent for alice
	seedTestAgentWithType(t, db, "alice-bot", "ai", 1)   // AI agent for alice
	seedTestAgentWithType(t, db, "bob", "human", 2)      // Need bob for messaging target

	agentStore := agents.NewSQLiteAgentStore(db)
	tracer := trace.NewTracer(db)
	t.Cleanup(func() { tracer.Close() })
	agentService := agents.NewAgentService(agentStore, tracer)

	msgStore := messaging.NewSQLiteMessageStore(db)
	msgService := messaging.NewMessagingService(msgStore, tracer)

	handler := NewMessagesHandler(msgService, agentService)

	t.Run("session auth overrides from with human agent", func(t *testing.T) {
		// Send a message with from="alice-bot" but session auth should override to "alice"
		body := `{"from":"alice-bot","to":"bob","body":"Hello from UI"}`
		req := httptest.NewRequest("POST", "/api/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		// Set up session auth context: set ownerID and session ID
		ctx := ContextWithOwnerID(req.Context(), 1)
		ctx = auth.ContextWithSessionID(ctx, "test-session-id")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.SendMessage(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d, body: %s", rr.Code, http.StatusCreated, rr.Body.String())
		}

		var msg map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &msg); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		// Verify the from_agent was overridden to the human agent
		if msg["from_agent"] != "alice" {
			t.Errorf("from_agent = %v, want 'alice' (human agent)", msg["from_agent"])
		}
	})

	t.Run("session auth without from field uses human agent", func(t *testing.T) {
		body := `{"to":"bob","body":"Hello without from"}`
		req := httptest.NewRequest("POST", "/api/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		ctx := ContextWithOwnerID(req.Context(), 1)
		ctx = auth.ContextWithSessionID(ctx, "test-session-id")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.SendMessage(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d, body: %s", rr.Code, http.StatusCreated, rr.Body.String())
		}

		var msg map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &msg); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if msg["from_agent"] != "alice" {
			t.Errorf("from_agent = %v, want 'alice' (human agent)", msg["from_agent"])
		}
	})

	t.Run("non-session auth respects from field", func(t *testing.T) {
		// Without session ID in context, the from field should be used as-is
		body := `{"from":"alice-bot","to":"bob","body":"Hello from API"}`
		req := httptest.NewRequest("POST", "/api/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		// Only set ownerID (no session ID) — simulates API key auth
		ctx := ContextWithOwnerID(req.Context(), 1)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.SendMessage(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d, body: %s", rr.Code, http.StatusCreated, rr.Body.String())
		}

		var msg map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &msg); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		// Should use the provided from field since it's not a session auth
		if msg["from_agent"] != "alice-bot" {
			t.Errorf("from_agent = %v, want 'alice-bot'", msg["from_agent"])
		}
	})
}
