ALTER TABLE groups
ADD COLUMN IF NOT EXISTS openai_image_main_model VARCHAR(100) NOT NULL DEFAULT 'gpt-5.4-mini';
