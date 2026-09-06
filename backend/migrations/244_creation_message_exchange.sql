ALTER TABLE creation_messages
    ADD COLUMN IF NOT EXISTS exchange_request_id VARCHAR(128);

CREATE UNIQUE INDEX IF NOT EXISTS creationmessage_session_id_exchange_request_id_role
    ON creation_messages (session_id, exchange_request_id, role);
