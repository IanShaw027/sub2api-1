-- Decouple payment fulfillment lease ownership from updated_at.
-- Ent rewrites updated_at on unrelated order updates, so using it as a CAS
-- version can make a valid worker lose its lease after side-effect updates.
ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS fulfillment_lease_token VARCHAR(64) NOT NULL DEFAULT '';
