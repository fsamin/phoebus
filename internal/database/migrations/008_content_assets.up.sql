-- 008_content_assets.up.sql

-- Content assets (deduplicated by hash)
CREATE TABLE content_assets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_hash    TEXT NOT NULL UNIQUE,
    content_type    TEXT NOT NULL,
    file_name       TEXT NOT NULL,
    size_bytes      BIGINT NOT NULL,
    storage_backend TEXT NOT NULL DEFAULT 'filesystem',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Relationship between steps and assets
CREATE TABLE step_assets (
    step_id       UUID NOT NULL REFERENCES steps(id) ON DELETE CASCADE,
    asset_id      UUID NOT NULL REFERENCES content_assets(id),
    original_path TEXT NOT NULL,
    PRIMARY KEY (step_id, asset_id)
);

CREATE INDEX idx_step_assets_asset ON step_assets(asset_id);
