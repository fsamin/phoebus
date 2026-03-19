-- Restore the unique constraint on module positions
DROP INDEX IF EXISTS idx_modules_path_position;
CREATE UNIQUE INDEX idx_modules_path_position ON modules(learning_path_id, position);
