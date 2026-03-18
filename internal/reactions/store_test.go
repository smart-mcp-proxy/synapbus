package reactions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/synapbus/synapbus/internal/storage"
)

func newTestDB(t *testing.T) *sql.DB {
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

// seedTestMessage creates a test user, agent, conversation, and message,
// returning the message ID.
func seedTestMessage(t *testing.T, db *sql.DB, agentName string) int64 {
	t.Helper()

	// Ensure user exists
	db.Exec(`INSERT OR IGNORE INTO users (id, username, password_hash, display_name) VALUES (1, 'testowner', 'hash', 'Test Owner')`)

	// Ensure agent exists
	_, err := db.Exec(
		`INSERT OR IGNORE INTO agents (name, display_name, type, capabilities, owner_id, api_key_hash, status) VALUES (?, ?, 'ai', '{}', 1, 'testhash', 'active')`,
		agentName, agentName,
	)
	if err != nil {
		t.Fatalf("seed agent %s: %v", agentName, err)
	}

	// Create conversation
	result, err := db.Exec(
		`INSERT INTO conversations (subject, created_by, created_at, updated_at) VALUES ('test', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		agentName,
	)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	convID, _ := result.LastInsertId()

	// Create message
	result, err = db.Exec(
		`INSERT INTO messages (conversation_id, from_agent, body, priority, status, created_at) VALUES (?, ?, 'test body', 5, 'pending', CURRENT_TIMESTAMP)`,
		convID, agentName,
	)
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	msgID, _ := result.LastInsertId()
	return msgID
}

func TestSQLiteStore_InsertAndGet(t *testing.T) {
	db := newTestDB(t)
	store := NewSQLiteStore(db)
	ctx := context.Background()

	msgID := seedTestMessage(t, db, "agent-a")

	r := &Reaction{
		MessageID: msgID,
		AgentName: "agent-a",
		Reaction:  ReactionApprove,
		Metadata:  json.RawMessage(`{"comment":"looks good"}`),
	}

	if err := store.Insert(ctx, r); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if r.ID == 0 {
		t.Error("reaction ID should not be 0 after insert")
	}

	// Verify it exists
	exists, err := store.Exists(ctx, msgID, "agent-a", ReactionApprove)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("expected reaction to exist after insert")
	}

	// GetByMessageID
	reactions, err := store.GetByMessageID(ctx, msgID)
	if err != nil {
		t.Fatalf("GetByMessageID: %v", err)
	}
	if len(reactions) != 1 {
		t.Fatalf("got %d reactions, want 1", len(reactions))
	}
	if reactions[0].AgentName != "agent-a" {
		t.Errorf("AgentName = %q, want %q", reactions[0].AgentName, "agent-a")
	}
	if reactions[0].Reaction != ReactionApprove {
		t.Errorf("Reaction = %q, want %q", reactions[0].Reaction, ReactionApprove)
	}
	if string(reactions[0].Metadata) != `{"comment":"looks good"}` {
		t.Errorf("Metadata = %s, want %s", reactions[0].Metadata, `{"comment":"looks good"}`)
	}
}

func TestSQLiteStore_UniqueConstraint(t *testing.T) {
	db := newTestDB(t)
	store := NewSQLiteStore(db)
	ctx := context.Background()

	msgID := seedTestMessage(t, db, "agent-a")

	r := &Reaction{
		MessageID: msgID,
		AgentName: "agent-a",
		Reaction:  ReactionApprove,
	}

	if err := store.Insert(ctx, r); err != nil {
		t.Fatalf("Insert first: %v", err)
	}

	// Inserting the same reaction again should fail with UNIQUE constraint
	r2 := &Reaction{
		MessageID: msgID,
		AgentName: "agent-a",
		Reaction:  ReactionApprove,
	}
	err := store.Insert(ctx, r2)
	if err == nil {
		t.Error("expected error on duplicate insert, got nil")
	}
}

func TestSQLiteStore_Delete(t *testing.T) {
	db := newTestDB(t)
	store := NewSQLiteStore(db)
	ctx := context.Background()

	msgID := seedTestMessage(t, db, "agent-a")

	r := &Reaction{
		MessageID: msgID,
		AgentName: "agent-a",
		Reaction:  ReactionApprove,
	}
	if err := store.Insert(ctx, r); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Delete the reaction
	if err := store.Delete(ctx, msgID, "agent-a", ReactionApprove); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone
	exists, err := store.Exists(ctx, msgID, "agent-a", ReactionApprove)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("expected reaction to not exist after delete")
	}

	// Deleting a non-existent reaction should return an error
	err = store.Delete(ctx, msgID, "agent-a", ReactionApprove)
	if err == nil {
		t.Error("expected error when deleting non-existent reaction, got nil")
	}
}

func TestSQLiteStore_GetByMessageID(t *testing.T) {
	db := newTestDB(t)
	store := NewSQLiteStore(db)
	ctx := context.Background()

	msgID := seedTestMessage(t, db, "agent-a")
	// Seed a second agent
	db.Exec(`INSERT OR IGNORE INTO agents (name, display_name, type, capabilities, owner_id, api_key_hash, status) VALUES ('agent-b', 'agent-b', 'ai', '{}', 1, 'testhash2', 'active')`)

	// Insert multiple reactions from different agents
	reactions := []*Reaction{
		{MessageID: msgID, AgentName: "agent-a", Reaction: ReactionApprove},
		{MessageID: msgID, AgentName: "agent-b", Reaction: ReactionInProgress},
		{MessageID: msgID, AgentName: "agent-a", Reaction: ReactionDone},
	}

	for _, r := range reactions {
		if err := store.Insert(ctx, r); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	got, err := store.GetByMessageID(ctx, msgID)
	if err != nil {
		t.Fatalf("GetByMessageID: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d reactions, want 3", len(got))
	}

	// Verify results are ordered by created_at ASC
	for i, r := range got {
		if r.ID == 0 {
			t.Errorf("reaction[%d] ID should not be 0", i)
		}
		if r.MessageID != msgID {
			t.Errorf("reaction[%d] MessageID = %d, want %d", i, r.MessageID, msgID)
		}
	}

	// Test with a message that has no reactions
	msgID2 := seedTestMessage(t, db, "agent-a")
	got2, err := store.GetByMessageID(ctx, msgID2)
	if err != nil {
		t.Fatalf("GetByMessageID (empty): %v", err)
	}
	if len(got2) != 0 {
		t.Errorf("got %d reactions for empty message, want 0", len(got2))
	}
}

func TestSQLiteStore_CountByMessage(t *testing.T) {
	db := newTestDB(t)
	store := NewSQLiteStore(db)
	ctx := context.Background()

	msgID := seedTestMessage(t, db, "agent-a")
	db.Exec(`INSERT OR IGNORE INTO agents (name, display_name, type, capabilities, owner_id, api_key_hash, status) VALUES ('agent-b', 'agent-b', 'ai', '{}', 1, 'testhash2', 'active')`)

	// Count should be 0 initially
	count, err := store.CountByMessage(ctx, msgID)
	if err != nil {
		t.Fatalf("CountByMessage: %v", err)
	}
	if count != 0 {
		t.Errorf("initial count = %d, want 0", count)
	}

	// Insert some reactions
	for _, r := range []*Reaction{
		{MessageID: msgID, AgentName: "agent-a", Reaction: ReactionApprove},
		{MessageID: msgID, AgentName: "agent-b", Reaction: ReactionDone},
	} {
		if err := store.Insert(ctx, r); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	count, err = store.CountByMessage(ctx, msgID)
	if err != nil {
		t.Fatalf("CountByMessage: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	// Delete one and verify count decreases
	if err := store.Delete(ctx, msgID, "agent-a", ReactionApprove); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	count, err = store.CountByMessage(ctx, msgID)
	if err != nil {
		t.Fatalf("CountByMessage after delete: %v", err)
	}
	if count != 1 {
		t.Errorf("count after delete = %d, want 1", count)
	}
}

func TestSQLiteStore_GetByMessageIDs(t *testing.T) {
	db := newTestDB(t)
	store := NewSQLiteStore(db)
	ctx := context.Background()

	msgID1 := seedTestMessage(t, db, "agent-a")
	msgID2 := seedTestMessage(t, db, "agent-a")

	// Add reactions to msg1
	if err := store.Insert(ctx, &Reaction{MessageID: msgID1, AgentName: "agent-a", Reaction: ReactionApprove}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := store.Insert(ctx, &Reaction{MessageID: msgID1, AgentName: "agent-a", Reaction: ReactionDone}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Add reaction to msg2
	if err := store.Insert(ctx, &Reaction{MessageID: msgID2, AgentName: "agent-a", Reaction: ReactionReject}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	result, err := store.GetByMessageIDs(ctx, []int64{msgID1, msgID2})
	if err != nil {
		t.Fatalf("GetByMessageIDs: %v", err)
	}

	if len(result[msgID1]) != 2 {
		t.Errorf("msg1 reactions = %d, want 2", len(result[msgID1]))
	}
	if len(result[msgID2]) != 1 {
		t.Errorf("msg2 reactions = %d, want 1", len(result[msgID2]))
	}

	// Empty slice returns empty map
	result, err = store.GetByMessageIDs(ctx, []int64{})
	if err != nil {
		t.Fatalf("GetByMessageIDs (empty): %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}
