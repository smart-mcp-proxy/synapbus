package messaging

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RetentionConfig holds message retention settings.
type RetentionConfig struct {
	// RetentionPeriod is how long messages are kept. 0 disables retention.
	RetentionPeriod time.Duration
	// WarningWindow is how long before deletion to send warnings (default 30 days).
	WarningWindow time.Duration
	// CleanupInterval is how often the cleanup job runs (default 24h).
	CleanupInterval time.Duration
	// Enabled is true if retention is active (RetentionPeriod > 0).
	Enabled bool
}

// DefaultRetentionConfig returns the default retention configuration (12 months).
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		RetentionPeriod: 365 * 24 * time.Hour, // 12 months ≈ 365 days
		WarningWindow:   30 * 24 * time.Hour,  // 1 month
		CleanupInterval: 24 * time.Hour,
		Enabled:         true,
	}
}

// ParseRetentionPeriod parses a retention string like "12m" (months), "365d" (days), "8760h" (hours), or "0" (disabled).
func ParseRetentionPeriod(s string) RetentionConfig {
	cfg := DefaultRetentionConfig()

	if s == "" || s == "0" {
		cfg.Enabled = false
		cfg.RetentionPeriod = 0
		return cfg
	}

	s = strings.TrimSpace(s)

	// Try "Nm" format (months)
	if strings.HasSuffix(s, "m") && !strings.HasSuffix(s, "ms") {
		months, err := strconv.Atoi(strings.TrimSuffix(s, "m"))
		if err == nil && months > 0 {
			cfg.RetentionPeriod = time.Duration(months) * 30 * 24 * time.Hour
			return cfg
		}
	}

	// Try "Nd" format (days)
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil && days > 0 {
			cfg.RetentionPeriod = time.Duration(days) * 24 * time.Hour
			return cfg
		}
	}

	// Try standard Go duration
	d, err := time.ParseDuration(s)
	if err == nil && d > 0 {
		cfg.RetentionPeriod = d
		return cfg
	}

	// Invalid — disable
	cfg.Enabled = false
	cfg.RetentionPeriod = 0
	return cfg
}

// RetentionPeriodHuman returns a human-readable retention period string.
func (c RetentionConfig) RetentionPeriodHuman() string {
	if !c.Enabled {
		return "disabled"
	}
	days := int(c.RetentionPeriod.Hours() / 24)
	if days%30 == 0 {
		months := days / 30
		if months == 1 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", months)
	}
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// RetentionWorker runs periodic message cleanup.
type RetentionWorker struct {
	db              *sql.DB
	config          RetentionConfig
	dataDir         string
	logger          *slog.Logger
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mu              sync.RWMutex
	lastCleanupAt   *time.Time
	lastDeleteCount int64
}

// NewRetentionWorker creates a new retention worker.
func NewRetentionWorker(db *sql.DB, config RetentionConfig, dataDir string) *RetentionWorker {
	return &RetentionWorker{
		db:      db,
		config:  config,
		dataDir: dataDir,
		logger:  slog.Default().With("component", "retention-worker"),
	}
}

// Start begins the retention cleanup loop.
func (w *RetentionWorker) Start() {
	if !w.config.Enabled {
		w.logger.Info("message retention disabled")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	w.logger.Info("starting message retention worker",
		"retention_period", w.config.RetentionPeriod.String(),
		"warning_window", w.config.WarningWindow.String(),
		"cleanup_interval", w.config.CleanupInterval.String(),
	)

	w.wg.Add(1)
	go w.loop(ctx)
}

// Stop halts the retention worker.
func (w *RetentionWorker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	w.logger.Info("retention worker stopped")
}

// Status returns the current retention status for admin queries.
func (w *RetentionWorker) Status() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := map[string]interface{}{
		"enabled":              w.config.Enabled,
		"retention_period":     w.config.RetentionPeriod.String(),
		"retention_period_human": w.config.RetentionPeriodHuman(),
		"warning_window":       w.config.WarningWindow.String(),
		"cleanup_interval":     w.config.CleanupInterval.String(),
	}

	if w.lastCleanupAt != nil {
		result["last_cleanup_at"] = w.lastCleanupAt.Format(time.RFC3339)
		next := w.lastCleanupAt.Add(w.config.CleanupInterval)
		result["next_cleanup_at"] = next.Format(time.RFC3339)
	}

	// Get message age distribution
	dist, total := w.messageAgeDistribution()
	result["message_age_distribution"] = dist
	result["total_messages"] = total

	return result
}

func (w *RetentionWorker) loop(ctx context.Context) {
	defer w.wg.Done()

	// Run initial cleanup after a short delay
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			w.runCleanup(ctx)
			timer.Reset(w.config.CleanupInterval)
		}
	}
}

