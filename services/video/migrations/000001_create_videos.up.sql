-- +migrate Up
CREATE TABLE videos (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    title VARCHAR(200) NOT NULL,
    description VARCHAR(1000) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL,
    content_type VARCHAR(50) NOT NULL,
    size_bytes BIGINT NOT NULL,
    source_key VARCHAR(500) NOT NULL,
    s3_upload_id VARCHAR(500),
    duration_ms INT,
    width INT,
    height INT,
    hls_key VARCHAR(500),
    thumbnail_key VARCHAR(500),
    renditions JSONB NOT NULL DEFAULT '[]'::jsonb,
    failure_code VARCHAR(50),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT videos_status_chk CHECK (status IN ('uploading', 'processing', 'ready', 'failed')),
    CONSTRAINT videos_size_chk CHECK (size_bytes > 0)
);

CREATE INDEX videos_user_created_idx ON videos (user_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX videos_status_idx ON videos (status) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS outbox_events (
    id           UUID PRIMARY KEY,
    topic        VARCHAR(200) NOT NULL,
    event_key    VARCHAR(200) NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts     INT NOT NULL DEFAULT 0,
    last_error   TEXT
);
CREATE INDEX IF NOT EXISTS outbox_events_pending_idx ON outbox_events (created_at, id) WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS outbox_events_published_idx ON outbox_events (published_at) WHERE published_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS processed_events (
    consumer     VARCHAR(100) NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX IF NOT EXISTS processed_events_processed_at_idx ON processed_events (processed_at);
