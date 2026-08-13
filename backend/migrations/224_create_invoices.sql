-- Invoices: user applications covering one or more completed payment orders.
-- Files are stored as private media_assets (biz_type=invoice).

CREATE TABLE IF NOT EXISTS invoices (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL,
    user_email        VARCHAR(255) NOT NULL DEFAULT '',
    status            VARCHAR(16) NOT NULL DEFAULT 'applied',
    unread_by_admin   BOOLEAN NOT NULL DEFAULT TRUE,
    invoice_amount    DECIMAL(20,2) NOT NULL DEFAULT 0,
    order_count       INTEGER NOT NULL DEFAULT 0,
    title             VARCHAR(200) NOT NULL,
    tax_number        VARCHAR(64) NOT NULL DEFAULT '',
    email             VARCHAR(255) NOT NULL,
    contact_name      VARCHAR(100) NOT NULL DEFAULT '',
    contact_phone     VARCHAR(40) NOT NULL DEFAULT '',
    request_note      TEXT NOT NULL DEFAULT '',
    file_media_id     BIGINT,
    file_name         VARCHAR(255) NOT NULL DEFAULT '',
    file_mime_type    VARCHAR(128) NOT NULL DEFAULT '',
    file_size_bytes   BIGINT NOT NULL DEFAULT 0,
    applied_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cancelled_at      TIMESTAMPTZ,
    issued_at         TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT invoices_status_check CHECK (status IN ('applied', 'issued', 'cancelled')),
    CONSTRAINT invoices_order_count_check CHECK (order_count >= 0),
    CONSTRAINT invoices_file_size_check CHECK (file_size_bytes >= 0)
);

CREATE INDEX IF NOT EXISTS idx_invoices_user_status ON invoices (user_id, status);
CREATE INDEX IF NOT EXISTS idx_invoices_status_unread ON invoices (status, unread_by_admin);
CREATE INDEX IF NOT EXISTS idx_invoices_created_at ON invoices (created_at);

CREATE TABLE IF NOT EXISTS invoice_orders (
    id                   BIGSERIAL PRIMARY KEY,
    invoice_id           BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    order_id             BIGINT NOT NULL,
    pay_amount_snapshot  DECIMAL(20,2) NOT NULL DEFAULT 0,
    out_trade_no         VARCHAR(64) NOT NULL DEFAULT '',
    payment_type         VARCHAR(30) NOT NULL DEFAULT '',
    is_active            BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invoice_orders_invoice_id ON invoice_orders (invoice_id);
CREATE INDEX IF NOT EXISTS idx_invoice_orders_order_id ON invoice_orders (order_id);
CREATE UNIQUE INDEX IF NOT EXISTS invoice_orders_order_id_active_unique
    ON invoice_orders (order_id) WHERE is_active = TRUE;

ALTER TABLE payment_provider_instances
    ADD COLUMN IF NOT EXISTS invoice_enabled BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON TABLE invoices IS 'User invoice applications; files live in private media_assets';
COMMENT ON COLUMN invoices.status IS 'applied | issued | cancelled';
COMMENT ON TABLE invoice_orders IS 'One completed payment order may belong to at most one active invoice';
COMMENT ON COLUMN payment_provider_instances.invoice_enabled IS 'When true, completed orders on this channel may request invoices';
