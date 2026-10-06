-- Transactional outbox, relayed to Kafka by services/outbox-relay.
-- Columns must match github.com/poro/shared-go/outbox.Schema.
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

-- Consumer idempotency. Columns must match github.com/poro/shared-go/inbox.Schema.
CREATE TABLE IF NOT EXISTS processed_events (
    consumer     VARCHAR(100) NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX IF NOT EXISTS processed_events_processed_at_idx ON processed_events (processed_at);
