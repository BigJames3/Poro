-- The search index is a projection: photo posts are dropped rather than
-- returned as videos. Replaying poro.video.ready rebuilds them.
DELETE FROM videos WHERE media_type = 'photo';
ALTER TABLE hashtags DROP CONSTRAINT hashtags_photos_count_chk;
ALTER TABLE hashtags DROP COLUMN photos_count;
ALTER TABLE videos DROP CONSTRAINT videos_media_type_chk;
ALTER TABLE videos DROP COLUMN media_type;
