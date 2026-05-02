CREATE TABLE IF NOT EXISTS ai_skill_installs (
    id BIGSERIAL PRIMARY KEY,
    skill_id BIGINT NOT NULL REFERENCES ai_skills(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (skill_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_ai_skill_installs_skill_id
    ON ai_skill_installs(skill_id);

CREATE INDEX IF NOT EXISTS idx_ai_skill_installs_user_id
    ON ai_skill_installs(user_id);
