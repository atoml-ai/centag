-- Add price_type column to pricing_rules (cost/revenue dual pricing support)
ALTER TABLE pricing_rules ADD COLUMN price_type TEXT DEFAULT 'cost';
