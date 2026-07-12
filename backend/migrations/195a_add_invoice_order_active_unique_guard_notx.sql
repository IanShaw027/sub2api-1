-- Create the partial unique index under a new name first so the legacy
-- non-unique invoiceorder_order_id lookup index keeps serving reads while
-- the unique build runs. Only drop the old index after uniqueness exists;
-- if CREATE fails, the previous index remains intact.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS invoiceorder_order_id_active_unique
    ON invoice_orders (order_id)
    WHERE is_active = TRUE;

DROP INDEX CONCURRENTLY IF EXISTS invoiceorder_order_id;
