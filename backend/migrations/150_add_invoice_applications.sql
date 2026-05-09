ALTER TABLE payment_provider_instances
ADD COLUMN IF NOT EXISTS invoice_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE payment_orders
ADD COLUMN IF NOT EXISTS invoice_status VARCHAR(20) NOT NULL DEFAULT '';

ALTER TABLE payment_orders
ADD COLUMN IF NOT EXISTS invoice_file_media_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_payment_orders_invoice_status
    ON payment_orders(invoice_status);

CREATE TABLE IF NOT EXISTS invoice_applications (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    user_email VARCHAR(255) NOT NULL DEFAULT '',
    order_out_trade_no VARCHAR(64) NOT NULL DEFAULT '',
    payment_type VARCHAR(30) NOT NULL DEFAULT '',
    provider_instance_id VARCHAR(64) NOT NULL DEFAULT '',
    provider_key VARCHAR(30) NOT NULL DEFAULT '',
    invoice_status VARCHAR(20) NOT NULL DEFAULT 'APPLIED',
    invoice_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    invoice_title VARCHAR(255) NOT NULL DEFAULT '',
    tax_number VARCHAR(64) NOT NULL DEFAULT '',
    email VARCHAR(255) NOT NULL DEFAULT '',
    contact_name VARCHAR(100) NOT NULL DEFAULT '',
    contact_phone VARCHAR(32) NOT NULL DEFAULT '',
    request_note TEXT,
    file_media_id BIGINT,
    file_name VARCHAR(255) NOT NULL DEFAULT '',
    file_mime_type VARCHAR(255) NOT NULL DEFAULT '',
    file_size_bytes BIGINT NOT NULL DEFAULT 0,
    applied_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    issued_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_invoice_applications_order_id
    ON invoice_applications(order_id);

CREATE INDEX IF NOT EXISTS idx_invoice_applications_user_id
    ON invoice_applications(user_id);

CREATE INDEX IF NOT EXISTS idx_invoice_applications_status
    ON invoice_applications(invoice_status);

CREATE INDEX IF NOT EXISTS idx_invoice_applications_created_at
    ON invoice_applications(created_at DESC);
