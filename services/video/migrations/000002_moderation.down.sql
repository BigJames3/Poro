ALTER TABLE videos
    DROP CONSTRAINT IF EXISTS videos_moderation_status_chk,
    DROP COLUMN IF EXISTS moderated_at,
    DROP COLUMN IF EXISTS moderation_status;
