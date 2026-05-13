-- Add fields required by self-service refund preview and approval flow.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS refund_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0;

COMMENT ON COLUMN groups.refund_rate_multiplier IS '订阅退款倍率，用于将已消耗额度折算为不可退金额';

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS refund_requested_amount DECIMAL(20,2) NOT NULL DEFAULT 0;

COMMENT ON COLUMN payment_orders.refund_requested_amount IS '用户申请退款时填写的退款金额；实际已退款金额仍记录在 refund_amount';
