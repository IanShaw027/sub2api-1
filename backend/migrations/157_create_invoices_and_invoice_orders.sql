-- 157_create_invoices_and_invoice_orders.sql
--
-- 修复发票表缺失：在生产 Postgres 上创建 invoices / invoice_orders 两张表。
--
-- 背景：
--   - 服务端 backend/internal/service/invoice_service.go 已经改为往新表写
--     （tx.Invoice.Create()、tx.InvoiceOrder.Create()），但生产迁移链里只有
--     150_add_invoice_applications.sql 建的旧表 invoice_applications，缺少
--     新表的 CREATE TABLE。任何升级后的真实库首次调发票接口都会
--     `relation "invoices" does not exist`。
--   - 运行时只执行 backend/migrations/ 下的内嵌 SQL（无 ent auto-migrate）。
--
-- 表结构对齐对象（权威源）：
--   - backend/ent/migrate/schema.go::InvoicesTable / InvoiceOrdersTable
--   - backend/ent/schema/invoice.go
--   - backend/ent/schema/invoice_order.go
--
-- 本文件只做建表（必须成功）。历史数据 backfill 拆到 158，避免 best-effort 的
-- backfill 失败时连带回滚掉建表——runner 把单个非 _notx 迁移包在一个事务里。
--
-- 幂等性：CREATE TABLE / CREATE INDEX 均用 IF NOT EXISTS；外键随建表声明。

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

-- 索引名严格对齐 ent/migrate/schema.go 的 InvoicesTable.Indexes。
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
-- 注意：order_id 上没有唯一约束。订单"同时只能挂在一张活跃发票上"的互斥性
-- 下放到应用层（invoice_service.go 的 Create 事务用 SELECT FOR UPDATE + 反查
-- 活跃发票实现），不在 DB 层建 partial unique index。
CREATE INDEX IF NOT EXISTS invoiceorder_order_id
    ON invoice_orders (order_id);
