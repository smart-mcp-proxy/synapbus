-- Dead letter queue for unread messages of deleted agents
CREATE TABLE IF NOT EXISTS dead_letters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id INTEGER NOT NULL REFERENCES users(id),
    original_message_id INTEGER NOT NULL,
    to_agent TEXT NOT NULL,
    from_agent TEXT NOT NULL,
    body TEXT NOT NULL,
    subject TEXT DEFAULT '',
    priority INTEGER DEFAULT 5,
    metadata TEXT DEFAULT '',
    acknowledged INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_dead_letters_owner ON dead_letters(owner_id, acknowledged);
CREATE INDEX IF NOT EXISTS idx_dead_letters_agent ON dead_letters(to_agent);

-- Add is_system flag to channels
ALTER TABLE channels ADD COLUMN is_system INTEGER DEFAULT 0;

INSERT INTO schema_migrations (version) VALUES (8);
