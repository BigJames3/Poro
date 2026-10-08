UPDATE videos_projection SET status = 'ready' WHERE status = 'removed';
ALTER TABLE videos_projection DROP CONSTRAINT videos_projection_status_chk;
ALTER TABLE videos_projection ADD CONSTRAINT videos_projection_status_chk
    CHECK (status IN ('ready', 'deleted'));