func (w *RetentionWorker) runCleanup(ctx context.Context) {
	w.logger.Info("starting retention cleanup")

	// Step 1: Send warnings for messages approaching retention
	warned := w.sendRetentionWarnings(ctx)

	// Step 2: Delete expired messages
	deleted := w.deleteExpiredMessages(ctx)

	// Step 3: Run incremental vacuum
	w.runIncrementalVacuum(ctx)

	now := time.Now()
	w.mu.Lock()
	w.lastCleanupAt = &now
	w.lastDeleteCount = deleted
	w.mu.Unlock()

	w.logger.Info("retention cleanup complete",
		"warnings_sent", warned,
		"messages_deleted", deleted,
	)
}

func (w *RetentionWorker) sendRetentionWarnings(ctx context.Context) int64 {
	if w.config.WarningWindow <= 0 {
		return 0
	}

	// Find conversations with messages in the warning window
	// (older than retention - warning, but not yet at retention limit)
	warningCutoff := time.Now().Add(-(w.config.RetentionPeriod - w.config.WarningWindow))
	retentionCutoff := time.Now().Add(-w.config.RetentionPeriod)

	rows, err := w.db.QueryContext(ctx,
		`SELECT DISTINCT c.id, c.subject, m.to_agent, m.from_agent
		 FROM conversations c
		 JOIN messages m ON m.conversation_id = c.id
		 WHERE m.created_at < ? AND m.created_at >= ?
		 AND m.from_agent != 'system'
		 AND NOT EXISTS (
			SELECT 1 FROM messages warn
			WHERE warn.conversation_id = c.id
			AND warn.from_agent = 'system'
			AND warn.body LIKE '%will be permanently deleted%'
			AND warn.created_at > ?
		 )`,
		warningCutoff, retentionCutoff,
		time.Now().Add(-w.config.CleanupInterval), // Don't re-warn within the same interval
	)
	if err != nil {
		w.logger.Error("query warning candidates failed", "error", err)
		return 0
	}
	defer rows.Close()

	type warnTarget struct {
		convID    int64
		subject   string
		toAgent   string
		fromAgent string
	}

	var targets []warnTarget
	for rows.Next() {
		var t warnTarget
		if err := rows.Scan(&t.convID, &t.subject, &t.toAgent, &t.fromAgent); err != nil {
			continue
		}
		targets = append(targets, t)
	}

	// Send system warnings to unique agents
	warned := int64(0)
	agentsSeen := make(map[string]bool)
	for _, t := range targets {
		for _, agent := range []string{t.toAgent, t.fromAgent} {
			if agent == "" || agent == "system" || agentsSeen[agent] {
				continue
			}
			agentsSeen[agent] = true

			subject := t.subject
			if subject == "" {
				subject = "(no subject)"
			}

			body := fmt.Sprintf("Conversation '%s' has messages older than %s. These will be permanently deleted in approximately %s.",
				subject,
				w.config.RetentionPeriodHuman(),
				w.formatDuration(w.config.WarningWindow),
			)

			_, err := w.db.ExecContext(ctx,
				`INSERT INTO conversations (subject, created_by, created_at, updated_at)
				 VALUES (?, 'system', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
				"Retention Warning",
			)
			if err != nil {
				continue
			}

			var convID int64
			w.db.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&convID)

			_, err = w.db.ExecContext(ctx,
				`INSERT INTO messages (conversation_id, from_agent, to_agent, body, priority, status, metadata, created_at, updated_at)
				 VALUES (?, 'system', ?, ?, 5, 'pending', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
				convID, agent, body,
			)
			if err != nil {
				w.logger.Error("send retention warning failed", "agent", agent, "error", err)
				continue
			}
			warned++
		}
	}

	return warned
}

func (w *RetentionWorker) deleteExpiredMessages(ctx context.Context) int64 {
	cutoff := time.Now().Add(-w.config.RetentionPeriod)

	// Get message IDs to delete (skip processing status)
	rows, err := w.db.QueryContext(ctx,
		`SELECT id FROM messages WHERE created_at < ? AND status != 'processing'`,
		cutoff,
	)
	if err != nil {
		w.logger.Error("query expired messages failed", "error", err)
		return 0
	}

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	rows.Close()

	if len(ids) == 0 {
		return 0
	}

	// Delete in batches
	batchSize := 500
	totalDeleted := int64(0)

	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]
		deleted := w.deleteBatch(ctx, batch)
		totalDeleted += deleted
	}

	// Clean up orphaned conversations
	orphaned, err := w.db.ExecContext(ctx,
		`DELETE FROM conversations WHERE id NOT IN (SELECT DISTINCT conversation_id FROM messages)`,
	)
	if err == nil {
		if n, _ := orphaned.RowsAffected(); n > 0 {
			w.logger.Info("cleaned orphaned conversations", "count", n)
		}
	}

	return totalDeleted
}

