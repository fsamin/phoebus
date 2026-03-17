CREATE TABLE path_dependencies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_path_id  UUID NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
    target_path_id  UUID NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
    dep_type        TEXT NOT NULL DEFAULT 'manual' CHECK (dep_type IN ('manual', 'yaml')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_path_dependency UNIQUE (source_path_id, target_path_id),
    CONSTRAINT chk_no_self_dep CHECK (source_path_id != target_path_id)
);

CREATE INDEX idx_path_deps_source ON path_dependencies(source_path_id);
CREATE INDEX idx_path_deps_target ON path_dependencies(target_path_id);
