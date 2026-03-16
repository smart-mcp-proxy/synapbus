-- A2A inbound gateway: task tracking for external A2A agents sending tasks to SynapBus agents.
CREATE TABLE IF NOT EXISTS a2a_tasks (
    id TEXT PRIMARY KEY,
    context_id TEXT NOT NULL,
    target_agent TEXT NOT NULL,
    source_agent TEXT DEFAULT '',
    conversation_id INTEGER,
    state TEXT NOT NULL DEFAULT 'SUBMITTED',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_a2a_tasks_target ON a2a_tasks(target_agent);
CREATE INDEX IF NOT EXISTS idx_a2a_tasks_state ON a2a_tasks(state);
