-- 157_create_invoices_and_invoice_orders.sql
--
-- 修复发票表缺失：在生产 Postgres 上创建 invoices / invoice_orders 两张表，
-- 并把旧表 invoice_applications 的历史数据 backfill 到新表。
--
-- 背景：
--   - 服务端 backend/internal/service/invoice_service.go 已经改为往新表写
--     （tx.Invoice.Create()、tx.InvoiceOrder.Create()），但生产迁移链里只有
--     150_add_invoice_applications.sql 建的旧表 invoice_applications，缺少
--     新表的 CREATE TABLE。任何升级后的真实库首次调发票接口都会
--     `relation "invoices" does not exist`。
--   - 没有 backfill 时，活跃发票判断（invoice_service.go 里只查 invoices/
--     invoice_orders）会丢历史已开票订单，导致前端把已开票订单当成"可申请
--     开票"，允许重复开票（合规风险）。
--
-- 表结构对齐对象（权威源）：
--   - backend/ent/migrate/schema.go::InvoicesTable / InvoiceOrdersTable
--   - backend/ent/schema/invoice.go
--   - backend/ent/schema/invoice_order.go
--
-- 幂等性：
--   - CREATE TABLE / CREATE INDEX 都用 IF NOT EXISTS。
--   - 外键随 CREATE TABLE 一并声明，受 IF NOT EXISTS 保护。
--   - Backfill 用 WHERE NOT EXISTS 防重复插入；setval 仅在有数据时执行。
--
-- 注意：
--   - 不删除 invoice_applications 旧表、也不删 payment_orders.invoice_status /
--     payment_orders.invoice_file_media_id —— 这些下线属于另一个收尾迁移，
--     本迁移只补缺失的建表 + backfill，避免任何下线动作。

CREATE TABLE IF NOT EXISTS invoices (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL,
    user_email      VARCHAR(255) NOT NULL DEFAULT '',
    status          VARCHAR(20) NOT NULL DEFAULT 'APPLIED',
    invoice_amount  DECIMAL(20, 2) NOT NULL DEFAULT 0,
    order_count     BIGINT NOT NULL DEFAULT 0,
    title           VARCHAR(255) NOT NULL DEFAULT '',
    tax_number      VARCHAR(64) NOT NULL DEFAULT '',
    email           VARCHAR(255) NOT NULL DEFAULT '',
    contact_name    VARCHAR(100) NOT NULL DEFAULT '',
    contact_phone   VARCHAR(32) NOT NULL DEFAULT '',
    request_note    TEXT,
    file_media_id   BIGINT,
    file_name       VARCHAR(255) NOT NULL DEFAULT '',
    file_mime_type  VARCHAR(255) NOT NULL DEFAULT '',
    file_size_bytes BIGINT NOT NULL DEFAULT 0,
    applied_at      TIMESTAMPTZ,
    cancelled_at    TIMESTAMPTZ,
    issued_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引名严格对齐 ent/migrate/schema.go 的 InvoicesTable.Indexes，避免后续如果
-- 切到 ent/atlas 自动诊断时报"索引缺失"。
CREATE INDEX IF NOT EXISTS invoice_user_id_status
    ON invoices (user_id, status);
CREATE INDEX IF NOT EXISTS invoice_status_created_at
    ON invoices (status, created_at);
CREATE INDEX IF NOT EXISTS invoice_created_at
    ON invoices (created_at);

CREATE TABLE IF NOT EXISTS invoice_orders (
    id                  BIGSERIAL PRIMARY KEY,
    order_id            BIGINT NOT NULL,
    pay_amount_snapshot DECIMAL(20, 2) NOT NULL DEFAULT 0,
    out_trade_no        VARCHAR(64) NOT NULL DEFAULT '',
    payment_type        VARCHAR(30) NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    invoice_id          BIGINT NOT NULL,
    CONSTRAINT invoice_orders_invoices_orders
        FOREIGN KEY (invoice_id) REFERENCES invoices (id) ON DELETE NO ACTION
);

CREATE INDEX IF NOT EXISTS invoiceorder_invoice_id
    ON invoice_orders (invoice_id);
CREATE INDEX IF NOT EXISTS invoiceorder_order_id
    ON invoice_orders (order_id);

-- ---------------------------------------------------------------------------
-- Backfill：旧 invoice_applications（一行 = 一张发票 + 一个订单）→ 新双表。
-- 字段映射（与 docs/superpowers/plans/2026-06-08-user-invoice-management.migration.sql
-- 一致；除了 ON DELETE NO ACTION 对齐 ent，索引名对齐 ent）：
--
--   invoice_applications 列         →  invoices 列
--   ----------------------------       --------------------
--   id                              →  id（保留主键以兼容外部引用）
--   user_id                         →  user_id
--   user_email                      →  user_email
--   invoice_status                  →  status
--   invoice_amount                  →  invoice_amount
--   (固定 1)                         →  order_count（旧表 1:1 语义）
--   invoice_title                   →  title
--   tax_number                      →  tax_number
--   email                           →  email
--   contact_name                    →  contact_name
--   contact_phone                   →  contact_phone
--   request_note                    →  request_note
--   file_media_id                   →  file_media_id
--   file_name                       →  file_name
--   file_mime_type                  →  file_mime_type
--   file_size_bytes                 →  file_size_bytes
--   applied_at / cancelled_at /
--   issued_at / created_at /
--   updated_at                      →  同名
--
--   invoice_applications 列         →  invoice_orders 列
--   ----------------------------       --------------------
--   id                              →  invoice_id（关联到上面 backfill 的 invoices.id）
--   order_id                        →  order_id
--   invoice_amount                  →  pay_amount_snapshot（旧表 1:1，发票金额=订单金额）
--   order_out_trade_no              →  out_trade_no
--   payment_type                    →  payment_type
--   created_at                      →  created_at
--
-- 旧表的 provider_instance_id / provider_key 不进入新表（新模型不再按发票
-- 单独冻结这两个值；如需追溯，仍可经 invoice_orders.order_id 反查 payment_orders）。
-- ---------------------------------------------------------------------------

DO $migration$
DECLARE
    new_max_id BIGINT;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'invoice_applications'
    ) THEN
        -- 旧表都不存在（极端：某些手动维护的环境），无数据可迁，结束。
        RETURN;
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
    FROM invoice_applications AS ia
    WHERE NOT EXISTS (
        SELECT 1 FROM invoices AS i WHERE i.id = ia.id
    );

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
    FROM invoice_applications AS ia
    WHERE NOT EXISTS (
        SELECT 1
        FROM invoice_orders AS io
        WHERE io.invoice_id = ia.id
          AND io.order_id   = ia.order_id
    );

    -- 把 invoices.id 序列推进到 MAX(id)，防止后续显式 id 写入主键冲突。
    -- 仅在 invoices 已经有数据时执行（空表时 setval 会消耗 id=1，没必要）。
    SELECT MAX(id) INTO new_max_id FROM invoices;
    IF new_max_id IS NOT NULL THEN
        PERFORM setval(pg_get_serial_sequence('invoices', 'id'), new_max_id);
    END IF;
END;
$migration$;
