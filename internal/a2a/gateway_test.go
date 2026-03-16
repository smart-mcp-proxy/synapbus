package a2a

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/synapbus/synapbus/internal/agents"
	"github.com/synapbus/synapbus/internal/messaging"
	"github.com/synapbus/synapbus/internal/storage"
)

// --- test helpers ---

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

	// Seed a test user for owner_id FK
	db.Exec(`INSERT OR IGNORE INTO users (id, username, password_hash, display_name) VALUES (1, 'testowner', 'hash', 'Test Owner')`)

	return db
}

func seedAgent(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT OR IGNORE INTO agents (name, display_name, type, capabilities, owner_id, api_key_hash, status) VALUES (?, ?, 'ai', '{}', 1, 'testhash', 'active')`,
		name, name,
	)
	if err != nil {
		t.Fatalf("seed agent %s: %v", name, err)
	}
}

// mockMsgService implements MessagingService for testing.
type mockMsgService struct {
	lastFrom    string
	lastTo      string
	lastBody    string
	lastOpts    messaging.SendOptions
	sendErr     error
	returnMsg   *messaging.Message
	convMsgs    []*messaging.Message
	getConvErr  error
}

func (m *mockMsgService) SendMessage(_ context.Context, from, to, body string, opts messaging.SendOptions) (*messaging.Message, error) {
	m.lastFrom = from
	m.lastTo = to
	m.lastBody = body
	m.lastOpts = opts
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	if m.returnMsg != nil {
		return m.returnMsg, nil
	}
	return &messaging.Message{
		ID:             1,
		ConversationID: 100,
		FromAgent:      from,
		ToAgent:        to,
		Body:           body,
	}, nil
}

func (m *mockMsgService) GetConversation(_ context.Context, id int64) (*messaging.Conversation, []*messaging.Message, error) {
	if m.getConvErr != nil {
		return nil, nil, m.getConvErr
	}
	conv := &messaging.Conversation{ID: id}
	return conv, m.convMsgs, nil
}

// mockAgentService implements AgentService for testing.
type mockAgentService struct {
	agents map[string]*agents.Agent
}

func (m *mockAgentService) GetAgent(_ context.Context, name string) (*agents.Agent, error) {
	if a, ok := m.agents[name]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("agent not found: %s", name)
}

func newTestGateway(t *testing.T) (*Gateway, *mockMsgService, *mockAgentService, *A2ATaskStore) {
	t.Helper()
	db := newTestDB(t)
	seedAgent(t, db, "target-bot")
	seedAgent(t, db, "sender-bot")

	taskStore := NewA2ATaskStore(db)
	msgSvc := &mockMsgService{}
	agentSvc := &mockAgentService{
		agents: map[string]*agents.Agent{
			"target-bot": {ID: 1, Name: "target-bot", Status: "active"},
			"sender-bot": {ID: 2, Name: "sender-bot", Status: "active"},
		},
	}

	gw := NewGateway(taskStore, msgSvc, agentSvc)
	return gw, msgSvc, agentSvc, taskStore
}

func jsonRPCCall(method string, params any) []byte {
	p, _ := json.Marshal(params)
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  json.RawMessage(p),
	}
	b, _ := json.Marshal(req)
	return b
}

func doRequest(t *testing.T, gw *Gateway, body []byte, agentCtx *agents.Agent) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if agentCtx != nil {
		req = req.WithContext(agents.ContextWithAgent(req.Context(), agentCtx))
	}
	w := httptest.NewRecorder()
	gw.HandleJSONRPC(w, req)
	return w
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) jsonRPCResponse {
	t.Helper()
	var resp jsonRPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, w.Body.String())
	}
	return resp
}

// --- tests ---

func TestMessageSend_CreatesTaskAndDM(t *testing.T) {
	gw, msgSvc, _, taskStore := newTestGateway(t)

	body := jsonRPCCall("message.send", map[string]any{
		"message": map[string]any{
			"body": "Hello target bot",
			"metadata": map[string]string{
				"target_agent": "target-bot",
			},
		},
	})

	callerAgent := &agents.Agent{Name: "sender-bot"}
	w := doRequest(t, gw, body, callerAgent)
	resp := parseResponse(t, w)

	if resp.Error != nil {
		t.Fatalf("unexpected error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}

	// Verify a task was returned.
	resultBytes, _ := json.Marshal(resp.Result)
	var task A2ATask
	if err := json.Unmarshal(resultBytes, &task); err != nil {
		t.Fatalf("unmarshal task result: %v", err)
	}

	if task.ID == "" {
		t.Error("task ID should not be empty")
	}
	if task.State != StateSubmitted {
		t.Errorf("task state = %q, want %q", task.State, StateSubmitted)
	}
	if task.TargetAgent != "target-bot" {
		t.Errorf("target_agent = %q, want %q", task.TargetAgent, "target-bot")
	}
	if task.SourceAgent != "sender-bot" {
		t.Errorf("source_agent = %q, want %q", task.SourceAgent, "sender-bot")
	}

	// Verify the DM was sent.
	if msgSvc.lastTo != "target-bot" {
		t.Errorf("DM to = %q, want %q", msgSvc.lastTo, "target-bot")
	}
	if msgSvc.lastFrom != "sender-bot" {
		t.Errorf("DM from = %q, want %q", msgSvc.lastFrom, "sender-bot")
	}
	if msgSvc.lastBody != "Hello target bot" {
		t.Errorf("DM body = %q, want %q", msgSvc.lastBody, "Hello target bot")
	}

	// Verify metadata contains a2a_task_id.
	var meta map[string]string
	if err := json.Unmarshal([]byte(msgSvc.lastOpts.Metadata), &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta["a2a_task_id"] != task.ID {
		t.Errorf("metadata a2a_task_id = %q, want %q", meta["a2a_task_id"], task.ID)
	}

	// Verify task is persisted.
	stored, err := taskStore.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if stored.State != StateSubmitted {
		t.Errorf("stored task state = %q, want %q", stored.State, StateSubmitted)
	}
}

func TestTasksGet_ReturnsTask(t *testing.T) {
	gw, _, _, taskStore := newTestGateway(t)

	// Create a task directly.
	convID := int64(100)
	task := &A2ATask{
		ID:             "test-task-123",
		ContextID:      "ctx-123",
		TargetAgent:    "target-bot",
		SourceAgent:    "sender-bot",
		ConversationID: &convID,
		State:          StateSubmitted,
	}
	if err := taskStore.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	body := jsonRPCCall("tasks.get", map[string]string{"id": "test-task-123"})
	w := doRequest(t, gw, body, nil)
	resp := parseResponse(t, w)

	if resp.Error != nil {
		t.Fatalf("unexpected error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var got A2ATask
	if err := json.Unmarshal(resultBytes, &got); err != nil {
		t.Fatalf("unmarshal task result: %v", err)
	}

	if got.ID != "test-task-123" {
		t.Errorf("task ID = %q, want %q", got.ID, "test-task-123")
	}
	if got.State != StateSubmitted {
		t.Errorf("task state = %q, want %q", got.State, StateSubmitted)
	}
}

func TestTasksGet_CompletesOnReply(t *testing.T) {
	gw, msgSvc, _, taskStore := newTestGateway(t)

	// Create a task.
	convID := int64(100)
	task := &A2ATask{
		ID:             "task-reply-test",
		ContextID:      "ctx-456",
		TargetAgent:    "target-bot",
		SourceAgent:    "sender-bot",
		ConversationID: &convID,
		State:          StateSubmitted,
	}
	if err := taskStore.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Simulate the target agent having replied.
	msgSvc.convMsgs = []*messaging.Message{
		{ID: 1, FromAgent: "sender-bot", Body: "Hello"},
		{ID: 2, FromAgent: "target-bot", Body: "Reply from target"},
	}

	body := jsonRPCCall("tasks.get", map[string]string{"id": "task-reply-test"})
	w := doRequest(t, gw, body, nil)
	resp := parseResponse(t, w)

	if resp.Error != nil {
		t.Fatalf("unexpected error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var got A2ATask
	if err := json.Unmarshal(resultBytes, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.State != StateCompleted {
		t.Errorf("task state = %q, want %q (target agent replied)", got.State, StateCompleted)
	}
}

func TestTasksCancel_TransitionsToCanceled(t *testing.T) {
	gw, _, _, taskStore := newTestGateway(t)

	task := &A2ATask{
		ID:          "task-cancel-test",
		ContextID:   "ctx-789",
		TargetAgent: "target-bot",
		SourceAgent: "sender-bot",
		State:       StateSubmitted,
	}
	if err := taskStore.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	body := jsonRPCCall("tasks.cancel", map[string]string{"id": "task-cancel-test"})
	w := doRequest(t, gw, body, nil)
	resp := parseResponse(t, w)

	if resp.Error != nil {
		t.Fatalf("unexpected error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var got A2ATask
	if err := json.Unmarshal(resultBytes, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.State != StateCanceled {
		t.Errorf("task state = %q, want %q", got.State, StateCanceled)
	}

	// Verify persisted state.
	stored, err := taskStore.GetTask(context.Background(), "task-cancel-test")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if stored.State != StateCanceled {
		t.Errorf("stored state = %q, want %q", stored.State, StateCanceled)
	}
}

func TestTasksCancel_TerminalStateError(t *testing.T) {
	gw, _, _, taskStore := newTestGateway(t)

	task := &A2ATask{
		ID:          "task-already-done",
		ContextID:   "ctx-done",
		TargetAgent: "target-bot",
		State:       StateCompleted,
	}
	if err := taskStore.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	body := jsonRPCCall("tasks.cancel", map[string]string{"id": "task-already-done"})
	w := doRequest(t, gw, body, nil)
	resp := parseResponse(t, w)

	if resp.Error == nil {
		t.Fatal("expected error for canceling terminal task")
	}
	if resp.Error.Code != errCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, errCodeInvalidParams)
	}
}

func TestMessageSend_NonExistentAgent(t *testing.T) {
	gw, _, _, _ := newTestGateway(t)

	body := jsonRPCCall("message.send", map[string]any{
		"message": map[string]any{
			"body": "Hello ghost",
			"metadata": map[string]string{
				"target_agent": "does-not-exist",
			},
		},
	})

	w := doRequest(t, gw, body, nil)
	resp := parseResponse(t, w)

	if resp.Error == nil {
		t.Fatal("expected error for non-existent target agent")
	}
	if resp.Error.Code != errCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, errCodeInvalidParams)
	}
}

func TestInvalidJSONRPC(t *testing.T) {
	gw, _, _, _ := newTestGateway(t)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "not JSON",
			body: "this is not json",
		},
		{
			name: "wrong jsonrpc version",
			body: `{"jsonrpc":"1.0","id":1,"method":"message.send","params":{}}`,
		},
		{
			name: "unknown method",
			body: `{"jsonrpc":"2.0","id":1,"method":"unknown.method","params":{}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			gw.HandleJSONRPC(w, req)

			var resp jsonRPCResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal response: %v\nbody: %s", err, w.Body.String())
			}
			if resp.Error == nil {
				t.Error("expected error response")
			}
		})
	}
}

