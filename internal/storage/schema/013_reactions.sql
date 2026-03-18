-- Message reactions for workflow state tracking
-- Supports: approve, reject, in_progress, done, published

CREATE TABLE IF NOT EXISTS message_reactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    agent_name TEXT NOT NULL,
    reaction TEXT NOT NULL CHECK(reaction IN ('approve', 'reject', 'in_progress', 'done', 'published')),
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(message_id, agent_name, reaction)
);

CREATE INDEX idx_reactions_message ON message_reactions(message_id);
CREATE INDEX idx_reactions_agent ON message_reactions(agent_name);
CREATE INDEX idx_reactions_type ON message_reactions(reaction);

-- Channel workflow settings
ALTER TABLE channels ADD COLUMN workflow_enabled BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE channels ADD COLUMN auto_approve BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE channels ADD COLUMN stalemate_remind_after TEXT NOT NULL DEFAULT '24h';
ALTER TABLE channels ADD COLUMN stalemate_escalate_after TEXT NOT NULL DEFAULT '72h';
