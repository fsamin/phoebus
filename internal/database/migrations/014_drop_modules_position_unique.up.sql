-- Drop the unique constraint on module positions.
-- Position ordering is managed by the syncer; the unique constraint causes
-- conflicts during resync when modules are reordered or have duplicate positions.
DROP INDEX IF EXISTS idx_modules_path_position;

-- Keep a non-unique index for query performance
CREATE INDEX idx_modules_path_position ON modules(learning_path_id, position);
