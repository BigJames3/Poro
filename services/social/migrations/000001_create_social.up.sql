-- Projections of other services, fed by Kafka (see internal/consumer).
CREATE TABLE videos_projection (
    video_id   UUID PRIMARY KEY,
    owner_id   UUID NOT NULL,
    status     VARCHAR(20) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT videos_projection_status_chk CHECK (status IN ('ready', 'deleted'))
);
CREATE INDEX videos_projection_owner_idx ON videos_projection (owner_id);

-- Accounts known to exist: from poro.auth.user.created, or a caller with a valid token.
CREATE TABLE users_projection (
    user_id    UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE likes (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL,
    video_id   UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT likes_user_video_key UNIQUE (user_id, video_id)
);
CREATE INDEX likes_video_created_idx ON likes (video_id, created_at DESC, id DESC);
CREATE INDEX likes_user_created_idx ON likes (user_id, created_at DESC, id DESC);

CREATE TABLE comments (
    id            UUID PRIMARY KEY,
    user_id       UUID NOT NULL,
    video_id      UUID NOT NULL,
    parent_id     UUID REFERENCES comments (id),
    content       VARCHAR(1000) NOT NULL,
    likes_count   INT NOT NULL DEFAULT 0,
    replies_count INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL,
    deleted_at    TIMESTAMPTZ,
    CONSTRAINT comments_not_own_parent_chk CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT comments_content_chk CHECK (char_length(content) BETWEEN 1 AND 1000),
    CONSTRAINT comments_likes_count_chk CHECK (likes_count >= 0),
    CONSTRAINT comments_replies_count_chk CHECK (replies_count >= 0)
);
CREATE INDEX comments_video_top_idx ON comments (video_id, created_at DESC, id DESC) WHERE parent_id IS NULL;
CREATE INDEX comments_parent_idx ON comments (parent_id, created_at, id) WHERE parent_id IS NOT NULL;
CREATE INDEX comments_user_idx ON comments (user_id);

CREATE TABLE comment_likes (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL,
    comment_id UUID NOT NULL REFERENCES comments (id),
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT comment_likes_user_comment_key UNIQUE (user_id, comment_id)
);
CREATE INDEX comment_likes_comment_idx ON comment_likes (comment_id);

CREATE TABLE follows (
    id           UUID PRIMARY KEY,
    follower_id  UUID NOT NULL,
    following_id UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT follows_pair_key UNIQUE (follower_id, following_id),
    CONSTRAINT follows_not_self_chk CHECK (follower_id <> following_id)
);
CREATE INDEX follows_following_created_idx ON follows (following_id, created_at DESC, id DESC);
CREATE INDEX follows_follower_created_idx ON follows (follower_id, created_at DESC, id DESC);

CREATE TABLE shares (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL,
    video_id   UUID NOT NULL,
    channel    VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT shares_channel_chk CHECK (channel IN ('whatsapp', 'copy_link', 'other'))
);
CREATE INDEX shares_video_idx ON shares (video_id);
CREATE INDEX shares_user_idx ON shares (user_id);

-- Counters change in the transaction of the action they count.
CREATE TABLE video_counters (
    video_id       UUID PRIMARY KEY,
    likes_count    BIGINT NOT NULL DEFAULT 0,
    comments_count BIGINT NOT NULL DEFAULT 0,
    shares_count   BIGINT NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT video_counters_non_negative_chk CHECK (likes_count >= 0 AND comments_count >= 0 AND shares_count >= 0)
);

CREATE TABLE user_counters (
    user_id         UUID PRIMARY KEY,
    followers_count BIGINT NOT NULL DEFAULT 0,
    following_count BIGINT NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_counters_non_negative_chk CHECK (followers_count >= 0 AND following_count >= 0)
);

-- Transactional outbox. Columns must match github.com/poro/shared-go/outbox.Schema.
CREATE TABLE outbox_events (
    id           UUID PRIMARY KEY,
    topic        VARCHAR(200) NOT NULL,
    event_key    VARCHAR(200) NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts     INT NOT NULL DEFAULT 0,
    last_error   TEXT
);
CREATE INDEX outbox_events_pending_idx ON outbox_events (created_at, id) WHERE published_at IS NULL;
CREATE INDEX outbox_events_published_idx ON outbox_events (published_at) WHERE published_at IS NOT NULL;

-- Consumer idempotency. Columns must match github.com/poro/shared-go/inbox.Schema.
CREATE TABLE processed_events (
    consumer     VARCHAR(100) NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX processed_events_processed_at_idx ON processed_events (processed_at);
