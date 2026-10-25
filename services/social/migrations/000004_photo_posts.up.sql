-- Photo posts (ADR-0020): the media type of each post, to keep photo posts
-- out of the liked list of a client that cannot display them. Existing rows
-- are videos.
ALTER TABLE videos_projection ADD COLUMN media_type VARCHAR(10) NOT NULL DEFAULT 'video';
ALTER TABLE videos_projection ADD CONSTRAINT videos_projection_media_type_chk CHECK (media_type IN ('video', 'photo'));
