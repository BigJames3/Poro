"""Transactional outbox (relayed by services/outbox-relay) and consumer inbox.
Both tables follow shared-go/outbox.Schema and shared-go/inbox.Schema."""

import json

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

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
