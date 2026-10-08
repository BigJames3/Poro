-- A video removed by moderation stops accepting actions until it is restored.
ALTER TABLE videos_projection DROP CONSTRAINT videos_projection_status_chk;
ALTER TABLE videos_projection ADD CONSTRAINT videos_projection_status_chk
    CHECK (status IN ('ready', 'deleted', 'removed'));
