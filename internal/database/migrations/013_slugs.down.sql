-- Remove slug columns from learning_paths, modules, and steps

DROP INDEX IF EXISTS idx_learning_paths_slug;
DROP INDEX IF EXISTS idx_modules_slug;
DROP INDEX IF EXISTS idx_steps_slug;

ALTER TABLE learning_paths DROP COLUMN IF EXISTS slug;
ALTER TABLE modules DROP COLUMN IF EXISTS slug;
ALTER TABLE steps DROP COLUMN IF EXISTS slug;
