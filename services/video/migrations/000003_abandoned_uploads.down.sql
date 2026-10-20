DROP INDEX IF EXISTS videos_uploading_created_idx;
UPDATE videos SET status = 'failed' WHERE status = 'abandoned';
ALTER TABLE videos DROP CONSTRAINT videos_status_chk;
ALTER TABLE videos ADD CONSTRAINT videos_status_chk
    CHECK (status IN ('uploading', 'processing', 'ready', 'failed'));
