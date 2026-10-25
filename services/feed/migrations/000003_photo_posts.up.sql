-- Photo posts (ADR-0020). Existing rows are videos. A photo post has no HLS
-- key and keeps its pictures in order in images, as a JSON array of
-- objects with image_key, thumbnail_key, width and height.
ALTER TABLE videos ADD COLUMN media_type VARCHAR(10) NOT NULL DEFAULT 'video';
ALTER TABLE videos ADD CONSTRAINT videos_media_type_chk CHECK (media_type IN ('video', 'photo'));
ALTER TABLE videos ADD COLUMN images JSONB NOT NULL DEFAULT '[]';
