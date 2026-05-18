-- Add attachments column to support_ticket_messages
ALTER TABLE support_ticket_messages ADD COLUMN IF NOT EXISTS attachments JSONB NOT NULL DEFAULT '[]'::jsonb;
