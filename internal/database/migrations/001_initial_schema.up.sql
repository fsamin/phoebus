-- 001_initial_schema.up.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT UNIQUE NOT NULL,
    email         TEXT,
    display_name  TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'learner' CHECK (role IN ('learner', 'instructor', 'admin')),
    password_hash TEXT,
    external_id   TEXT,
    auth_provider TEXT NOT NULL DEFAULT 'local' CHECK (auth_provider IN ('local', 'oidc', 'ldap')),
    active        BOOLEAN NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_external_id ON users(external_id) WHERE external_id IS NOT NULL;

-- Git Repositories
CREATE TABLE git_repositories (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clone_url      TEXT NOT NULL,
    branch         TEXT NOT NULL DEFAULT 'main',
    auth_type      TEXT NOT NULL DEFAULT 'none' CHECK (auth_type IN ('none', 'ssh-key', 'http-basic', 'http-token')),
    credentials    BYTEA,
    webhook_uuid   UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    sync_status    TEXT NOT NULL DEFAULT 'never_synced' CHECK (sync_status IN ('never_synced', 'syncing', 'synced', 'error')),
    sync_error     TEXT,
    last_synced_at TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Learning Paths
CREATE TABLE learning_paths (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id     UUID NOT NULL REFERENCES git_repositories(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    icon        TEXT,
    tags        TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_learning_paths_repo ON learning_paths(repo_id);

-- Modules
CREATE TABLE modules (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    learning_path_id UUID NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    competencies     TEXT[] NOT NULL DEFAULT '{}',
    position         INT NOT NULL DEFAULT 0,
    file_path        TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_modules_path_position ON modules(learning_path_id, position);
CREATE UNIQUE INDEX idx_modules_path_filepath ON modules(learning_path_id, file_path);

-- Steps
CREATE TABLE steps (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id          UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    title              TEXT NOT NULL,
    type               TEXT NOT NULL CHECK (type IN ('lesson', 'quiz', 'terminal-exercise', 'code-exercise')),
    estimated_duration TEXT,
    content_md         TEXT NOT NULL DEFAULT '',
    exercise_data      JSONB,
    position           INT NOT NULL DEFAULT 0,
    file_path          TEXT NOT NULL,
    deleted_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_steps_module_filepath ON steps(module_id, file_path) WHERE deleted_at IS NULL;

-- Codebase Files (for code exercises)
CREATE TABLE codebase_files (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    step_id   UUID NOT NULL REFERENCES steps(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    content   TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX idx_codebase_files_step_path ON codebase_files(step_id, file_path);

-- Progress
CREATE TABLE progress (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    step_id      UUID NOT NULL REFERENCES steps(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'not_started' CHECK (status IN ('not_started', 'in_progress', 'completed')),
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_progress_user_step ON progress(user_id, step_id);

-- Exercise Attempts
CREATE TABLE exercise_attempts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    step_id    UUID NOT NULL REFERENCES steps(id) ON DELETE CASCADE,
    answers    JSONB NOT NULL DEFAULT '{}',
    is_correct BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_exercise_attempts_user_step ON exercise_attempts(user_id, step_id);

-- Sync Jobs (PostgreSQL-based job queue)
CREATE TABLE sync_jobs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id    UUID NOT NULL REFERENCES git_repositories(id) ON DELETE CASCADE,
    status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    error      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sync_jobs_status ON sync_jobs(status) WHERE status = 'pending';
