// Package runs persists harness execution records to SQLite. It
// implements harness.Observer so it can be wired into the Registry
// without the harness core depending on storage.
package runs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/synapbus/synapbus/internal/agents"
	"github.com/synapbus/synapbus/internal/harness"
	"github.com/synapbus/synapbus/internal/observability"
)

// Run is one row of the harness_runs table. JSON tags match the
// snake_case convention used everywhere else in the Web UI TypeScript
// layer; fields map 1:1 to column names.
type Run struct {
	ID            int64      `json:"id"`
	RunID         string     `json:"run_id"`
	AgentName     string     `json:"agent_name"`
	Backend       string     `json:"backend"`
	MessageID     *int64     `json:"message_id,omitempty"`
	ReactiveRunID *int64     `json:"reactive_run_id,omitempty"`
	Status        string     `json:"status"`
	ExitCode      *int       `json:"exit_code,omitempty"`
	TraceID       string     `json:"trace_id,omitempty"`
	SpanID        string     `json:"span_id,omitempty"`
	SessionID     string     `json:"session_id,omitempty"`
	TokensIn      int64      `json:"tokens_in"`
	TokensOut     int64      `json:"tokens_out"`
	TokensCached  int64      `json:"tokens_cached"`
	CostUSD       float64    `json:"cost_usd"`
	DurationMs    *int64     `json:"duration_ms,omitempty"`
	ResultJSON    string     `json:"result_json,omitempty"`
	LogsExcerpt   string     `json:"logs_excerpt,omitempty"`
	Prompt        string     `json:"prompt,omitempty"`
	Response      string     `json:"response,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

// Status constants match the harness_runs.status column domain.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusTimeout   = "timeout"
)

// Store is the SQLite-backed harness_runs store. It also tracks start
// timestamps in memory so OnFinish can compute duration without needing
// the caller to pass it.
type Store struct {
	db     *sql.DB
	logger *slog.Logger

	mu    sync.Mutex
	start map[string]time.Time // runID → start time
}

// New constructs a Store. Safe for concurrent use.
func New(db *sql.DB, logger *slog.Logger) *Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &Store{
		db:     db,
		logger: logger.With("component", "harness-runs"),
		start:  map[string]time.Time{},
	}
}

// Compile-time check: Store satisfies harness.Observer.
var _ harness.Observer = (*Store)(nil)

// OnStart writes a 'running' row for the run. Errors are logged, not
// returned, so storage issues never block Execute.
func (s *Store) OnStart(ctx context.Context, agent *agents.Agent, harnessName string, req *harness.ExecRequest) {
	s.mu.Lock()
	s.start[req.RunID] = time.Now().UTC()
	s.mu.Unlock()

	var msgID *int64
	if req.Message != nil {
		id := req.Message.ID
		msgID = &id
	}
	var reactiveRunID *int64
	if req.ReactiveRunID > 0 {
		id := req.ReactiveRunID
		reactiveRunID = &id
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO harness_runs (run_id, agent_name, backend, message_id, reactive_run_id, status, trace_id, session_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		req.RunID,
		agentNameOf(agent),
		harnessName,
		msgID,
		reactiveRunID,
		StatusRunning,
		observability.TraceIDFromContext(ctx),
		req.SessionID,
	)
	if err != nil {
		s.logger.Warn("harness_runs insert failed",
			"run_id", req.RunID,
			"agent", agentNameOf(agent),
			"error", err,
		)
	}
}

// OnFinish updates the row with the terminal status, usage, and logs.
func (s *Store) OnFinish(ctx context.Context, agent *agents.Agent, harnessName string, req *harness.ExecRequest, res *harness.ExecResult, execErr error) {
	s.mu.Lock()
	startedAt, ok := s.start[req.RunID]
	delete(s.start, req.RunID)
	s.mu.Unlock()

	var durationMs *int64
	if ok {
		d := time.Since(startedAt).Milliseconds()
		durationMs = &d
	}

	status := StatusSuccess
	var exitCode *int
	logsExcerpt := ""
	var resultJSON string
	var promptText, responseText string
	var tokensIn, tokensOut, tokensCached int64
	var costUSD float64
	sessionID := req.SessionID

	if res != nil {
		ec := res.ExitCode
		exitCode = &ec
		logsExcerpt = res.Logs
		if len(res.ResultJSON) > 0 {
			resultJSON = string(res.ResultJSON)
		}
		promptText = res.Prompt
		responseText = res.Response
		tokensIn = res.Usage.TokensIn
		tokensOut = res.Usage.TokensOut
		tokensCached = res.Usage.TokensCached
		costUSD = res.Usage.CostUSD
		if res.SessionID != "" {
			sessionID = res.SessionID
		}
		if res.ExitCode != 0 {
			status = StatusFailed
		}
	}
	if execErr != nil {
		status = StatusFailed
	}
	// Cap each large text field so harness_runs rows stay reasonable.
	const (
		logsCap     = 16 * 1024
		promptCap   = 32 * 1024
		responseCap = 32 * 1024
	)
	if len(logsExcerpt) > logsCap {
		logsExcerpt = "... [truncated] ...\n" + logsExcerpt[len(logsExcerpt)-logsCap:]
	}
	if len(promptText) > promptCap {
		promptText = "... [truncated " + fmt.Sprintf("%d", len(promptText)-promptCap) + " bytes] ...\n" + promptText[len(promptText)-promptCap:]
	}
	if len(responseText) > responseCap {
		responseText = "... [truncated " + fmt.Sprintf("%d", len(responseText)-responseCap) + " bytes] ...\n" + responseText[len(responseText)-responseCap:]
	}

	traceID := ""
	if res != nil && res.TraceID != "" {
		traceID = res.TraceID
	} else {
		traceID = observability.TraceIDFromContext(ctx)
	}

	// If the run was never inserted (OnStart failed or skipped), fall
	// back to an UPSERT via INSERT OR REPLACE on run_id to avoid losing
	// the terminal row.
	query := `UPDATE harness_runs SET
		status = ?, exit_code = ?, trace_id = ?, session_id = ?,
		tokens_in = ?, tokens_out = ?, tokens_cached = ?, cost_usd = ?,
		duration_ms = ?, result_json = ?, logs_excerpt = ?,
		prompt = ?, response = ?,
		finished_at = CURRENT_TIMESTAMP
		WHERE run_id = ?`

	result, err := s.db.ExecContext(ctx, query,
		status, exitCode, traceID, sessionID,
		tokensIn, tokensOut, tokensCached, costUSD,
		durationMs, nullableString(resultJSON), nullableString(logsExcerpt),
		nullableString(promptText), nullableString(responseText),
		req.RunID,
	)
	if err == nil {
		if n, _ := result.RowsAffected(); n == 0 {
			// Row didn't exist — insert it fresh.
			var reactiveRunID *int64
			if req.ReactiveRunID > 0 {
				id := req.ReactiveRunID
				reactiveRunID = &id
			}
			_, err = s.db.ExecContext(ctx,
				`INSERT INTO harness_runs (run_id, agent_name, backend, reactive_run_id, status, exit_code, trace_id, session_id,
					tokens_in, tokens_out, tokens_cached, cost_usd, duration_ms, result_json, logs_excerpt, prompt, response, created_at, finished_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
				req.RunID, agentNameOf(agent), harnessName, reactiveRunID,
				status, exitCode, traceID, sessionID,
				tokensIn, tokensOut, tokensCached, costUSD,
				durationMs, nullableString(resultJSON), nullableString(logsExcerpt),
				nullableString(promptText), nullableString(responseText),
			)
		}
	}
	if err != nil {
		s.logger.Warn("harness_runs update failed",
			"run_id", req.RunID, "error", err,
		)
	}
}

