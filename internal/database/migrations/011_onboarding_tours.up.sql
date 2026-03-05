-- Add onboarding tour tracking to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS onboarding_tours_seen JSONB NOT NULL DEFAULT '{}';
