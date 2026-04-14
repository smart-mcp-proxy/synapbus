-- 020: Harness run detail — link reactive_runs to harness_runs and
-- capture the rendered prompt + raw response so the Web UI can show
-- operators exactly what the agent saw and what it replied.

ALTER TABLE harness_runs ADD COLUMN reactive_run_id INTEGER;
ALTER TABLE harness_runs ADD COLUMN prompt          TEXT;
ALTER TABLE harness_runs ADD COLUMN response        TEXT;

CREATE INDEX idx_harness_runs_reactive ON harness_runs(reactive_run_id);
