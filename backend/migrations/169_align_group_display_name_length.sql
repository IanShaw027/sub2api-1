DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM groups
        WHERE display_name IS NOT NULL
          AND char_length(display_name) > 100
    ) THEN
        RAISE EXCEPTION 'groups.display_name has values longer than 100 chars; resolve them before narrowing display_name to VARCHAR(100)';
    END IF;
END
$$;

ALTER TABLE groups
    ALTER COLUMN display_name TYPE VARCHAR(100);