// GetByRunID retrieves a single harness run by its caller-assigned id.
func (s *Store) GetByRunID(ctx context.Context, runID string) (*Run, error) {
	row := s.db.QueryRowContext(ctx, selectSQL()+` WHERE run_id = ?`, runID)
	return scanRun(row)
}

// GetByReactiveRunID returns the most recent harness_run linked to a
// reactive_run. Used by the Web UI to JOIN the two tables without
// leaking SQL into the API layer. Returns nil, nil when no row exists.
func (s *Store) GetByReactiveRunID(ctx context.Context, reactiveRunID int64) (*Run, error) {
	row := s.db.QueryRowContext(ctx,
		selectSQL()+` WHERE reactive_run_id = ? ORDER BY id DESC LIMIT 1`,
		reactiveRunID,
	)
	run, err := scanRun(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return run, err
}

// ListByAgent returns recent runs for an agent, newest first.
func (s *Store) ListByAgent(ctx context.Context, agentName string, limit int) ([]*Run, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		selectSQL()+` WHERE agent_name = ? ORDER BY created_at DESC LIMIT ?`,
		agentName, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("harness_runs list: %w", err)
	}
	defer rows.Close()
	return scanRuns(rows)
}

// -- helpers --------------------------------------------------------------

func agentNameOf(a *agents.Agent) string {
	if a == nil {
		return ""
	}
	return a.Name
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func selectSQL() string {
	return `SELECT id, run_id, agent_name, backend, message_id, reactive_run_id, status, exit_code,
		trace_id, span_id, session_id, tokens_in, tokens_out, tokens_cached, cost_usd,
		duration_ms, result_json, logs_excerpt, prompt, response, created_at, finished_at
		FROM harness_runs`
}

func scanRun(row *sql.Row) (*Run, error) {
	var r Run
	var msgID, reactiveRunID sql.NullInt64
	var exitCode sql.NullInt64
	var traceID, spanID, sessionID, resultJSON, logsExcerpt, prompt, response sql.NullString
	var durationMs sql.NullInt64
	var createdAt, finishedAt sql.NullTime
	if err := row.Scan(
		&r.ID, &r.RunID, &r.AgentName, &r.Backend, &msgID, &reactiveRunID, &r.Status, &exitCode,
		&traceID, &spanID, &sessionID, &r.TokensIn, &r.TokensOut, &r.TokensCached, &r.CostUSD,
		&durationMs, &resultJSON, &logsExcerpt, &prompt, &response, &createdAt, &finishedAt,
	); err != nil {
		return nil, err
	}
	if msgID.Valid {
		v := msgID.Int64
		r.MessageID = &v
	}
	if reactiveRunID.Valid {
		v := reactiveRunID.Int64
		r.ReactiveRunID = &v
	}
	if exitCode.Valid {
		v := int(exitCode.Int64)
		r.ExitCode = &v
	}
	r.TraceID = traceID.String
	r.SpanID = spanID.String
	r.SessionID = sessionID.String
	r.ResultJSON = resultJSON.String
	r.LogsExcerpt = logsExcerpt.String
	r.Prompt = prompt.String
	r.Response = response.String
	if durationMs.Valid {
		v := durationMs.Int64
		r.DurationMs = &v
	}
	if createdAt.Valid {
		r.CreatedAt = createdAt.Time
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		r.FinishedAt = &t
	}
	return &r, nil
}

func scanRuns(rows *sql.Rows) ([]*Run, error) {
	var out []*Run
	for rows.Next() {
		var r Run
		var msgID, reactiveRunID sql.NullInt64
		var exitCode sql.NullInt64
		var traceID, spanID, sessionID, resultJSON, logsExcerpt, prompt, response sql.NullString
		var durationMs sql.NullInt64
		var createdAt, finishedAt sql.NullTime
		if err := rows.Scan(
			&r.ID, &r.RunID, &r.AgentName, &r.Backend, &msgID, &reactiveRunID, &r.Status, &exitCode,
			&traceID, &spanID, &sessionID, &r.TokensIn, &r.TokensOut, &r.TokensCached, &r.CostUSD,
			&durationMs, &resultJSON, &logsExcerpt, &prompt, &response, &createdAt, &finishedAt,
		); err != nil {
			return nil, err
		}
		if msgID.Valid {
			v := msgID.Int64
			r.MessageID = &v
		}
		if reactiveRunID.Valid {
			v := reactiveRunID.Int64
			r.ReactiveRunID = &v
		}
		if exitCode.Valid {
			v := int(exitCode.Int64)
			r.ExitCode = &v
		}
		r.TraceID = traceID.String
		r.SpanID = spanID.String
		r.SessionID = sessionID.String
		r.ResultJSON = resultJSON.String
		r.LogsExcerpt = logsExcerpt.String
		r.Prompt = prompt.String
		r.Response = response.String
		if durationMs.Valid {
			v := durationMs.Int64
			r.DurationMs = &v
		}
		if createdAt.Valid {
			r.CreatedAt = createdAt.Time
		}
		if finishedAt.Valid {
			t := finishedAt.Time
			r.FinishedAt = &t
		}
		out = append(out, &r)
	}
	if out == nil {
		out = []*Run{}
	}
	return out, rows.Err()
}
