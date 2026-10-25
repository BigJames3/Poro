-- Rolls back ADR-0020. Without media_type a photo post would read as a broken
-- video, so the rollback stops while one exists, deleted or not: this check
-- constraint cannot be added then, and nothing below runs. Purge the photo
-- posts (and their objects) first.
ALTER TABLE videos ADD CONSTRAINT rollback_000003_needs_no_photo_post CHECK (media_type <> 'photo');
DROP TABLE IF EXISTS video_images;
ALTER TABLE videos
    DROP CONSTRAINT IF EXISTS rollback_000003_needs_no_photo_post,
    DROP CONSTRAINT IF EXISTS videos_media_type_chk,
    DROP COLUMN IF EXISTS media_type;
