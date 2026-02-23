-- Remove custom ssh-key auth type (replaced by instance-ssh-key)
-- Migrate any existing ssh-key repos to instance-ssh-key
UPDATE git_repositories SET auth_type = 'instance-ssh-key', credentials = NULL WHERE auth_type = 'ssh-key';

ALTER TABLE git_repositories DROP CONSTRAINT IF EXISTS git_repositories_auth_type_check;
ALTER TABLE git_repositories ADD CONSTRAINT git_repositories_auth_type_check
    CHECK (auth_type IN ('none', 'http-basic', 'http-token', 'instance-ssh-key'));
