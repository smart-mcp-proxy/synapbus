-- 026_goals_completion_summary.sql — record the critic's FINAL summary
-- on the goal row so /goals/<id> can render it without a JOIN through
-- goal_tasks. Also stash the message id the completion came from so
-- the UI can deep-link to the full findings JSON.
--
-- Backfill is unnecessary — existing goals never completed cleanly
-- (the demo had no complete_goal path before this migration).

ALTER TABLE goals ADD COLUMN completion_summary TEXT;
ALTER TABLE goals ADD COLUMN completion_message_id INTEGER
    REFERENCES messages(id) ON DELETE SET NULL;
