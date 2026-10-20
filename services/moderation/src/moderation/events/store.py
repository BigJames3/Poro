"""Transactional outbox (relayed by services/outbox-relay) and consumer inbox.
Both tables follow shared-go/outbox.Schema and shared-go/inbox.Schema."""

import asyncio
import json
import logging
from datetime import UTC, datetime, timedelta

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from moderation.events.envelope import Envelope


async def enqueue(session: AsyncSession, env: Envelope) -> None:
    """Store env for publication, in the caller's transaction."""
    await session.execute(
        text(
            "INSERT INTO outbox_events (id, topic, event_key, payload, created_at) "
            "VALUES (:id, :topic, :key, CAST(:payload AS jsonb), :at)"
        ),
        {
            "id": env.id,
            "topic": env.type,
            "key": env.subject,
            "payload": json.dumps(env.to_json(), ensure_ascii=False),
            "at": env.occurred_at,
        },
    )


async def claim(session: AsyncSession, consumer: str, event_id: object) -> bool:
    """Record event_id for consumer; False when it was already applied."""
    result = await session.execute(
        text(
            "INSERT INTO processed_events (consumer, event_id) VALUES (:consumer, :id) "
            "ON CONFLICT DO NOTHING RETURNING 1"
        ),
        {"consumer": consumer, "id": event_id},
    )
    return result.first() is not None


# Kafka keeps events 7 days: an older inbox row can no longer stop a duplicate.
INBOX_RETENTION = timedelta(days=30)
_PURGE_BATCH = 5000

log = logging.getLogger(__name__)


async def purge_processed(sessions: async_sessionmaker[AsyncSession], before: datetime) -> int:
    """Delete the inbox rows processed before `before`, in batches (PORO-019)."""
    total = 0
    while True:
        async with sessions.begin() as session:
            result = await session.execute(
                text(
                    "DELETE FROM processed_events WHERE ctid IN ("
                    "SELECT ctid FROM processed_events WHERE processed_at < :before LIMIT :batch)"
                ),
                {"before": before, "batch": _PURGE_BATCH},
            )
        deleted = int(getattr(result, "rowcount", 0) or 0)
        total += deleted
        if deleted < _PURGE_BATCH:
            return total


async def run_inbox_purge(sessions: async_sessionmaker[AsyncSession], interval_s: float = 3600) -> None:
    """Purge once per interval, starting one interval from now, until cancelled."""
    while True:
        await asyncio.sleep(interval_s)
        try:
            deleted = await purge_processed(sessions, datetime.now(UTC) - INBOX_RETENTION)
            if deleted:
                log.info("processed events purged", extra={"count": deleted})
        except Exception as err:  # noqa: BLE001 -- logged; the next pass tries again.
            log.error("processed events purge failed", extra={"error": str(err)})
