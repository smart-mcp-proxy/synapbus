package console

import (
	"bytes"
	"strings"
	"testing"
)

func TestSuccess(t *testing.T) {
	var buf bytes.Buffer
	p := NewWithWriter(&buf)
	p.Success("SynapBus listening on :8900")
	out := buf.String()
	if !strings.Contains(out, "✓") {
		t.Errorf("expected checkmark, got: %s", out)
	}
	if !strings.Contains(out, "SynapBus listening on :8900") {
		t.Errorf("expected message, got: %s", out)
	}
}

func TestArrow(t *testing.T) {
	var buf bytes.Buffer
	p := NewWithWriter(&buf)
	p.Arrow("something happened")
	out := buf.String()
	if !strings.Contains(out, "→") {
		t.Errorf("expected arrow, got: %s", out)
	}
}

func TestAgentConnectedWithClient(t *testing.T) {
	var buf bytes.Buffer
	p := NewWithWriter(&buf)
	p.AgentConnected("planner", "claude-code", "1.2.3")
	out := buf.String()
	if !strings.Contains(out, `"planner"`) {
		t.Errorf("expected agent name, got: %s", out)
	}
	if !strings.Contains(out, "claude-code") {
		t.Errorf("expected client name, got: %s", out)
	}
	if !strings.Contains(out, "1.2.3") {
		t.Errorf("expected client version, got: %s", out)
	}
}

func TestAgentConnectedWithoutClient(t *testing.T) {
	var buf bytes.Buffer
	p := NewWithWriter(&buf)
	p.AgentConnected("coder", "", "")
	out := buf.String()
	if !strings.Contains(out, `"coder"`) {
		t.Errorf("expected agent name, got: %s", out)
	}
	if strings.Contains(out, "(") {
		t.Errorf("should not have parens without client info, got: %s", out)
	}
}

func TestAgentDisconnected(t *testing.T) {
	var buf bytes.Buffer
	p := NewWithWriter(&buf)
	p.AgentDisconnected("reviewer")
	out := buf.String()
	if !strings.Contains(out, "←") {
		t.Errorf("expected left arrow, got: %s", out)
	}
	if !strings.Contains(out, `"reviewer"`) {
		t.Errorf("expected agent name, got: %s", out)
	}
}

func TestClientConnected(t *testing.T) {
	var buf bytes.Buffer
	p := NewWithWriter(&buf)
	p.ClientConnected("mcp-inspector", "0.5.0")
	out := buf.String()
	if !strings.Contains(out, "Client connected") {
		t.Errorf("expected client connected, got: %s", out)
	}
	if !strings.Contains(out, "mcp-inspector") {
		t.Errorf("expected client name, got: %s", out)
	}
}

func TestInfoAndWarn(t *testing.T) {
	var buf bytes.Buffer
	p := NewWithWriter(&buf)
	p.Info("Waiting for agents...")
	p.Warn("something")
	out := buf.String()
	if !strings.Contains(out, "Waiting for agents...") {
		t.Errorf("expected info message, got: %s", out)
	}
	if !strings.Contains(out, "⚠") {
		t.Errorf("expected warning symbol, got: %s", out)
	}
}

func TestBlank(t *testing.T) {
	var buf bytes.Buffer
	p := NewWithWriter(&buf)
	p.Blank()
	if buf.String() != "\n" {
		t.Errorf("expected blank line, got: %q", buf.String())
	}
}
