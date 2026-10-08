"""Moderation cases, reports, decisions, content snapshots, outbox and inbox.

Revision ID: 0001
Revises:
"""

from alembic import op

revision = "0001"
down_revision = None
branch_labels = None
depends_on = None

REASONS = "('spam', 'nudity', 'violence', 'harassment', 'hate', 'fraud', 'other')"


def _execute(script: str) -> None:
    """asyncpg runs one statement per call."""
    for statement in script.split(";"):
        if statement.strip():
            op.execute(statement)


def upgrade() -> None:
    _execute(f"""
CREATE TABLE cases (
    id               UUID PRIMARY KEY,
    target_type      VARCHAR(10) NOT NULL,
    target_id        UUID NOT NULL,
    owner_id         UUID,
    status           VARCHAR(10) NOT NULL DEFAULT 'open',
    priority         SMALLINT NOT NULL DEFAULT 0,
    reports_count    INT NOT NULL DEFAULT 0,
    auto_verdict     VARCHAR(10),
    auto_reason      VARCHAR(20),
    auto_matches     JSONB NOT NULL DEFAULT '[]',
    reason           VARCHAR(20),
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL,
    decided_at       TIMESTAMPTZ,
    CONSTRAINT cases_target_key UNIQUE (target_type, target_id),
    CONSTRAINT cases_target_type_chk CHECK (target_type IN ('video', 'comment', 'user')),
    CONSTRAINT cases_status_chk CHECK (status IN ('open', 'removed', 'dismissed', 'restored')),
    CONSTRAINT cases_priority_chk CHECK (priority IN (0, 1)),
    CONSTRAINT cases_reports_count_chk CHECK (reports_count >= 0),
    CONSTRAINT cases_auto_verdict_chk CHECK (auto_verdict IN ('allow', 'review', 'block')),
    CONSTRAINT cases_auto_reason_chk CHECK (auto_reason IN {REASONS}),
    CONSTRAINT cases_reason_chk CHECK (reason IN {REASONS})
);
CREATE INDEX cases_queue_idx ON cases (status, priority DESC, updated_at DESC, id DESC);
CREATE INDEX cases_owner_idx ON cases (owner_id);

CREATE TABLE reports (
    id          UUID PRIMARY KEY,
    case_id     UUID NOT NULL REFERENCES cases (id),
    reporter_id UUID NOT NULL,
    reason      VARCHAR(20) NOT NULL,
    comment     VARCHAR(500),
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT reports_case_reporter_key UNIQUE (case_id, reporter_id),
    CONSTRAINT reports_reason_chk CHECK (reason IN {REASONS})
);
CREATE INDEX reports_reporter_idx ON reports (reporter_id, created_at);

CREATE TABLE decisions (
    id           UUID PRIMARY KEY,
    case_id      UUID NOT NULL REFERENCES cases (id),
    action       VARCHAR(10) NOT NULL,
    reason       VARCHAR(20),
    note         VARCHAR(500),
    decided_by   VARCHAR(10) NOT NULL,
    moderator_id UUID,
    created_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT decisions_action_chk CHECK (action IN ('remove', 'dismiss', 'restore')),
    CONSTRAINT decisions_decided_by_chk CHECK (decided_by IN ('auto', 'moderator')),
    CONSTRAINT decisions_moderator_chk CHECK ((decided_by = 'moderator') = (moderator_id IS NOT NULL)),
    CONSTRAINT decisions_reason_chk CHECK (reason IN {REASONS})
);
CREATE INDEX decisions_case_idx ON decisions (case_id, created_at);

-- Latest known text of each target, from its source events.
CREATE TABLE content_snapshots (
    target_type VARCHAR(10) NOT NULL,
    target_id   UUID NOT NULL,
    owner_id    UUID NOT NULL,
    video_id    UUID,
    body        TEXT NOT NULL,
    source_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (target_type, target_id),
    CONSTRAINT content_snapshots_target_type_chk CHECK (target_type IN ('video', 'comment', 'user'))
);

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

CREATE TABLE processed_events (
    consumer     VARCHAR(100) NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX processed_events_processed_at_idx ON processed_events (processed_at);
""")


def downgrade() -> None:
    _execute("""
DROP TABLE IF EXISTS processed_events;
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS content_snapshots;
DROP TABLE IF EXISTS decisions;
DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS cases;
""")
