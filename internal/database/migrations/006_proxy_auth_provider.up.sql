-- Add 'proxy' to the auth_provider check constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_auth_provider_check;
ALTER TABLE users ADD CONSTRAINT users_auth_provider_check CHECK (auth_provider IN ('local', 'oidc', 'ldap', 'proxy'));
