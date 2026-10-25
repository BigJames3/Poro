-- Photo posts (ADR-0020). Existing rows are videos. Hashtags count visible
-- videos and visible photo posts apart: a client without photo posts sees
-- only the video count, and no tag carried by photo posts alone.
ALTER TABLE videos ADD COLUMN media_type VARCHAR(10) NOT NULL DEFAULT 'video';
ALTER TABLE videos ADD CONSTRAINT videos_media_type_chk CHECK (media_type IN ('video', 'photo'));
ALTER TABLE hashtags ADD COLUMN photos_count BIGINT NOT NULL DEFAULT 0;
ALTER TABLE hashtags ADD CONSTRAINT hashtags_photos_count_chk CHECK (photos_count >= 0);
