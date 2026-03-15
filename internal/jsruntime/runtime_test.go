package jsruntime

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockCaller records calls for testing.
type mockCaller struct {
	calls []struct {
		Action string
		Args   map[string]any
	}
	result any
	err    error
}

func (m *mockCaller) Call(ctx context.Context, actionName string, args map[string]any) (any, error) {
	m.calls = append(m.calls, struct {
		Action string
		Args   map[string]any
	}{Action: actionName, Args: args})
	return m.result, m.err
}

func TestPool_Execute_SingleCall(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	caller := &mockCaller{result: map[string]any{"ok": true}}

	result, err := pool.Execute(context.Background(), `call("read_inbox", { limit: 10 })`, caller, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.Calls != 1 {
		t.Errorf("calls = %d, want 1", result.Calls)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
	if caller.calls[0].Action != "read_inbox" {
		t.Errorf("action = %q, want read_inbox", caller.calls[0].Action)
	}
	limit, ok := caller.calls[0].Args["limit"]
	if !ok {
		t.Error("expected limit in args")
	}
	if limit.(float64) != 10 {
		t.Errorf("limit = %v, want 10", limit)
	}
}

func TestPool_Execute_MultipleCalls(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	caller := &mockCaller{result: map[string]any{"ok": true}}

	code := `
		call("read_inbox", { limit: 5 })
		call("send_message", { to: "bob", body: "hello" })
	`

	result, err := pool.Execute(context.Background(), code, caller, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.Calls != 2 {
		t.Errorf("calls = %d, want 2", result.Calls)
	}

	if len(caller.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(caller.calls))
	}
	if caller.calls[0].Action != "read_inbox" {
		t.Errorf("first action = %q, want read_inbox", caller.calls[0].Action)
	}
	if caller.calls[1].Action != "send_message" {
		t.Errorf("second action = %q, want send_message", caller.calls[1].Action)
	}
}

func TestPool_Execute_SingleQuotes(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	caller := &mockCaller{result: "ok"}

	_, err := pool.Execute(context.Background(), `call('read_inbox', { limit: 5 })`, caller, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if caller.calls[0].Action != "read_inbox" {
		t.Errorf("action = %q, want read_inbox", caller.calls[0].Action)
	}
}

func TestPool_Execute_NoArgs(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	caller := &mockCaller{result: "ok"}

	_, err := pool.Execute(context.Background(), `call("list_channels")`, caller, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
	if len(caller.calls[0].Args) != 0 {
		t.Errorf("expected empty args, got %v", caller.calls[0].Args)
	}
}

func TestPool_Execute_EmptyArgs(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	caller := &mockCaller{result: "ok"}

	_, err := pool.Execute(context.Background(), `call("list_channels", {})`, caller, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
}

func TestPool_Execute_Comments(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	caller := &mockCaller{result: "ok"}

	code := `
		// Read the inbox first
		call("read_inbox", { limit: 5 })
		// Then send a message
		call("send_message", { to: "bob", body: "hi" })
	`

	_, err := pool.Execute(context.Background(), code, caller, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(caller.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(caller.calls))
	}
}

func TestPool_Execute_NoCalls(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	caller := &mockCaller{result: "ok"}

	_, err := pool.Execute(context.Background(), "// just a comment", caller, 5*time.Second)
	if err == nil {
		t.Error("expected error for code with no call() expressions")
	}
}

func TestPool_Execute_CallError(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	caller := &mockCaller{err: fmt.Errorf("action not found")}

	_, err := pool.Execute(context.Background(), `call("unknown", {})`, caller, 5*time.Second)
	if err == nil {
		t.Error("expected error when call fails")
	}
}

func TestPool_Execute_Timeout(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	// Create a caller that blocks
	caller := &mockCaller{}
	slowCaller := &slowToolCaller{delay: 2 * time.Second, result: "ok"}

	_ = caller // unused, using slowCaller

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := pool.Execute(ctx, `call("slow_action", {})`, slowCaller, 100*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

type slowToolCaller struct {
	delay  time.Duration
	result any
}

func (s *slowToolCaller) Call(ctx context.Context, actionName string, args map[string]any) (any, error) {
	select {
	case <-time.After(s.delay):
		return s.result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestPool_Execute_MaxCalls(t *testing.T) {
	pool := NewPool(2)
	defer pool.Close()

	caller := &mockCaller{result: "ok"}

	// Build code with MaxCalls+1 calls
	code := ""
	for i := 0; i <= MaxCalls; i++ {
		code += `call("action", {})` + "\n"
	}

	_, err := pool.Execute(context.Background(), code, caller, 5*time.Second)
	if err == nil {
		t.Error("expected error for exceeding max calls")
	}
}

func TestParseCalls_JSONArgs(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantLen  int
		wantName string
	}{
		{
			name:     "standard JSON args",
			code:     `call("read_inbox", {"limit": 10})`,
			wantLen:  1,
			wantName: "read_inbox",
		},
		{
			name:     "JS-style unquoted keys",
			code:     `call("read_inbox", { limit: 10, from_agent: "alice" })`,
			wantLen:  1,
			wantName: "read_inbox",
		},
		{
			name:     "boolean args",
			code:     `call("read_inbox", { include_read: true })`,
			wantLen:  1,
			wantName: "read_inbox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, err := parseCalls(tt.code)
			if err != nil {
				t.Fatalf("parseCalls: %v", err)
			}
			if len(calls) != tt.wantLen {
				t.Errorf("got %d calls, want %d", len(calls), tt.wantLen)
			}
			if len(calls) > 0 && calls[0].Action != tt.wantName {
				t.Errorf("action = %q, want %q", calls[0].Action, tt.wantName)
			}
		})
	}
}

func TestJsObjectToJSON(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{`{ limit: 10 }`, true},
		{`{ "limit": 10 }`, true},
		{`{ from_agent: "alice", limit: 5 }`, true},
		{`{ include_read: true }`, true},
		{`{ limit: 10, }`, true}, // trailing comma
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseArgsJSON(tt.input)
			if tt.valid && err != nil {
				t.Errorf("parseArgsJSON(%q) failed: %v", tt.input, err)
			}
			if tt.valid && result == nil {
				t.Errorf("parseArgsJSON(%q) returned nil", tt.input)
			}
		})
	}
}
