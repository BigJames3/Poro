package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestPhotoPostsMigration applies 000003_photo_posts over rows of every kind
// the video database already holds, then checks the rollback: refused while
// a photo post exists, clean once it is gone (ADR-0020).
func TestPhotoPostsMigration(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("poro_video"), postgres.WithUsername("poro"), postgres.WithPassword("poro_dev_password"),
		postgres.BasicWaitStrategies())
	if err != nil {
		t.Skipf("postgres testcontainer unavailable: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cwd, err := os.Getwd()
	require.NoError(t, err)
	source, err := iofs.New(os.DirFS(filepath.Join(cwd, "..", "..", "migrations")), ".")
	require.NoError(t, err)
	driver, err := pgx5.WithInstance(db, &pgx5.Config{MultiStatementEnabled: true})
	require.NoError(t, err)
	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	require.NoError(t, err)

	require.NoError(t, m.Migrate(2))
	// One row of each state a video can be in before photo posts.
	_, err = db.ExecContext(ctx, `INSERT INTO videos (id, user_id, title, description, status, content_type, size_bytes,
		source_key, s3_upload_id, duration_ms, width, height, hls_key, thumbnail_key, renditions, failure_code,
		moderation_status, deleted_at) VALUES
	('00000000-0000-7000-8000-000000000001', '00000000-0000-7000-8000-0000000000aa', 'uploading', '', 'uploading', 'video/mp4', 10,
		'videos/a/1/source.mp4', 'up-1', NULL, NULL, NULL, NULL, NULL, '[]', NULL, 'approved', NULL),
	('00000000-0000-7000-8000-000000000002', '00000000-0000-7000-8000-0000000000aa', 'processing', 'd', 'processing', 'video/quicktime', 20,
		'videos/a/2/source.mov', NULL, NULL, NULL, NULL, NULL, NULL, '[]', NULL, 'approved', NULL),
	('00000000-0000-7000-8000-000000000003', '00000000-0000-7000-8000-0000000000aa', 'ready', '#foot', 'ready', 'video/mp4', 30,
		'videos/a/3/source.mp4', NULL, 60000, 720, 1280, 'videos/a/3/hls/master.m3u8', 'videos/a/3/thumb.jpg',
		'[{"name":"360p","bandwidth":800000,"width":360,"height":640,"playlist_key":"k"}]', NULL, 'approved', NULL),
	('00000000-0000-7000-8000-000000000004', '00000000-0000-7000-8000-0000000000aa', 'failed', '', 'failed', 'video/webm', 40,
		'videos/a/4/source.webm', NULL, NULL, NULL, NULL, NULL, NULL, '[]', 'too_long', 'approved', NULL),
	('00000000-0000-7000-8000-000000000005', '00000000-0000-7000-8000-0000000000aa', 'removed', '', 'ready', 'video/mp4', 50,
		'videos/a/5/source.mp4', NULL, 1000, 720, 1280, 'videos/a/5/hls/master.m3u8', 'videos/a/5/thumb.jpg', '[]', NULL, 'removed', NULL),
	('00000000-0000-7000-8000-000000000006', '00000000-0000-7000-8000-0000000000aa', '', '', 'ready', 'video/mp4', 60,
		'videos/a/6/source.mp4', NULL, 1000, 720, 1280, 'videos/a/6/hls/master.m3u8', 'videos/a/6/thumb.jpg', '[]', NULL, 'approved', now())`)
	require.NoError(t, err)
	snapshot := func() string {
		var out string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT string_agg(concat_ws('|', id, user_id, title, description, status,
			content_type, size_bytes, source_key, s3_upload_id, duration_ms, width, height, hls_key, thumbnail_key,
			renditions::text, failure_code, moderation_status, deleted_at IS NOT NULL), E'\n' ORDER BY id) FROM videos`).Scan(&out))
		return out
	}
	before := snapshot()

	require.NoError(t, m.Migrate(3))
	require.Equal(t, before, snapshot(), "existing rows are untouched")
	var videos, photos int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE media_type = 'video'),
		count(*) FILTER (WHERE media_type = 'photo') FROM videos`).Scan(&videos, &photos))
	require.Equal(t, 6, videos, "every existing row is a video")
	require.Zero(t, photos)

	// The constraints refuse what the API never writes.
	for _, bad := range []string{
		`UPDATE videos SET media_type = 'audio' WHERE id = '00000000-0000-7000-8000-000000000003'`,
		`INSERT INTO video_images VALUES ('00000000-0000-7000-8000-000000000001', 11, 'image/jpeg', 1, 'k', NULL, NULL, NULL, NULL)`,
		`INSERT INTO video_images VALUES ('00000000-0000-7000-8000-000000000001', 1, 'image/heic', 1, 'k', NULL, NULL, NULL, NULL)`,
		`INSERT INTO video_images VALUES ('00000000-0000-7000-8000-000000000001', 1, 'image/jpeg', 10485761, 'k', NULL, NULL, NULL, NULL)`,
		`INSERT INTO video_images VALUES ('00000000-0000-7000-8000-0000000000ff', 1, 'image/jpeg', 1, 'k', NULL, NULL, NULL, NULL)`,
	} {
		_, err := db.ExecContext(ctx, bad)
		require.Error(t, err, bad)
	}

	_, err = db.ExecContext(ctx, `INSERT INTO videos (id, user_id, title, status, content_type, size_bytes, source_key, media_type)
		VALUES ('00000000-0000-7000-8000-000000000007', '00000000-0000-7000-8000-0000000000aa', '', 'uploading', 'image/*', 5, 'videos/a/7/', 'photo')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO video_images VALUES
		('00000000-0000-7000-8000-000000000007', 1, 'image/jpeg', 5, 'videos/a/7/source-1', NULL, NULL, NULL, NULL)`)
	require.NoError(t, err)

	err = m.Migrate(2)
	require.Error(t, err, "the rollback refuses to run while a photo post exists")
	require.Contains(t, err.Error(), "rollback_000003_needs_no_photo_post")
	var images int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM video_images`).Scan(&images))
	require.Equal(t, 1, images, "the refused rollback changed nothing")
	// golang-migrate marks a failed migration dirty: the operator forces the
	// version back, as documented in the ADR, once the photo posts are gone.
	require.NoError(t, m.Force(3))
	_, err = db.ExecContext(ctx, `DELETE FROM videos WHERE media_type = 'photo'`)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(2))
	require.Equal(t, before, snapshot(), "the rollback keeps every video")
	var hasTable bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT to_regclass('video_images') IS NOT NULL`).Scan(&hasTable))
	require.False(t, hasTable)

	require.NoError(t, m.Up(), "and the migration applies again")
	require.Equal(t, before, snapshot())
}
