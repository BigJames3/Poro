-- An upload never completed is abandoned by the upload janitor (PORO-008):
-- its multipart upload is aborted and the row leaves the owner's list.
ALTER TABLE videos DROP CONSTRAINT videos_status_chk;
ALTER TABLE videos ADD CONSTRAINT videos_status_chk
    CHECK (status IN ('uploading', 'processing', 'ready', 'failed', 'abandoned'));
CREATE INDEX videos_uploading_created_idx ON videos (created_at)
    WHERE status = 'uploading' AND deleted_at IS NULL;
