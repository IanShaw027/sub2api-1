ALTER TABLE groups ADD COLUMN IF NOT EXISTS display_name TEXT;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS user_selectable BOOLEAN NOT NULL DEFAULT true;

UPDATE groups
SET display_name = name
WHERE display_name IS NULL OR display_name = '';

COMMENT ON COLUMN groups.display_name IS '前台展示名；为空时回退到 name';
COMMENT ON COLUMN groups.user_selectable IS '前台是否允许用户直接选择该线路';