func (w *RetentionWorker) deleteBatch(ctx context.Context, ids []int64) int64 {
	if len(ids) == 0 {
		return 0
	}

	// Build placeholders
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	ph := strings.Join(placeholders, ",")

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		w.logger.Error("begin tx failed", "error", err)
		return 0
	}
	defer tx.Rollback()

	// Delete embedding queue entries
	tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM embedding_queue WHERE message_id IN (%s)`, ph), args...)

	// Delete embeddings
	tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM embeddings WHERE message_id IN (%s)`, ph), args...)

	// Collect attachment hashes before deleting
	hashRows, _ := tx.QueryContext(ctx,
		fmt.Sprintf(`SELECT DISTINCT hash FROM attachments WHERE message_id IN (%s)`, ph), args...)
	var hashes []string
	if hashRows != nil {
		for hashRows.Next() {
			var h string
			hashRows.Scan(&h)
			hashes = append(hashes, h)
		}
		hashRows.Close()
	}

	// Delete attachment records
	tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM attachments WHERE message_id IN (%s)`, ph), args...)

	// Delete messages (FTS trigger handles FTS cleanup)
	result, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM messages WHERE id IN (%s)`, ph), args...)
	if err != nil {
		w.logger.Error("delete messages failed", "error", err)
		return 0
	}

	if err := tx.Commit(); err != nil {
		w.logger.Error("commit failed", "error", err)
		return 0
	}

	deleted, _ := result.RowsAffected()

	// Clean up orphaned attachment files (outside transaction)
	w.cleanupAttachmentFiles(ctx, hashes)

	return deleted
}

func (w *RetentionWorker) cleanupAttachmentFiles(ctx context.Context, hashes []string) {
	for _, hash := range hashes {
		// Check if any other attachment references this hash
		var count int
		err := w.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM attachments WHERE hash = ?`, hash,
		).Scan(&count)
		if err != nil || count > 0 {
			continue
		}

		// Remove file from CAS
		prefix := hash[:2]
		filePath := filepath.Join(w.dataDir, "attachments", prefix, hash)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			w.logger.Warn("remove attachment file failed", "hash", hash, "error", err)
		}
	}
}

func (w *RetentionWorker) runIncrementalVacuum(ctx context.Context) {
	_, err := w.db.ExecContext(ctx, `PRAGMA incremental_vacuum(1000)`)
	if err != nil {
		w.logger.Warn("incremental vacuum failed", "error", err)
	}
}

func (w *RetentionWorker) messageAgeDistribution() (map[string]int64, int64) {
	ctx := context.Background()
	dist := map[string]int64{
		"< 1 month":   0,
		"1-3 months":  0,
		"3-6 months":  0,
		"6-12 months": 0,
		"> 12 months": 0,
	}

	now := time.Now()
	boundaries := []struct {
		label  string
		cutoff time.Time
	}{
		{"< 1 month", now.Add(-30 * 24 * time.Hour)},
		{"1-3 months", now.Add(-90 * 24 * time.Hour)},
		{"3-6 months", now.Add(-180 * 24 * time.Hour)},
		{"6-12 months", now.Add(-365 * 24 * time.Hour)},
	}

	var total int64

	// Count messages in each bucket
	for i, b := range boundaries {
		var count int64
		var err error
		if i == 0 {
			err = w.db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM messages WHERE created_at >= ?`, b.cutoff,
			).Scan(&count)
		} else {
			err = w.db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM messages WHERE created_at < ? AND created_at >= ?`,
				boundaries[i-1].cutoff, b.cutoff,
			).Scan(&count)
		}
		if err == nil {
			dist[b.label] = count
			total += count
		}
	}

	// > 12 months
	var oldCount int64
	w.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages WHERE created_at < ?`,
		boundaries[len(boundaries)-1].cutoff,
	).Scan(&oldCount)
	dist["> 12 months"] = oldCount
	total += oldCount

	return dist, total
}

