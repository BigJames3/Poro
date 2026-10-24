-- Photo posts (ADR-0020): a publication is a video or a photo post. Existing
-- rows become videos through the column default, no row is rewritten.
ALTER TABLE videos
    ADD COLUMN media_type VARCHAR(10) NOT NULL DEFAULT 'video',
    ADD CONSTRAINT videos_media_type_chk CHECK (media_type IN ('video', 'photo'));

-- The 1 to 10 images of a photo post, in display order. Sources live in the
-- private source bucket until processing. image_key, thumbnail_key, width and
-- height are filled by the media-worker.
CREATE TABLE video_images (
    video_id      UUID NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
    position      SMALLINT NOT NULL,
    content_type  VARCHAR(30) NOT NULL,
    size_bytes    BIGINT NOT NULL,
    source_key    VARCHAR(500) NOT NULL,
    image_key     VARCHAR(500),
    thumbnail_key VARCHAR(500),
    width         INT,
    height        INT,
    PRIMARY KEY (video_id, position),
    CONSTRAINT video_images_position_chk CHECK (position BETWEEN 1 AND 10),
    CONSTRAINT video_images_size_chk CHECK (size_bytes BETWEEN 1 AND 10485760),
    CONSTRAINT video_images_content_type_chk CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp'))
);
