import asyncio
import contextlib
from datetime import UTC, datetime

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from moderation.events.store import INBOX_RETENTION, claim, purge_processed, run_inbox_purge
from moderation.ids import uuid7


async def test_purge_forgets_rows_older_than_the_retention(
    sessions: async_sessionmaker[AsyncSession],
) -> None:
    old, recent = uuid7(), uuid7()
    async with sessions.begin() as session:
        assert await claim(session, "poro-test", old)
        assert await claim(session, "poro-test", recent)
        await session.execute(
            text(
                "UPDATE processed_events SET processed_at = now() - interval '31 days' WHERE event_id = :id"
            ),
            {"id": old},
        )

    assert await purge_processed(sessions, datetime.now(UTC) - INBOX_RETENTION) == 1
    async with sessions.begin() as session:
        assert not await claim(session, "poro-test", recent), "a recent event is still deduplicated"
        assert await claim(session, "poro-test", old), "an event older than the retention is forgotten"


async def test_purge_loop_stops_when_cancelled(sessions: async_sessionmaker[AsyncSession]) -> None:
    task = asyncio.create_task(run_inbox_purge(sessions, interval_s=0.01))
    await asyncio.sleep(0.05)
    task.cancel()
    with contextlib.suppress(asyncio.CancelledError):
        await task
    assert task.cancelled()