func (w *RetentionWorker) formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days%30 == 0 && days > 0 {
		months := days / 30
		if months == 1 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", months)
	}
	if days == 1 {
		return "1 day"
	}
	if days > 0 {
		return fmt.Sprintf("%d days", days)
	}
	return d.String()
}

// PurgeMessages deletes messages matching the given filters.
// At least one filter must be specified. Returns counts of deleted items.
func PurgeMessages(ctx context.Context, db *sql.DB, dataDir string, olderThan time.Duration, agent string, channel string) (map[string]int64, error) {
	if olderThan == 0 && agent == "" && channel == "" {
		return nil, fmt.Errorf("at least one filter is required")
	}

	var conditions []string
	var args []interface{}

	if olderThan > 0 {
		cutoff := time.Now().Add(-olderThan)
		conditions = append(conditions, "m.created_at < ?")
		args = append(args, cutoff)
	}
	if agent != "" {
		conditions = append(conditions, "(m.from_agent = ? OR m.to_agent = ?)")
		args = append(args, agent, agent)
	}
	if channel != "" {
		conditions = append(conditions, "m.channel_id IN (SELECT id FROM channels WHERE name = ?)")
		args = append(args, channel)
	}

	where := strings.Join(conditions, " AND ")
	query := fmt.Sprintf(`SELECT m.id FROM messages m WHERE %s`, where)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages to purge: %w", err)
	}

	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	if len(ids) == 0 {
		return map[string]int64{
			"deleted_messages":     0,
			"deleted_embeddings":   0,
			"deleted_attachments":  0,
			"cleaned_conversations": 0,
		}, nil
	}

	// Build placeholders for batch delete
	placeholders := make([]string, len(ids))
	batchArgs := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		batchArgs[i] = id
	}
	ph := strings.Join(placeholders, ",")

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Count embeddings before delete
	var embCount int64
	tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM embeddings WHERE message_id IN (%s)`, ph), batchArgs...).Scan(&embCount)

	// Count attachments before delete
	var attCount int64
	tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM attachments WHERE message_id IN (%s)`, ph), batchArgs...).Scan(&attCount)

	// Collect attachment hashes
	hashRows, _ := tx.QueryContext(ctx,
		fmt.Sprintf(`SELECT DISTINCT hash FROM attachments WHERE message_id IN (%s)`, ph), batchArgs...)
	var hashes []string
	if hashRows != nil {
		for hashRows.Next() {
			var h string
			hashRows.Scan(&h)
			hashes = append(hashes, h)
		}
		hashRows.Close()
	}

	// Cascade delete
	tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM embedding_queue WHERE message_id IN (%s)`, ph), batchArgs...)
	tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM embeddings WHERE message_id IN (%s)`, ph), batchArgs...)
	tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM attachments WHERE message_id IN (%s)`, ph), batchArgs...)
	tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM messages WHERE id IN (%s)`, ph), batchArgs...)

	// Clean orphaned conversations
	convResult, _ := tx.ExecContext(ctx,
		`DELETE FROM conversations WHERE id NOT IN (SELECT DISTINCT conversation_id FROM messages)`)
	var convCount int64
	if convResult != nil {
		convCount, _ = convResult.RowsAffected()
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit purge: %w", err)
	}

	// Clean attachment files outside transaction
	for _, hash := range hashes {
		var count int
		db.QueryRowContext(ctx, `SELECT COUNT(*) FROM attachments WHERE hash = ?`, hash).Scan(&count)
		if count == 0 {
			prefix := hash[:2]
			filePath := filepath.Join(dataDir, "attachments", prefix, hash)
			os.Remove(filePath)
		}
	}

	return map[string]int64{
		"deleted_messages":     int64(len(ids)),
		"deleted_embeddings":   embCount,
		"deleted_attachments":  attCount,
		"cleaned_conversations": convCount,
	}, nil
}
