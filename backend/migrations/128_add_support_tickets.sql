CREATE TABLE IF NOT EXISTS support_tickets (
    id BIGSERIAL PRIMARY KEY,
    ticket_no VARCHAR(32) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category VARCHAR(32) NOT NULL,
    title VARCHAR(200) NOT NULL,
    status VARCHAR(32) NOT NULL,
    current_form_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    current_revision_no INTEGER NOT NULL DEFAULT 1,
    latest_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_reply_role VARCHAR(16) NOT NULL DEFAULT 'system',
    unread_by_user BOOLEAN NOT NULL DEFAULT FALSE,
    unread_by_admin BOOLEAN NOT NULL DEFAULT FALSE,
    submitted_at TIMESTAMPTZ NULL,
    closed_at TIMESTAMPTZ NULL,
    withdrawn_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_tickets_user_id ON support_tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_support_tickets_status ON support_tickets(status);
CREATE INDEX IF NOT EXISTS idx_support_tickets_category ON support_tickets(category);
CREATE INDEX IF NOT EXISTS idx_support_tickets_created_at ON support_tickets(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_tickets_updated_at ON support_tickets(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_tickets_latest_message_at ON support_tickets(latest_message_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'support_tickets_category_check') THEN
        ALTER TABLE support_tickets
            ADD CONSTRAINT support_tickets_category_check
            CHECK (category IN ('consult', 'refund', 'concurrency_apply', 'rate_apply', 'other')) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'support_tickets_status_check') THEN
        ALTER TABLE support_tickets
            ADD CONSTRAINT support_tickets_status_check
            CHECK (status IN ('submitted', 'processing', 'waiting_user', 'waiting_admin', 'resolved', 'closed', 'withdrawn')) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'support_tickets_last_reply_role_check') THEN
        ALTER TABLE support_tickets
            ADD CONSTRAINT support_tickets_last_reply_role_check
            CHECK (last_reply_role IN ('user', 'admin', 'system')) NOT VALID;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS support_ticket_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_role VARCHAR(16) NOT NULL,
    sender_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    sender_name_snapshot VARCHAR(120) NOT NULL,
    sender_avatar_snapshot TEXT NOT NULL DEFAULT '',
    message_type VARCHAR(16) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_ticket_messages_ticket_id ON support_ticket_messages(ticket_id);
CREATE INDEX IF NOT EXISTS idx_support_ticket_messages_created_at ON support_ticket_messages(ticket_id, created_at, id);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'support_ticket_messages_sender_role_check') THEN
        ALTER TABLE support_ticket_messages
            ADD CONSTRAINT support_ticket_messages_sender_role_check
            CHECK (sender_role IN ('user', 'admin', 'system')) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'support_ticket_messages_message_type_check') THEN
        ALTER TABLE support_ticket_messages
            ADD CONSTRAINT support_ticket_messages_message_type_check
            CHECK (message_type IN ('message', 'system')) NOT VALID;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS support_ticket_revisions (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    revision_no INTEGER NOT NULL,
    title VARCHAR(200) NOT NULL,
    form_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    submitted_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(ticket_id, revision_no)
);

CREATE INDEX IF NOT EXISTS idx_support_ticket_revisions_ticket_id ON support_ticket_revisions(ticket_id);
