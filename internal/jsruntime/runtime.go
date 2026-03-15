// Package jsruntime provides a lightweight code execution environment for the
// execute MCP tool. It parses simple call() expressions and dispatches them to
// a ToolCaller bridge, which maps action names to service methods.
//
// This is NOT a full JavaScript runtime. It supports:
//   - call(actionName, argsObject) — calls an action via the bridge
//   - Multiple sequential call() invocations
//   - JSON-like argument objects
//
// For the zero-CGO constraint, we avoid embedding a full JS engine and instead
// provide a purpose-built mini-interpreter that covers the execute tool's needs.
package jsruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ToolCaller is the interface that bridges action names to service method calls.
type ToolCaller interface {
	Call(ctx context.Context, actionName string, args map[string]any) (any, error)
}

// ExecuteResult holds the result of code execution.
type ExecuteResult struct {
	Value    any           `json:"value"`
	Calls    int           `json:"calls"`
	Duration time.Duration `json:"duration"`
}

// Pool manages a set of execution contexts. In the current implementation,
// each Execute call runs synchronously with timeout enforcement.
type Pool struct {
	maxConcurrent int
	sem           chan struct{}
}

// NewPool creates a new execution pool with the given concurrency limit.
func NewPool(maxConcurrent int) *Pool {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	return &Pool{
		maxConcurrent: maxConcurrent,
		sem:           make(chan struct{}, maxConcurrent),
	}
}

// Close releases pool resources.
func (p *Pool) Close() {
	// Nothing to clean up in the current implementation.
}

// MaxCalls is the maximum number of call() invocations per execute request.
const MaxCalls = 50

// Execute runs code with the provided ToolCaller bridge and timeout.
// The code is parsed for call(action, args) expressions and each is dispatched.
func (p *Pool) Execute(ctx context.Context, code string, caller ToolCaller, timeout time.Duration) (*ExecuteResult, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	// Acquire semaphore slot.
	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	calls, err := parseCalls(code)
	if err != nil {
		return nil, fmt.Errorf("syntax error: %w", err)
	}

	if len(calls) == 0 {
		return nil, fmt.Errorf("no call() expressions found in code")
	}

	if len(calls) > MaxCalls {
		return nil, fmt.Errorf("too many call() expressions: %d (max %d)", len(calls), MaxCalls)
	}

	var lastResult any
	callCount := 0

	for _, c := range calls {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("execution timeout after %d calls", callCount)
		default:
		}

		result, err := caller.Call(ctx, c.Action, c.Args)
		if err != nil {
			return nil, fmt.Errorf("call %q failed: %w", c.Action, err)
		}
		lastResult = result
		callCount++
	}

	return &ExecuteResult{
		Value:    lastResult,
		Calls:    callCount,
		Duration: time.Since(start),
	}, nil
}

// callExpr represents a parsed call(action, args) expression.
type callExpr struct {
	Action string
	Args   map[string]any
}

// callPattern matches call("action_name", { ... }) or call('action_name', { ... })
var callPattern = regexp.MustCompile(`call\s*\(\s*["']([^"']+)["']\s*(?:,\s*(` + jsonObjectPattern + `))?\s*\)`)

// jsonObjectPattern is a rough pattern for JSON-like objects. It's not perfect
// but works for typical usage. Deep nesting may need the fallback parser.
const jsonObjectPattern = `\{[^}]*\}`

// parseCalls extracts all call() expressions from the code string.
func parseCalls(code string) ([]callExpr, error) {
	// Strip single-line comments.
	lines := strings.Split(code, "\n")
	var cleaned []string
	for _, line := range lines {
		// Remove // comments (but not inside strings -- good enough for typical usage).
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		cleaned = append(cleaned, line)
	}
	code = strings.Join(cleaned, "\n")

	// Try regex-based extraction first.
	matches := callPattern.FindAllStringSubmatch(code, -1)
	if len(matches) == 0 {
		// Try a more lenient parse for nested objects.
		return parseCallsLenient(code)
	}

	var calls []callExpr
	for _, m := range matches {
		actionName := m[1]
		argsStr := "{}"
		if len(m) > 2 && m[2] != "" {
			argsStr = m[2]
		}

		args, err := parseArgsJSON(argsStr)
		if err != nil {
			return nil, fmt.Errorf("invalid arguments for call(%q): %w", actionName, err)
		}

		calls = append(calls, callExpr{Action: actionName, Args: args})
	}

	return calls, nil
}

