-- Align invoices.status with the uppercase API contract (APPLIED/ISSUED/CANCELLED).
-- Migration 224 created a lowercase CHECK; Go writes uppercase and PostgreSQL rejects it.

ALTER TABLE invoices
    DROP CONSTRAINT IF EXISTS invoices_status_check;

UPDATE invoices
SET status = UPPER(status)
WHERE status IN ('applied', 'issued', 'cancelled');

ALTER TABLE invoices
    ALTER COLUMN status SET DEFAULT 'APPLIED';

ALTER TABLE invoices
    ADD CONSTRAINT invoices_status_check
    CHECK (status IN ('APPLIED', 'ISSUED', 'CANCELLED'));

COMMENT ON COLUMN invoices.status IS 'APPLIED | ISSUED | CANCELLED';
