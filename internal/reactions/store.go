package reactions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Store defines the storage interface for reactions.
type Store interface {
	Insert(ctx context.Context, r *Reaction) error
	Delete(ctx context.Context, messageID int64, agentName, reaction string) error
	GetByMessageID(ctx context.Context, messageID int64) ([]*Reaction, error)
	GetByMessageIDs(ctx context.Context, messageIDs []int64) (map[int64][]*Reaction, error)
	Exists(ctx context.Context, messageID int64, agentName, reaction string) (bool, error)
	CountByMessage(ctx context.Context, messageID int64) (int, error)
	// GetMessageIDsByState returns message IDs in a channel that have the given workflow state.
	GetMessageIDsByState(ctx context.Context, channelID int64, state string) ([]int64, error)
}

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed reaction store.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Insert(ctx context.Context, r *Reaction) error {
	metadata := r.Metadata
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}

	result, err := s.db.ExecContext(ctx,
		`INSERT INTO message_reactions (message_id, agent_name, reaction, metadata, created_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		r.MessageID, r.AgentName, r.Reaction, string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert reaction: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get reaction id: %w", err)
	}
	r.ID = id
	return nil
}

func (s *SQLiteStore) Delete(ctx context.Context, messageID int64, agentName, reaction string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM message_reactions WHERE message_id = ? AND agent_name = ? AND reaction = ?`,
		messageID, agentName, reaction,
	)
	if err != nil {
		return fmt.Errorf("delete reaction: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("reaction not found")
	}
	return nil
}

func (s *SQLiteStore) GetByMessageID(ctx context.Context, messageID int64) ([]*Reaction, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, message_id, agent_name, reaction, metadata, created_at
		 FROM message_reactions WHERE message_id = ?
		 ORDER BY created_at ASC`, messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("get reactions: %w", err)
	}
	defer rows.Close()
	return scanReactions(rows)
}

func (s *SQLiteStore) GetByMessageIDs(ctx context.Context, messageIDs []int64) (map[int64][]*Reaction, error) {
	if len(messageIDs) == 0 {
		return map[int64][]*Reaction{}, nil
	}

	placeholders := make([]string, len(messageIDs))
	args := make([]any, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT id, message_id, agent_name, reaction, metadata, created_at
		 FROM message_reactions WHERE message_id IN (%s)
		 ORDER BY created_at ASC`,
		strings.Join(placeholders, ","),
	)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get reactions by ids: %w", err)
	}
	defer rows.Close()

	all, err := scanReactions(rows)
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]*Reaction)
	for _, r := range all {
		result[r.MessageID] = append(result[r.MessageID], r)
	}
	return result, nil
}

func (s *SQLiteStore) Exists(ctx context.Context, messageID int64, agentName, reaction string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM message_reactions WHERE message_id = ? AND agent_name = ? AND reaction = ?`,
		messageID, agentName, reaction,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check reaction exists: %w", err)
	}
	return count > 0, nil
}

func (s *SQLiteStore) CountByMessage(ctx context.Context, messageID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM message_reactions WHERE message_id = ?`, messageID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count reactions: %w", err)
	}
	return count, nil
}

func (s *SQLiteStore) GetMessageIDsByState(ctx context.Context, channelID int64, state string) ([]int64, error) {
	var query string
	var args []any

	if state == StateProposed {
		// Messages with no reactions
		query = `SELECT m.id FROM messages m
				 WHERE m.channel_id = ?
				 AND NOT EXISTS (SELECT 1 FROM message_reactions r WHERE r.message_id = m.id)
				 ORDER BY m.created_at DESC`
		args = []any{channelID}
	} else {
		// Find the reaction type for this state
		var reactionType string
		for rt, st := range reactionToState {
			if st == state {
				reactionType = rt
				break
			}
		}
		if reactionType == "" {
			return nil, fmt.Errorf("unknown workflow state: %s", state)
		}

		// Messages where the highest-priority reaction maps to this state
		// We get all messages with this reaction type and filter in app layer
		query = `SELECT DISTINCT r.message_id FROM message_reactions r
				 JOIN messages m ON m.id = r.message_id
				 WHERE m.channel_id = ? AND r.reaction = ?
				 ORDER BY m.created_at DESC`
		args = []any{channelID, reactionType}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get message ids by state: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan message id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func scanReactions(rows *sql.Rows) ([]*Reaction, error) {
	var reactions []*Reaction
	for rows.Next() {
		var r Reaction
		var metadata string
		err := rows.Scan(&r.ID, &r.MessageID, &r.AgentName, &r.Reaction, &metadata, &r.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan reaction: %w", err)
		}
		r.Metadata = json.RawMessage(metadata)
		reactions = append(reactions, &r)
	}
	if reactions == nil {
		reactions = []*Reaction{}
	}
	return reactions, rows.Err()
}
