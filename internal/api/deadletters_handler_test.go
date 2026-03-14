package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	_ "modernc.org/sqlite"

	"github.com/synapbus/synapbus/internal/messaging"
	"github.com/synapbus/synapbus/internal/storage"
)

func newTestDBFull(t *testing.T) *sql.DB {
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

	// Seed test users
	db.Exec(`INSERT OR IGNORE INTO users (id, username, password_hash, display_name) VALUES (1, 'owner1', 'hash', 'Owner 1')`)
	db.Exec(`INSERT OR IGNORE INTO users (id, username, password_hash, display_name) VALUES (2, 'owner2', 'hash', 'Owner 2')`)

	return db
}

func seedTestAgent(t *testing.T, db *sql.DB, name string, ownerID int64) {
	t.Helper()
	_, err := db.Exec(
		`INSERT OR IGNORE INTO agents (name, display_name, type, capabilities, owner_id, api_key_hash, status) VALUES (?, ?, 'ai', '{}', ?, 'testhash', 'active')`,
		name, name, ownerID,
	)
	if err != nil {
		t.Fatalf("seed agent %s: %v", name, err)
	}
}

func seedTestMessage(t *testing.T, db *sql.DB, from, to, body string, priority int) int64 {
	t.Helper()
	// Ensure a conversation exists
	var convID int64
	result, err := db.Exec(
		`INSERT INTO conversations (subject, created_by, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"test", from,
	)
	if err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	convID, _ = result.LastInsertId()

	result, err = db.Exec(
		`INSERT INTO messages (conversation_id, from_agent, to_agent, body, priority, status, metadata, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'pending', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		convID, from, to, body, priority,
	)
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

func seedDeadLetter(t *testing.T, db *sql.DB, ownerID int64, toAgent, fromAgent, body string, priority int) int64 {
	t.Helper()
	result, err := db.Exec(
		`INSERT INTO dead_letters (owner_id, original_message_id, to_agent, from_agent, body, subject, priority, metadata) VALUES (?, 0, ?, ?, ?, '', ?, '{}')`,
		ownerID, toAgent, fromAgent, body, priority,
	)
	if err != nil {
		t.Fatalf("seed dead letter: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

func makeDeadLetterRequest(t *testing.T, handler http.Handler, method, path string, ownerID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if ownerID != "" {
		req.Header.Set("X-Owner-ID", ownerID)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestDeadLetterCaptureOnAgentDeletion(t *testing.T) {
	db := newTestDBFull(t)

	seedTestAgent(t, db, "sender", 1)
	seedTestAgent(t, db, "target-bot", 1)

	// Send pending messages to target-bot
	seedTestMessage(t, db, "sender", "target-bot", "Hello, are you there?", 5)
	seedTestMessage(t, db, "sender", "target-bot", "Urgent task for you", 8)

	// Create dead letter store and capture
	dls := messaging.NewDeadLetterStore(db)

	captured, err := dls.CaptureDeadLetters(context.Background(), 1, "target-bot")
	if err != nil {
		t.Fatalf("CaptureDeadLetters: %v", err)
	}
	if captured != 2 {
		t.Errorf("captured = %d, want 2", captured)
	}

	// Verify dead letters were stored
	letters, total, err := dls.ListDeadLetters(context.Background(), 1, false, 50)
	if err != nil {
		t.Fatalf("ListDeadLetters: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(letters) != 2 {
		t.Fatalf("len(letters) = %d, want 2", len(letters))
	}

	// Verify dead letter data
	foundHello := false
	foundUrgent := false
	for _, dl := range letters {
		if dl.ToAgent != "target-bot" {
			t.Errorf("to_agent = %q, want target-bot", dl.ToAgent)
		}
		if dl.FromAgent != "sender" {
			t.Errorf("from_agent = %q, want sender", dl.FromAgent)
		}
		if dl.OwnerID != 1 {
			t.Errorf("owner_id = %d, want 1", dl.OwnerID)
		}
		if dl.Body == "Hello, are you there?" {
			foundHello = true
		}
		if dl.Body == "Urgent task for you" {
			foundUrgent = true
			if dl.Priority != 8 {
				t.Errorf("priority = %d, want 8", dl.Priority)
			}
		}
	}
	if !foundHello {
		t.Error("expected to find 'Hello, are you there?' dead letter")
	}
	if !foundUrgent {
		t.Error("expected to find 'Urgent task for you' dead letter")
	}
}

func TestDeadLetterListAPI(t *testing.T) {
	db := newTestDBFull(t)
	dls := messaging.NewDeadLetterStore(db)

	// Seed dead letters for owner 1
	seedDeadLetter(t, db, 1, "deleted-bot", "sender", "Message 1", 5)
	seedDeadLetter(t, db, 1, "deleted-bot", "sender", "Message 2", 3)
	// Seed one for owner 2 (should not appear)
	seedDeadLetter(t, db, 2, "other-bot", "other-sender", "Other message", 5)

	deadLettersHandler := NewDeadLettersHandler(dls)

	// Build router with dead letters handler
	router := NewRouter(nil, nil, nil)
	router.Group(func(r chi.Router) {
		r.Use(OwnerAuthMiddleware)
		r.Get("/api/dead-letters", deadLettersHandler.List)
		r.Get("/api/dead-letters/count", deadLettersHandler.Count)
		r.Post("/api/dead-letters/{id}/acknowledge", deadLettersHandler.Acknowledge)
	})

	t.Run("list returns owner dead letters", func(t *testing.T) {
		rr := makeDeadLetterRequest(t, router, "GET", "/api/dead-letters", "1")
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}

		var resp struct {
			DeadLetters []messaging.DeadLetter `json:"dead_letters"`
			Total       int                    `json:"total"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(resp.DeadLetters) != 2 {
			t.Errorf("got %d dead letters, want 2", len(resp.DeadLetters))
		}
		if resp.Total != 2 {
			t.Errorf("total = %d, want 2", resp.Total)
		}
	})

	t.Run("owner isolation", func(t *testing.T) {
		rr := makeDeadLetterRequest(t, router, "GET", "/api/dead-letters", "2")
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		var resp struct {
			DeadLetters []messaging.DeadLetter `json:"dead_letters"`
			Total       int                    `json:"total"`
		}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if len(resp.DeadLetters) != 1 {
			t.Errorf("got %d dead letters, want 1", len(resp.DeadLetters))
		}
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		rr := makeDeadLetterRequest(t, router, "GET", "/api/dead-letters", "")
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})
}

func TestDeadLetterAcknowledgeAPI(t *testing.T) {
	db := newTestDBFull(t)
	dls := messaging.NewDeadLetterStore(db)

	dlID := seedDeadLetter(t, db, 1, "deleted-bot", "sender", "Unread message", 5)

	deadLettersHandler := NewDeadLettersHandler(dls)

	router := NewRouter(nil, nil, nil)
	router.Group(func(r chi.Router) {
		r.Use(OwnerAuthMiddleware)
		r.Get("/api/dead-letters", deadLettersHandler.List)
		r.Get("/api/dead-letters/count", deadLettersHandler.Count)
		r.Post("/api/dead-letters/{id}/acknowledge", deadLettersHandler.Acknowledge)
	})

	t.Run("acknowledge dead letter", func(t *testing.T) {
		rr := makeDeadLetterRequest(t, router, "POST", fmt.Sprintf("/api/dead-letters/%d/acknowledge", dlID), "1")
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}

		var resp map[string]any
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["acknowledged"] != true {
			t.Errorf("acknowledged = %v, want true", resp["acknowledged"])
		}
	})

	t.Run("acknowledged dead letter not in default list", func(t *testing.T) {
		rr := makeDeadLetterRequest(t, router, "GET", "/api/dead-letters", "1")
		var resp struct {
			DeadLetters []messaging.DeadLetter `json:"dead_letters"`
		}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if len(resp.DeadLetters) != 0 {
			t.Errorf("got %d dead letters, want 0 (acknowledged should be hidden)", len(resp.DeadLetters))
		}
	})

	t.Run("acknowledged dead letter visible when including acknowledged", func(t *testing.T) {
		rr := makeDeadLetterRequest(t, router, "GET", "/api/dead-letters?acknowledged=true", "1")
		var resp struct {
			DeadLetters []messaging.DeadLetter `json:"dead_letters"`
		}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if len(resp.DeadLetters) != 1 {
			t.Errorf("got %d dead letters, want 1", len(resp.DeadLetters))
		}
	})

	t.Run("wrong owner cannot acknowledge", func(t *testing.T) {
		// Seed another dead letter for owner 1
		newID := seedDeadLetter(t, db, 1, "deleted-bot", "sender", "Another message", 5)
		rr := makeDeadLetterRequest(t, router, "POST", fmt.Sprintf("/api/dead-letters/%d/acknowledge", newID), "2")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

func TestDeadLetterCountAPI(t *testing.T) {
	db := newTestDBFull(t)
	dls := messaging.NewDeadLetterStore(db)

	seedDeadLetter(t, db, 1, "deleted-bot", "sender", "Message 1", 5)
	seedDeadLetter(t, db, 1, "deleted-bot", "sender", "Message 2", 3)

	deadLettersHandler := NewDeadLettersHandler(dls)

	router := NewRouter(nil, nil, nil)
	router.Group(func(r chi.Router) {
		r.Use(OwnerAuthMiddleware)
		r.Get("/api/dead-letters/count", deadLettersHandler.Count)
		r.Post("/api/dead-letters/{id}/acknowledge", deadLettersHandler.Acknowledge)
	})

	t.Run("count returns unacknowledged count", func(t *testing.T) {
		rr := makeDeadLetterRequest(t, router, "GET", "/api/dead-letters/count", "1")
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		var resp map[string]any
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["count"].(float64) != 2 {
			t.Errorf("count = %v, want 2", resp["count"])
		}
	})

	t.Run("count decreases after acknowledge", func(t *testing.T) {
		// Get the first dead letter's ID
		var dlID int64
		db.QueryRow(`SELECT id FROM dead_letters WHERE owner_id = 1 LIMIT 1`).Scan(&dlID)

		makeDeadLetterRequest(t, router, "POST", fmt.Sprintf("/api/dead-letters/%d/acknowledge", dlID), "1")

		rr := makeDeadLetterRequest(t, router, "GET", "/api/dead-letters/count", "1")
		var resp map[string]any
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp["count"].(float64) != 1 {
			t.Errorf("count = %v, want 1", resp["count"])
		}
	})
}
