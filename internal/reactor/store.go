package reactor

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RunStatus constants for reactive_runs.
const (
	StatusQueued          = "queued"
	StatusRunning         = "running"
	StatusSucceeded       = "succeeded"
	StatusFailed          = "failed"
	StatusCooldownSkipped = "cooldown_skipped"
	StatusBudgetExhausted = "budget_exhausted"
	StatusDepthExceeded   = "depth_exceeded"
)

// ReactiveRun represents a single trigger evaluation and its outcome.
type ReactiveRun struct {
	ID               int64      `json:"id"`
	AgentName        string     `json:"agent_name"`
	TriggerMessageID *int64     `json:"trigger_message_id,omitempty"`
	TriggerEvent     string     `json:"trigger_event"`
	TriggerDepth     int        `json:"trigger_depth"`
	TriggerFrom      string     `json:"trigger_from,omitempty"`
	Status           string     `json:"status"`
	K8sJobName       string     `json:"k8s_job_name,omitempty"`
	K8sNamespace     string     `json:"k8s_namespace,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	DurationMs       *int64     `json:"duration_ms,omitempty"`
	ErrorLog         string     `json:"error_log,omitempty"`
	TokenCostJSON    string     `json:"token_cost_json,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// Store handles SQLite persistence for reactive runs.
type Store struct {
	db *sql.DB
}

// NewStore creates a new reactor store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// InsertRun creates a new reactive_runs record.
func (s *Store) InsertRun(ctx context.Context, run *ReactiveRun) (int64, error) {
	now := time.Now().UTC()
	run.CreatedAt = now
	nowStr := now.Format(time.RFC3339)
	var startedAtStr *string
	if run.StartedAt != nil {
		s := run.StartedAt.UTC().Format(time.RFC3339)
		startedAtStr = &s
	}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO reactive_runs (agent_name, trigger_message_id, trigger_event, trigger_depth, trigger_from, status, k8s_job_name, k8s_namespace, started_at, error_log, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.AgentName, run.TriggerMessageID, run.TriggerEvent, run.TriggerDepth,
		run.TriggerFrom, run.Status, run.K8sJobName, run.K8sNamespace, startedAtStr, run.ErrorLog, nowStr,
	)
	if err != nil {
		return 0, fmt.Errorf("insert reactive run: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	run.ID = id
	return id, nil
}

// UpdateRunStatus updates a run's status and optional fields.
func (s *Store) UpdateRunStatus(ctx context.Context, id int64, status string, jobName, namespace string, startedAt *time.Time) error {
	var startedAtStr *string
	if startedAt != nil {
		str := startedAt.UTC().Format(time.RFC3339)
		startedAtStr = &str
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE reactive_runs SET status = ?, k8s_job_name = ?, k8s_namespace = ?, started_at = ? WHERE id = ?`,
		status, jobName, namespace, startedAtStr, id,
	)
	return err
}

// CompleteRun marks a run as completed (succeeded or failed).
func (s *Store) CompleteRun(ctx context.Context, id int64, status, errorLog string, completedAt time.Time) error {
	completedStr := completedAt.UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE reactive_runs SET status = ?, error_log = ?, completed_at = ?,
		 duration_ms = CAST((julianday(?) - julianday(started_at)) * 86400000 AS INTEGER)
		 WHERE id = ?`,
		status, errorLog, completedStr, completedStr, id,
	)
	return err
}

// GetRunByID returns a single run.
func (s *Store) GetRunByID(ctx context.Context, id int64) (*ReactiveRun, error) {
	return s.scanRun(s.db.QueryRowContext(ctx, runSelectSQL()+` WHERE id = ?`, id))
}

// ListRuns returns recent runs with optional filters.
func (s *Store) ListRuns(ctx context.Context, agentName, status string, limit, offset int) ([]*ReactiveRun, int, error) {
	where := "WHERE 1=1"
	args := []any{}

	if agentName != "" {
		where += " AND agent_name = ?"
		args = append(args, agentName)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	// Count total
	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM reactive_runs "+where, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Query with pagination
	query := runSelectSQL() + " " + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	runs, err := s.scanRuns(rows)
	return runs, total, err
}

// GetActiveRuns returns runs with status 'running' (for polling).
func (s *Store) GetActiveRuns(ctx context.Context) ([]*ReactiveRun, error) {
	rows, err := s.db.QueryContext(ctx, runSelectSQL()+` WHERE status = 'running'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanRuns(rows)
}

// CountTodayRuns counts runs that count against the daily budget for an agent.
func (s *Store) CountTodayRuns(ctx context.Context, agentName string) (int, error) {
	// Compute start of today in UTC as RFC3339
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	startStr := startOfDay.Format(time.RFC3339)

	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM reactive_runs
		 WHERE agent_name = ? AND status IN ('running', 'succeeded', 'failed')
		 AND created_at >= ?`,
		agentName, startStr,
	).Scan(&count)
	return count, err
}

// GetLastRunTime returns the created_at of the most recent countable run.
func (s *Store) GetLastRunTime(ctx context.Context, agentName string) (*time.Time, error) {
	var t sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM reactive_runs
		 WHERE agent_name = ? AND status IN ('running', 'succeeded', 'failed')`,
		agentName,
	).Scan(&t)
	if err != nil {
		return nil, err
	}
	if !t.Valid || t.String == "" {
		return nil, nil
	}
	parsed, err := parseTime(t.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// parseTime tries multiple time formats used by SQLite / Go driver.
func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05+00:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999Z07:00",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}

// IsAgentRunning checks if the agent has an active (running) reactive run.
func (s *Store) IsAgentRunning(ctx context.Context, agentName string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM reactive_runs WHERE agent_name = ? AND status = 'running'`,
		agentName,
	).Scan(&count)
	return count > 0, err
}

func runSelectSQL() string {
	return `SELECT id, agent_name, trigger_message_id, trigger_event, trigger_depth, trigger_from,
		 status, k8s_job_name, k8s_namespace, started_at, completed_at, duration_ms, error_log, token_cost_json, created_at
		 FROM reactive_runs`
}

func scanRunFields(r *ReactiveRun, msgID *sql.NullInt64, triggerFrom, jobName, namespace, errorLog, tokenCost *sql.NullString, startedAt, completedAt *sql.NullString, durationMs *sql.NullInt64, createdAt *string) {
	if msgID.Valid {
		r.TriggerMessageID = &msgID.Int64
	}
	r.TriggerFrom = triggerFrom.String
	r.K8sJobName = jobName.String
	r.K8sNamespace = namespace.String
	if startedAt.Valid && startedAt.String != "" {
		if t, err := parseTime(startedAt.String); err == nil {
			r.StartedAt = &t
		}
	}
	if completedAt.Valid && completedAt.String != "" {
		if t, err := parseTime(completedAt.String); err == nil {
			r.CompletedAt = &t
		}
	}
	if durationMs.Valid {
		r.DurationMs = &durationMs.Int64
	}
	r.ErrorLog = errorLog.String
	r.TokenCostJSON = tokenCost.String
	if *createdAt != "" {
		if t, err := parseTime(*createdAt); err == nil {
			r.CreatedAt = t
		}
	}
}

func (s *Store) scanRun(row *sql.Row) (*ReactiveRun, error) {
	var r ReactiveRun
	var msgID sql.NullInt64
	var triggerFrom, jobName, namespace, errorLog, tokenCost sql.NullString
	var startedAt, completedAt sql.NullString
	var durationMs sql.NullInt64
	var createdAt string

	err := row.Scan(
		&r.ID, &r.AgentName, &msgID, &r.TriggerEvent, &r.TriggerDepth, &triggerFrom,
		&r.Status, &jobName, &namespace, &startedAt, &completedAt, &durationMs, &errorLog, &tokenCost, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	scanRunFields(&r, &msgID, &triggerFrom, &jobName, &namespace, &errorLog, &tokenCost, &startedAt, &completedAt, &durationMs, &createdAt)
	return &r, nil
}

func (s *Store) scanRuns(rows *sql.Rows) ([]*ReactiveRun, error) {
	var runs []*ReactiveRun
	for rows.Next() {
		var r ReactiveRun
		var msgID sql.NullInt64
		var triggerFrom, jobName, namespace, errorLog, tokenCost sql.NullString
		var startedAt, completedAt sql.NullString
		var durationMs sql.NullInt64
		var createdAt string

		err := rows.Scan(
			&r.ID, &r.AgentName, &msgID, &r.TriggerEvent, &r.TriggerDepth, &triggerFrom,
			&r.Status, &jobName, &namespace, &startedAt, &completedAt, &durationMs, &errorLog, &tokenCost, &createdAt,
		)
		if err != nil {
			return nil, err
		}
		scanRunFields(&r, &msgID, &triggerFrom, &jobName, &namespace, &errorLog, &tokenCost, &startedAt, &completedAt, &durationMs, &createdAt)
		runs = append(runs, &r)
	}
	if runs == nil {
		runs = []*ReactiveRun{}
	}
	return runs, rows.Err()
}
