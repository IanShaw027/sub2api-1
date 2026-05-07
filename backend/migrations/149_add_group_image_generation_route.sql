ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS image_generation_route character varying(20) NOT NULL DEFAULT 'codex';

UPDATE groups
SET image_generation_route = 'codex'
WHERE COALESCE(BTRIM(image_generation_route), '') = '';

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