func TestUnauthenticatedRequest_NoAgentContext(t *testing.T) {
	// This tests that message.send works even without an authenticated agent
	// in context (caller is anonymous), using "a2a-gateway" as the sender.
	gw, msgSvc, _, _ := newTestGateway(t)

	body := jsonRPCCall("message.send", map[string]any{
		"message": map[string]any{
			"body": "Hello from anonymous",
			"metadata": map[string]string{
				"target_agent": "target-bot",
			},
		},
	})

	// No agent context — simulates unauthenticated-at-gateway-level
	// (in practice, the auth middleware would block this; this tests the
	// gateway's fallback behavior).
	w := doRequest(t, gw, body, nil)
	resp := parseResponse(t, w)

	if resp.Error != nil {
		t.Fatalf("unexpected error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
	}

	// Should use "a2a-gateway" as sender when no caller agent.
	if msgSvc.lastFrom != "a2a-gateway" {
		t.Errorf("DM from = %q, want %q", msgSvc.lastFrom, "a2a-gateway")
	}
}

func TestHTTPMethodNotAllowed(t *testing.T) {
	gw, _, _, _ := newTestGateway(t)

	req := httptest.NewRequest(http.MethodGet, "/a2a", nil)
	w := httptest.NewRecorder()
	gw.HandleJSONRPC(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}
