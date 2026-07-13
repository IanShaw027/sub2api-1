-- Codex alpha/search web search per-call billing: group-level price override.
-- NULL uses the built-in default of USD 0.01 per call (OpenAI web search pricing: USD 10/1000 calls).
ALTER TABLE groups ADD COLUMN IF NOT EXISTS web_search_price_per_call DECIMAL(20,8);
