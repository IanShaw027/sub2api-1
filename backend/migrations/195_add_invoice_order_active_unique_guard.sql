-- 195_add_invoice_order_active_unique_guard.sql
--
-- 为 invoice_orders 增加 DB 级“单订单只能挂一张活跃发票”兜底：
--   1. 新增 is_active 反范式列；
--   2. 按 invoices.status 回填现有数据；
--   3. 发现历史重复活跃链路时 fail-closed，避免静默篡改会计状态；
--   4. 在 order_id 上建立仅针对活跃行的 partial unique index。
--
-- 背景：
--   - 现有 Create() 依赖 payment_orders FOR UPDATE + 反查活跃发票做互斥，
--     正常链路正确，但缺少数据库层最终防线。
--   - partial unique index 不能跨表引用 invoices.status，因此需要在
--     invoice_orders 冻结一个同步维护的 is_active 位。
--
-- 幂等性：
--   - ADD COLUMN / CREATE INDEX 均用 IF NOT EXISTS；
--   - 回填语句每次都会按发票状态重算 is_active，重复执行结果一致。
--   - 重复活跃链路需要人工先核对发票状态，不能由迁移静默 demote。

ALTER TABLE invoice_orders
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE invoice_orders io
SET is_active = CASE
    WHEN i.status = 'CANCELLED' THEN FALSE
    ELSE TRUE
END
FROM invoices i
WHERE io.invoice_id = i.id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM invoice_orders io
        WHERE io.is_active = TRUE
        GROUP BY io.order_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot enforce invoice order active uniqueness: duplicate active invoice_orders exist; resolve manually before migration'
            USING ERRCODE = '23505';
    END IF;
END $$;

-- The partial unique index is created concurrently in
-- 195a_add_invoice_order_active_unique_guard_notx.sql. Keep this data-shaping
-- migration transactional so the backfill/dedup state is all-or-nothing.
