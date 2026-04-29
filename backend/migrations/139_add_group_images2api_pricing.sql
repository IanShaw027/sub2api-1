ALTER TABLE groups ADD COLUMN IF NOT EXISTS images2api_price_1k DECIMAL(20,8);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS images2api_price_2k DECIMAL(20,8);
ALTER TABLE groups ADD COLUMN IF NOT EXISTS images2api_price_4k DECIMAL(20,8);

COMMENT ON COLUMN groups.images2api_price_1k IS 'images2api 1K image fixed unit price (USD)';
COMMENT ON COLUMN groups.images2api_price_2k IS 'images2api 2K image fixed unit price (USD)';
COMMENT ON COLUMN groups.images2api_price_4k IS 'images2api 4K image fixed unit price (USD)';
