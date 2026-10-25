-- The feed is a projection: photo posts are dropped rather than shown as
-- videos without HLS. Replaying poro.video.ready rebuilds them.
DELETE FROM videos WHERE media_type = 'photo';
ALTER TABLE videos DROP COLUMN images;
ALTER TABLE videos DROP CONSTRAINT videos_media_type_chk;
ALTER TABLE videos DROP COLUMN media_type;
