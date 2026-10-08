-- A video removed by moderation is hidden from everyone but its owner and its
-- media sit in the private quarantine bucket until it is restored.
ALTER TABLE videos
    ADD COLUMN moderation_status VARCHAR(10) NOT NULL DEFAULT 'approved',
    ADD COLUMN moderated_at TIMESTAMPTZ,
    ADD CONSTRAINT videos_moderation_status_chk CHECK (moderation_status IN ('approved', 'removed'));
