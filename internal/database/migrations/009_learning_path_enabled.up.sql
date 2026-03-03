-- Add enabled column to learning_paths for admin path management
ALTER TABLE learning_paths ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true;
