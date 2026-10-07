-- Projections of video, social and user events (see internal/consumer).
-- Feed publishes no event: no outbox.

-- A tombstone (deleted_at set, empty media keys) is stored when poro.video.deleted
-- arrives before poro.video.ready, so a late ready never revives the video.
CREATE TABLE videos (
    video_id          UUID PRIMARY KEY,
    author_id         UUID NOT NULL,
    title             VARCHAR(200) NOT NULL DEFAULT '',
    hashtags          TEXT[] NOT NULL DEFAULT '{}',
    thumbnail_key     VARCHAR(500) NOT NULL DEFAULT '',
    hls_key           VARCHAR(500) NOT NULL DEFAULT '',
    duration_ms       INT NOT NULL DEFAULT 0,
    published_at      TIMESTAMPTZ NOT NULL,
    moderation_status VARCHAR(20) NOT NULL DEFAULT 'approved',
    deleted_at        TIMESTAMPTZ,
    CONSTRAINT videos_moderation_chk CHECK (moderation_status IN ('approved', 'pending', 'rejected'))
);
CREATE INDEX videos_visible_published_idx ON videos (published_at DESC, video_id DESC)
    WHERE deleted_at IS NULL AND moderation_status = 'approved';
CREATE INDEX videos_author_published_idx ON videos (author_id, published_at DESC, video_id DESC)
    WHERE deleted_at IS NULL AND moderation_status = 'approved';

CREATE TABLE follows (
    follower_id  UUID NOT NULL,
    following_id UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (follower_id, following_id)
);
CREATE INDEX follows_following_idx ON follows (following_id);

CREATE TABLE author_followers (
    author_id       UUID PRIMARY KEY,
    followers_count BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT author_followers_non_negative_chk CHECK (followers_count >= 0)
);

-- Counters are a projection of social: they are clamped at zero instead of
-- failing, so a missed event can never block the consumer.
CREATE TABLE video_stats (
    video_id       UUID PRIMARY KEY,
    likes_count    BIGINT NOT NULL DEFAULT 0,
    comments_count BIGINT NOT NULL DEFAULT 0,
    shares_count   BIGINT NOT NULL DEFAULT 0,
    trending_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT video_stats_non_negative_chk CHECK (likes_count >= 0 AND comments_count >= 0 AND shares_count >= 0)
);
CREATE INDEX video_stats_trending_idx ON video_stats (trending_score DESC, video_id DESC) WHERE trending_score > 0;

-- Latest public profile snapshot of each author (poro.user.profile.updated).
CREATE TABLE authors (
    author_id    UUID PRIMARY KEY,
    username     VARCHAR(30),
    display_name VARCHAR(50),
    avatar_url   VARCHAR(500),
    updated_at   TIMESTAMPTZ NOT NULL
);

-- Consumer idempotency. Columns must match github.com/poro/shared-go/inbox.Schema.
CREATE TABLE processed_events (
    consumer     VARCHAR(100) NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX processed_events_processed_at_idx ON processed_events (processed_at);