// parseCallsLenient handles cases where the regex fails (nested braces, etc.)
func parseCallsLenient(code string) ([]callExpr, error) {
	var calls []callExpr
	remaining := code

	for {
		// Find next call(
		idx := strings.Index(remaining, "call(")
		if idx == -1 {
			break
		}
		remaining = remaining[idx+5:] // skip "call("

		// Extract action name.
		remaining = strings.TrimSpace(remaining)
		if len(remaining) == 0 {
			return nil, fmt.Errorf("unexpected end after call(")
		}

		quote := remaining[0]
		if quote != '"' && quote != '\'' {
			return nil, fmt.Errorf("expected quoted action name after call(")
		}
		remaining = remaining[1:]
		endQuote := strings.IndexByte(remaining, quote)
		if endQuote == -1 {
			return nil, fmt.Errorf("unterminated action name string")
		}
		actionName := remaining[:endQuote]
		remaining = remaining[endQuote+1:]

		// Skip whitespace and comma.
		remaining = strings.TrimSpace(remaining)

		args := make(map[string]any)
		if len(remaining) > 0 && remaining[0] == ',' {
			remaining = strings.TrimSpace(remaining[1:])

			// Find matching brace for args object.
			if len(remaining) > 0 && remaining[0] == '{' {
				braceEnd := findMatchingBrace(remaining)
				if braceEnd == -1 {
					return nil, fmt.Errorf("unmatched brace in arguments")
				}
				argsStr := remaining[:braceEnd+1]
				var err error
				args, err = parseArgsJSON(argsStr)
				if err != nil {
					return nil, fmt.Errorf("invalid arguments for call(%q): %w", actionName, err)
				}
				remaining = remaining[braceEnd+1:]
			}
		}

		// Skip closing paren.
		remaining = strings.TrimSpace(remaining)
		if len(remaining) > 0 && remaining[0] == ')' {
			remaining = remaining[1:]
		}

		calls = append(calls, callExpr{Action: actionName, Args: args})
	}

	return calls, nil
}

// findMatchingBrace finds the index of the closing brace matching the opening brace at position 0.
func findMatchingBrace(s string) int {
	if len(s) == 0 || s[0] != '{' {
		return -1
	}
	depth := 0
	inString := false
	var stringChar byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if ch == '\\' {
				i++ // skip escaped char
				continue
			}
			if ch == stringChar {
				inString = false
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inString = true
			stringChar = ch
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseArgsJSON converts a JavaScript-like object literal to a Go map.
// It handles unquoted keys by converting to valid JSON first.
func parseArgsJSON(s string) (map[string]any, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return make(map[string]any), nil
	}

	// Try standard JSON first.
	var result map[string]any
	if err := json.Unmarshal([]byte(s), &result); err == nil {
		return result, nil
	}

	// Convert JS-style object to JSON (unquoted keys, trailing commas, single quotes).
	jsonStr := jsObjectToJSON(s)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("cannot parse args: %s", err)
	}
	return result, nil
}

// jsObjectToJSON converts a JavaScript-style object literal to valid JSON.
func jsObjectToJSON(s string) string {
	// Replace single quotes with double quotes (simple approach).
	s = strings.ReplaceAll(s, "'", "\"")

	// Add quotes around unquoted keys.
	// Match: word characters (possibly with underscores) followed by colon.
	keyPattern := regexp.MustCompile(`(?m)([\{\,]\s*)([a-zA-Z_][a-zA-Z0-9_]*)\s*:`)
	s = keyPattern.ReplaceAllString(s, `$1"$2":`)

	// Remove trailing commas before closing braces/brackets.
	trailingComma := regexp.MustCompile(`,\s*([}\]])`)
	s = trailingComma.ReplaceAllString(s, `$1`)

	// Handle true/false/null (already valid JSON, but just in case).
	return s
}
