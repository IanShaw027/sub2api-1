INSERT INTO settings (key, value)
VALUES ('ai_studio_enabled', 'false')
ON CONFLICT (key) DO NOTHING;
