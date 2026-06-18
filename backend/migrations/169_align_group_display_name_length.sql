ALTER TABLE groups
    ALTER COLUMN display_name TYPE VARCHAR(100)
    USING LEFT(display_name, 100);
