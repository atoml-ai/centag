-- Add model_vars column to user_config for per-user model variable overrides
ALTER TABLE user_config ADD COLUMN model_vars TEXT DEFAULT '';
