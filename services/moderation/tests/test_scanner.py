from datetime import UTC, datetime
from typing import Any

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from moderation.config import DEFAULT_RULES
from moderation.events.consumer import PermanentError
from moderation.events.envelope import Envelope, new_envelope
from moderation.ids import uuid7
from moderation.rules.engine import RuleEngine
from moderation.scanner import Scanner
from tests.conftest import outbox


@pytest.fixture(scope="module")
def scanner(sessions: async_sessionmaker[AsyncSession]) -> Scanner:
    return Scanner(sessions, RuleEngine.from_file(DEFAULT_RULES))


def event(event_type: str, data: dict[str, Any]) -> Envelope:
    return new_envelope(event_type, "subject", data)


def now() -> str:
    return datetime.now(UTC).isoformat()


async def status(sessions: async_sessionmaker[AsyncSession], target_id: str) -> str | None:
    async with sessions() as session:
        value: str | None = (
            await session.execute(text("SELECT status FROM cases WHERE target_id = :id"), {"id": target_id})
        ).scalar_one_or_none()
        return value


async def test_comment_with_slur_is_removed_once(
    scanner: Scanner, sessions: async_sessionmaker[AsyncSession]
) -> None:
    comment = str(uuid7())
    env = event(
        "poro.social.comment.created",
        {
            "comment_id": comment,
            "user_id": str(uuid7()),
            "video_id": str(uuid7()),
            "video_owner_id": str(uuid7()),
            "parent_id": None,
            "parent_author_id": None,
            "excerpt": "Salut",
            "text": "Salut sale nègre",
            "created_at": now(),
        },
    )
    await scanner.handle(env)
    await scanner.handle(env)
    assert await status(sessions, comment) == "removed"
    assert len(await outbox(sessions, "poro.moderation.content.removed", comment)) == 1


async def test_edits_are_scanned(scanner: Scanner, sessions: async_sessionmaker[AsyncSession]) -> None:
    comment, author, video = str(uuid7()), str(uuid7()), str(uuid7())
    base = {"comment_id": comment, "user_id": author, "video_id": video}
    await scanner.handle(
        event(
            "poro.social.comment.created",
            {
                **base,
                "video_owner_id": str(uuid7()),
                "parent_id": None,
                "parent_author_id": None,
                "excerpt": "Bravo",
                "created_at": now(),
            },
        )
    )
    assert await status(sessions, comment) is None
    await scanner.handle(
        event("poro.social.comment.updated", {**base, "text": "espèce de connard", "updated_at": now()})
    )
    assert await status(sessions, comment) == "open"


async def test_videos_and_profiles(scanner: Scanner, sessions: async_sessionmaker[AsyncSession]) -> None:
    video, user = str(uuid7()), str(uuid7())
    await scanner.handle(
        event(
            "poro.video.ready",
            {
                "video_id": video,
                "user_id": user,
                "ready_at": now(),
                "title": "Vidéo porno gratuit",
                "description": None,
                "hashtags": ["abidjan"],
            },
        )
    )
    assert await status(sessions, video) == "removed"
    await scanner.handle(
        event(
            "poro.user.profile.updated",
            {
                "user_id": user,
                "username": "awa",
                "display_name": "Je vais te tuer",
                "avatar_url": None,
                "is_creator": False,
                "updated_at": now(),
            },
        )
    )
    assert await status(sessions, user) == "open", "accounts are never removed automatically"


@pytest.mark.parametrize(
    ("event_type", "data", "message"),
    [
        (
            "poro.video.ready",
            {"video_id": "x", "user_id": str(uuid7()), "ready_at": "2026-10-08T00:00:00Z"},
            "video_id",
        ),
        ("poro.video.ready", {"video_id": str(uuid7()), "user_id": str(uuid7())}, "published_at"),
        (
            "poro.video.ready",
            {
                "video_id": str(uuid7()),
                "user_id": str(uuid7()),
                "ready_at": "2026-10-08T00:00:00Z",
                "hashtags": "x",
            },
            "hashtags",
        ),
        (
            "poro.social.comment.updated",
            {
                "comment_id": str(uuid7()),
                "user_id": str(uuid7()),
                "video_id": str(uuid7()),
                "text": 42,
                "updated_at": "2026-10-08T00:00:00Z",
            },
            "text",
        ),
        ("poro.social.like.created", {}, "unsupported event"),
    ],
)
async def test_bad_events_are_permanent(
    scanner: Scanner, event_type: str, data: dict[str, Any], message: str
) -> None:
    with pytest.raises(PermanentError, match=message):
        await scanner.handle(event(event_type, data))


async def test_unknown_versions_are_permanent(scanner: Scanner) -> None:
    env = event("poro.video.ready", {})
    with pytest.raises(PermanentError, match="v2"):
        await scanner.handle(
            Envelope(env.id, env.type, 2, env.source, env.subject, env.occurred_at, env.data)
        )
