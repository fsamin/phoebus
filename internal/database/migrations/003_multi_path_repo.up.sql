-- Support multiple learning paths per git repository
ALTER TABLE learning_paths ADD COLUMN file_path TEXT NOT NULL DEFAULT '';
DROP INDEX idx_learning_paths_repo;
CREATE UNIQUE INDEX idx_learning_paths_repo_path ON learning_paths(repo_id, file_path);
