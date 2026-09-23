-- Step-level analytics aggregate per step_id; every existing index on these
-- tables leads with user_id and cannot serve those lookups.
CREATE INDEX idx_progress_step ON progress(step_id);
CREATE INDEX idx_exercise_attempts_step ON exercise_attempts(step_id);
