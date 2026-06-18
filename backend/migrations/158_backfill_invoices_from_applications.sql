-- 158_backfill_invoices_from_applications.sql
--
-- 把旧表 invoice_applications 的历史数据 backfill 到 157 新建的 invoices /
-- invoice_orders 两张表。与建表拆分（157 建表必须成功，本文件是 best-effort
-- 的数据迁移），避免 backfill 失败连带回滚建表。
--
-- 字典序 157 < 158 保证 157 建表先于本迁移执行。
--
-- 字段映射（旧表一行 = 一张发票 + 一个订单，1:1）：
--   invoice_applications        →  invoices
--   ----------------------         --------------------
--   id                          →  id（保留主键以兼容外部引用）
--   user_id                     →  user_id
--   user_email                  →  user_email
--   invoice_status              →  status
--   invoice_amount              →  invoice_amount
--   (固定 1)                     →  order_count
--   invoice_title               →  title
--   tax_number / email /
--   contact_name / contact_phone /
--   request_note / file_media_id /
--   file_name / file_mime_type /
--   file_size_bytes / applied_at /
--   cancelled_at / issued_at /
--   created_at / updated_at     →  同名
--
--   invoice_applications        →  invoice_orders
--   ----------------------         --------------------
--   id                          →  invoice_id
--   order_id                    →  order_id
--   invoice_amount              →  pay_amount_snapshot（旧表 1:1，发票金额=订单金额）
--   order_out_trade_no          →  out_trade_no
--   payment_type                →  payment_type
--   created_at                  →  created_at
--
--   旧表 provider_instance_id / provider_key 不进入新表（新模型不再按发票冻结
--   这两个值；如需追溯可经 invoice_orders.order_id 反查 payment_orders）。
--
-- 防御：
--   - 旧表 invoice_applications 不存在（全新安装）：跳过。
--   - invoices 或 invoice_orders 已非空：说明这两张表不是由 157 在本次迁移中
--     刚建出来的（例如曾被 ent auto-migrate 等本迁移之外的途径建过并写入过真实
--     数据）。此时按 ia.id 直接 INSERT 会与已有数据撞主键/产生错关联（旧记录被
--     WHERE NOT EXISTS 跳过丢数据，但其 invoice_orders 仍以 invoice_id=ia.id 插入
--     指向不相干的发票）。为安全起见直接中止迁移，交由人工处置。
--   - 正常 runner 流程下，157 刚建表、两表必为空，本迁移正常执行 backfill。

DO $migration$
DECLARE
    new_max_id BIGINT;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'invoice_applications'
    ) THEN
        RAISE NOTICE 'invoice_applications 表不存在，跳过 backfill（全新安装）';
        RETURN;
    END IF;

    -- 防御：目标表非空说明并非由 157 在本次迁移刚建出来，直接 backfill 有
    -- 撞 id / 错关联风险。中止迁移并提示人工处理，避免静默丢历史数据。
    IF EXISTS (SELECT 1 FROM invoices LIMIT 1)
       OR EXISTS (SELECT 1 FROM invoice_orders LIMIT 1) THEN
        RAISE EXCEPTION 'invoices/invoice_orders 已有数据，疑似经本迁移之外的途径建过；中止 backfill 以避免撞 id / 错关联 / 静默丢历史数据，请人工核对 invoice_applications、invoices、invoice_orders 后处理';
    END IF;

    INSERT INTO invoices (
        id, user_id, user_email, status, invoice_amount, order_count,
        title, tax_number, email, contact_name, contact_phone, request_note,
        file_media_id, file_name, file_mime_type, file_size_bytes,
        applied_at, cancelled_at, issued_at, created_at, updated_at
    )
    SELECT
        ia.id,
        ia.user_id,
        ia.user_email,
        ia.invoice_status,
        ia.invoice_amount,
        1,
        ia.invoice_title,
        ia.tax_number,
        ia.email,
        ia.contact_name,
        ia.contact_phone,
        ia.request_note,
        ia.file_media_id,
        ia.file_name,
        ia.file_mime_type,
        ia.file_size_bytes,
        ia.applied_at,
        ia.cancelled_at,
        ia.issued_at,
        ia.created_at,
        ia.updated_at
    FROM invoice_applications AS ia;

    INSERT INTO invoice_orders (
        invoice_id, order_id, pay_amount_snapshot,
        out_trade_no, payment_type, created_at
    )
    SELECT
        ia.id,
        ia.order_id,
        ia.invoice_amount,
        ia.order_out_trade_no,
        ia.payment_type,
        ia.created_at
    FROM invoice_applications AS ia;

    -- 把 invoices.id 序列推进到 MAX(id)，防止后续显式 id 写入主键冲突。
    SELECT MAX(id) INTO new_max_id FROM invoices;
    IF new_max_id IS NOT NULL THEN
        PERFORM setval(pg_get_serial_sequence('invoices', 'id'), new_max_id);
    END IF;
END;
$migration$;
