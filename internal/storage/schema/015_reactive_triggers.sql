-- 013: Reactive agent triggering
-- Extends agents with trigger configuration, adds reactive_runs tracking table.

-- Extend agents table with reactive trigger configuration
ALTER TABLE agents ADD COLUMN trigger_mode TEXT NOT NULL DEFAULT 'passive';
ALTER TABLE agents ADD COLUMN cooldown_seconds INTEGER NOT NULL DEFAULT 600;
ALTER TABLE agents ADD COLUMN daily_trigger_budget INTEGER NOT NULL DEFAULT 8;
ALTER TABLE agents ADD COLUMN max_trigger_depth INTEGER NOT NULL DEFAULT 5;
ALTER TABLE agents ADD COLUMN k8s_image TEXT;
ALTER TABLE agents ADD COLUMN k8s_env_json TEXT;
ALTER TABLE agents ADD COLUMN k8s_resource_preset TEXT NOT NULL DEFAULT 'default';
ALTER TABLE agents ADD COLUMN pending_work INTEGER NOT NULL DEFAULT 0;

-- Reactive trigger runs: tracks every trigger evaluation and K8s job lifecycle
CREATE TABLE reactive_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name TEXT NOT NULL REFERENCES agents(name),
    trigger_message_id INTEGER,
    trigger_event TEXT NOT NULL,
    trigger_depth INTEGER NOT NULL DEFAULT 0,
    trigger_from TEXT,
    status TEXT NOT NULL DEFAULT 'queued',
    k8s_job_name TEXT,
    k8s_namespace TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    duration_ms INTEGER,
    error_log TEXT,
    token_cost_json TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_reactive_runs_agent_created ON reactive_runs(agent_name, created_at);
CREATE INDEX idx_reactive_runs_status ON reactive_runs(status);
CREATE INDEX idx_reactive_runs_agent_status ON reactive_runs(agent_name, status);
