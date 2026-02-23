-- Instance-level key/value settings (SSH keys, etc.)
CREATE TABLE IF NOT EXISTS instance_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    encrypted  BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Add instance-ssh-key to allowed auth types
ALTER TABLE git_repositories DROP CONSTRAINT IF EXISTS git_repositories_auth_type_check;
ALTER TABLE git_repositories ADD CONSTRAINT git_repositories_auth_type_check
    CHECK (auth_type IN ('none', 'ssh-key', 'http-basic', 'http-token', 'instance-ssh-key'));
