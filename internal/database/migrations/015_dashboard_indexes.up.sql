-- Dashboard performance indexes
CREATE INDEX idx_progress_user_status ON progress(user_id, status);
CREATE INDEX idx_progress_user_updated ON progress(user_id, updated_at DESC);
CREATE INDEX idx_steps_module_active ON steps(module_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_exercise_attempts_user ON exercise_attempts(user_id);
