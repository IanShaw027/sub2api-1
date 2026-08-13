-- Invoice currency snapshots and document the occupancy unique index in schema-aligned form.

ALTER TABLE invoices
    ADD COLUMN IF NOT EXISTS currency VARCHAR(8) NOT NULL DEFAULT '';

ALTER TABLE invoice_orders
    ADD COLUMN IF NOT EXISTS currency VARCHAR(8) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS invoice_orders_order_id_active_unique
    ON invoice_orders (order_id) WHERE is_active = TRUE;
