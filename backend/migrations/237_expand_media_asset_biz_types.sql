ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_biz_type_check_v2
    CHECK (biz_type IN (
        'invoice',
        'ticket',
        'avatar',
        'site_logo',
        'support_qr',
        'announcement',
        'payment_help',
        'image_task'
    )) NOT VALID;

ALTER TABLE media_assets
    VALIDATE CONSTRAINT media_assets_biz_type_check_v2;

ALTER TABLE media_assets
    DROP CONSTRAINT IF EXISTS media_assets_biz_type_check;

ALTER TABLE media_assets
    RENAME CONSTRAINT media_assets_biz_type_check_v2 TO media_assets_biz_type_check;
