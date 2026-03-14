package messaging

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// DeadLetterStore provides storage operations for the dead letter queue.
type DeadLetterStore struct {
	db *sql.DB
}

// NewDeadLetterStore creates a new dead letter store.
func NewDeadLetterStore(db *sql.DB) *DeadLetterStore {
	return &DeadLetterStore{db: db}
}

// CaptureDeadLetters moves pending/processing messages for an agent to the dead_letters table.
// Returns the number of messages captured.
func (s *DeadLetterStore) CaptureDeadLetters(ctx context.Context, ownerID int64, agentName string) (int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.id, m.from_agent, m.body, COALESCE(c.subject, ''), m.priority, m.metadata
		 FROM messages m
		 LEFT JOIN conversations c ON c.id = m.conversation_id
		 WHERE m.to_agent = ? AND m.status IN ('pending', 'processing')`,
		agentName,
	)
	if err != nil {
		return 0, fmt.Errorf("query pending messages: %w", err)
	}
	defer rows.Close()

	type pendingMsg struct {
		id        int64
		fromAgent string
		body      string
		subject   string
		priority  int
		metadata  string
	}

	var pending []pendingMsg
	for rows.Next() {
		var msg pendingMsg
		if err := rows.Scan(&msg.id, &msg.fromAgent, &msg.body, &msg.subject, &msg.priority, &msg.metadata); err != nil {
			return 0, fmt.Errorf("scan pending message: %w", err)
		}
		pending = append(pending, msg)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate pending messages: %w", err)
	}

	if len(pending) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, msg := range pending {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO dead_letters (owner_id, original_message_id, to_agent, from_agent, body, subject, priority, metadata)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			ownerID, msg.id, agentName, msg.fromAgent, msg.body, msg.subject, msg.priority, msg.metadata,
		)
		if err != nil {
			return 0, fmt.Errorf("insert dead letter: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return len(pending), nil
}

// ListDeadLetters returns dead letters for an owner, optionally including acknowledged ones.
// Returns the list and the total count of unacknowledged dead letters.
func (s *DeadLetterStore) ListDeadLetters(ctx context.Context, ownerID int64, includeAcknowledged bool, limit int) ([]DeadLetter, int, error) {
	if limit <= 0 {
		limit = 50
	}

	var query string
	var args []any

	if includeAcknowledged {
		query = `SELECT id, owner_id, original_message_id, to_agent, from_agent, body, subject, priority, metadata, acknowledged, created_at
			 FROM dead_letters WHERE owner_id = ?
			 ORDER BY created_at DESC
			 LIMIT ?`
		args = []any{ownerID, limit}
	} else {
		query = `SELECT id, owner_id, original_message_id, to_agent, from_agent, body, subject, priority, metadata, acknowledged, created_at
			 FROM dead_letters WHERE owner_id = ? AND acknowledged = 0
			 ORDER BY created_at DESC
			 LIMIT ?`
		args = []any{ownerID, limit}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query dead letters: %w", err)
	}
	defer rows.Close()

	var letters []DeadLetter
	for rows.Next() {
		var dl DeadLetter
		var acknowledged int
		var metadata string
		if err := rows.Scan(&dl.ID, &dl.OwnerID, &dl.OriginalMessageID, &dl.ToAgent, &dl.FromAgent,
			&dl.Body, &dl.Subject, &dl.Priority, &metadata, &acknowledged, &dl.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan dead letter: %w", err)
		}
		dl.Acknowledged = acknowledged != 0
		dl.Metadata = json.RawMessage(metadata)
		letters = append(letters, dl)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate dead letters: %w", err)
	}

	if letters == nil {
		letters = []DeadLetter{}
	}

	// Get unacknowledged count
	unackCount, err := s.CountUnacknowledged(ctx, ownerID)
	if err != nil {
		return nil, 0, err
	}

	return letters, unackCount, nil
}

// AcknowledgeDeadLetter marks a dead letter as acknowledged, verifying ownership.
func (s *DeadLetterStore) AcknowledgeDeadLetter(ctx context.Context, id int64, ownerID int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE dead_letters SET acknowledged = 1 WHERE id = ? AND owner_id = ?`,
		id, ownerID,
	)
	if err != nil {
		return fmt.Errorf("acknowledge dead letter: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("dead letter not found or not owned by user")
	}
	return nil
}

// CountUnacknowledged returns the count of unacknowledged dead letters for an owner.
func (s *DeadLetterStore) CountUnacknowledged(ctx context.Context, ownerID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM dead_letters WHERE owner_id = ? AND acknowledged = 0`,
		ownerID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unacknowledged dead letters: %w", err)
	}
	return count, nil
}
