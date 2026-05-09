ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS image_generation_route character varying(20);

UPDATE groups
SET image_generation_route = 'web2api'
WHERE COALESCE(BTRIM(image_generation_route), '') = ''
  AND (
      image_rate_independent = TRUE
      OR images2api_price_1k IS NOT NULL
      OR images2api_price_2k IS NOT NULL
      OR images2api_price_4k IS NOT NULL
  );

UPDATE groups
SET image_generation_route = 'codex'
WHERE COALESCE(BTRIM(image_generation_route), '') = ''
  AND NOT (
      image_rate_independent = TRUE
      OR images2api_price_1k IS NOT NULL
      OR images2api_price_2k IS NOT NULL
      OR images2api_price_4k IS NOT NULL
  );

ALTER TABLE groups
    ALTER COLUMN image_generation_route SET DEFAULT 'codex';

ALTER TABLE groups
    ALTER COLUMN image_generation_route SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_image_generation_route_check'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_image_generation_route_check
            CHECK (image_generation_route IN ('codex', 'web2api'));
    END IF;
END $$;
