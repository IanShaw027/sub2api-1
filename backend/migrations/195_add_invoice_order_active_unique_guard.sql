-- 195_add_invoice_order_active_unique_guard.sql
--
-- 为 invoice_orders 增加 DB 级“单订单只能挂一张活跃发票”兜底：
--   1. 新增 is_active 反范式列；
--   2. 按 invoices.status 回填现有数据；
--   3. 在 order_id 上建立仅针对活跃行的 partial unique index。
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

ALTER TABLE invoice_orders
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE invoice_orders io
SET is_active = CASE
    WHEN i.status = 'CANCELLED' THEN FALSE
    ELSE TRUE
END
FROM invoices i
WHERE io.invoice_id = i.id;

WITH ranked_active AS (
    SELECT
        io.id,
        ROW_NUMBER() OVER (
            PARTITION BY io.order_id
            ORDER BY
                CASE i.status
                    WHEN 'ISSUED' THEN 0
                    WHEN 'APPLIED' THEN 1
                    ELSE 2
                END,
                io.invoice_id DESC,
                io.id DESC
        ) AS rn
    FROM invoice_orders io
    JOIN invoices i ON i.id = io.invoice_id
    WHERE io.is_active = TRUE
)
UPDATE invoice_orders io
SET is_active = FALSE
FROM ranked_active ra
WHERE io.id = ra.id
  AND ra.rn > 1;

-- The partial unique index is created concurrently in
-- 195a_add_invoice_order_active_unique_guard_notx.sql. Keep this data-shaping
-- migration transactional so the backfill/dedup state is all-or-nothing.
