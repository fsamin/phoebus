-- Add content_hash for hash-based sync (skip unchanged content)
ALTER TABLE learning_paths ADD COLUMN IF NOT EXISTS content_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE modules ADD COLUMN IF NOT EXISTS content_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE steps ADD COLUMN IF NOT EXISTS content_hash TEXT NOT NULL DEFAULT '';

-- Add soft-delete support to modules (steps already have deleted_at)
ALTER TABLE modules ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Change progress/exercise_attempts FK from CASCADE to SET NULL
-- so learner progress survives content deletion
ALTER TABLE progress ALTER COLUMN step_id DROP NOT NULL;
ALTER TABLE progress DROP CONSTRAINT IF EXISTS progress_step_id_fkey;
ALTER TABLE progress ADD CONSTRAINT progress_step_id_fkey
    FOREIGN KEY (step_id) REFERENCES steps(id) ON DELETE SET NULL;

ALTER TABLE exercise_attempts ALTER COLUMN step_id DROP NOT NULL;
ALTER TABLE exercise_attempts DROP CONSTRAINT IF EXISTS exercise_attempts_step_id_fkey;
ALTER TABLE exercise_attempts ADD CONSTRAINT exercise_attempts_step_id_fkey
    FOREIGN KEY (step_id) REFERENCES steps(id) ON DELETE SET NULL;

-- Also protect learning_paths from cascade: soft-delete instead of hard-delete
ALTER TABLE learning_paths ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
