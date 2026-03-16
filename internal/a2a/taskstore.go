// Package a2a provides the A2A (Agent-to-Agent) inbound gateway for SynapBus.
// External A2A-compliant agents can send tasks to SynapBus agents via JSON-RPC.
package a2a

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Task states following the A2A protocol.
const (
	StateSubmitted = "SUBMITTED"
	StateCompleted = "COMPLETED"
	StateCanceled  = "CANCELED"
)

// A2ATask represents an inbound A2A task tracked by the gateway.
type A2ATask struct {
	ID             string    `json:"id"`
	ContextID      string    `json:"context_id"`
	TargetAgent    string    `json:"target_agent"`
	SourceAgent    string    `json:"source_agent"`
	ConversationID *int64    `json:"conversation_id,omitempty"`
	State          string    `json:"state"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// A2ATaskStore provides CRUD operations for A2A tasks backed by SQLite.
type A2ATaskStore struct {
	db *sql.DB
}

// NewA2ATaskStore creates a new task store.
func NewA2ATaskStore(db *sql.DB) *A2ATaskStore {
	return &A2ATaskStore{db: db}
}

// CreateTask inserts a new A2A task into the database.
func (s *A2ATaskStore) CreateTask(ctx context.Context, task *A2ATask) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO a2a_tasks (id, context_id, target_agent, source_agent, conversation_id, state, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		task.ID, task.ContextID, task.TargetAgent, task.SourceAgent, task.ConversationID, task.State,
	)
	if err != nil {
		return fmt.Errorf("insert a2a task: %w", err)
	}
	return nil
}

// GetTask returns an A2A task by its ID.
func (s *A2ATaskStore) GetTask(ctx context.Context, id string) (*A2ATask, error) {
	var task A2ATask
	var conversationID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, context_id, target_agent, source_agent, conversation_id, state, created_at, updated_at
		 FROM a2a_tasks WHERE id = ?`, id,
	).Scan(&task.ID, &task.ContextID, &task.TargetAgent, &task.SourceAgent,
		&conversationID, &task.State, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("a2a task not found: %s", id)
		}
		return nil, fmt.Errorf("get a2a task: %w", err)
	}
	if conversationID.Valid {
		task.ConversationID = &conversationID.Int64
	}
	return &task, nil
}

// UpdateTaskState transitions a task to a new state.
func (s *A2ATaskStore) UpdateTaskState(ctx context.Context, id, state string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE a2a_tasks SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		state, id,
	)
	if err != nil {
		return fmt.Errorf("update a2a task state: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("a2a task not found: %s", id)
	}
	return nil
}

// ListTasks returns tasks for a target agent, optionally filtered by state.
func (s *A2ATaskStore) ListTasks(ctx context.Context, targetAgent, state string) ([]*A2ATask, error) {
	var query string
	var args []any

	if state != "" {
		query = `SELECT id, context_id, target_agent, source_agent, conversation_id, state, created_at, updated_at
				 FROM a2a_tasks WHERE target_agent = ? AND state = ? ORDER BY created_at DESC`
		args = []any{targetAgent, state}
	} else {
		query = `SELECT id, context_id, target_agent, source_agent, conversation_id, state, created_at, updated_at
				 FROM a2a_tasks WHERE target_agent = ? ORDER BY created_at DESC`
		args = []any{targetAgent}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list a2a tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*A2ATask
	for rows.Next() {
		var task A2ATask
		var conversationID sql.NullInt64
		if err := rows.Scan(&task.ID, &task.ContextID, &task.TargetAgent, &task.SourceAgent,
			&conversationID, &task.State, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan a2a task: %w", err)
		}
		if conversationID.Valid {
			task.ConversationID = &conversationID.Int64
		}
		tasks = append(tasks, &task)
	}
	if tasks == nil {
		tasks = []*A2ATask{}
	}
	return tasks, rows.Err()
}
