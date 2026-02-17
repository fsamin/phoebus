-- 002_schema_alignment.up.sql
-- Align schema with technical architecture document

-- E1: Add audit_log table
CREATE TABLE audit_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   UUID,
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_actor ON audit_log(actor_id);
CREATE INDEX idx_audit_log_resource ON audit_log(resource_type, resource_id);

-- E2: Add language and position to codebase_files
ALTER TABLE codebase_files ADD COLUMN language TEXT NOT NULL DEFAULT '';
ALTER TABLE codebase_files ADD COLUMN position INT NOT NULL DEFAULT 0;

-- E3: Add attempts, started_at, completed_at to sync_jobs
ALTER TABLE sync_jobs ADD COLUMN attempts INT NOT NULL DEFAULT 0;
ALTER TABLE sync_jobs ADD COLUMN started_at TIMESTAMPTZ;
ALTER TABLE sync_jobs ADD COLUMN completed_at TIMESTAMPTZ;

-- E4: Add estimated_duration and prerequisites to learning_paths
ALTER TABLE learning_paths ADD COLUMN estimated_duration TEXT;
ALTER TABLE learning_paths ADD COLUMN prerequisites TEXT[] NOT NULL DEFAULT '{}';
