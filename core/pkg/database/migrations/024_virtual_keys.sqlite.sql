-- 024: Add API key limit columns (budget, rate limit, model whitelist) to api_keys.

ALTER TABLE api_keys ADD COLUMN budget_usd REAL NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN used_usd REAL NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN rate_limit_rpm INTEGER NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN rate_limit_tpm INTEGER NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN model_whitelist TEXT NOT NULL DEFAULT '*';
