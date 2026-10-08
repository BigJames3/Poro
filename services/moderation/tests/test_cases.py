"""Case lifecycle against a real Postgres: automatic verdicts, reports, decisions."""

import uuid
from datetime import UTC, datetime, timedelta

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from moderation import cases
from moderation.cases import Content, DomainError
from moderation.ids import uuid7
from moderation.rules.engine import ALLOW, Action, Verdict
from tests.conftest import outbox

BLOCK = Verdict(Action.BLOCK, "hate", ["block:hate:x"])
REVIEW = Verdict(Action.REVIEW, "harassment", ["review:harassment:x"])


def content(target_type: str = "video", at: datetime | None = None, body: str = "texte") -> Content:
    target = uuid7()
    return Content(
        target_type,
        target,
        uuid7(),
        target if target_type == "video" else uuid7(),
        body,
        at or datetime.now(UTC),
    )


async def case_of(sessions: async_sessionmaker[AsyncSession], c: Content) -> dict[str, object] | None:
    async with sessions() as session:
        row = (
            (
                await session.execute(
                    text("SELECT * FROM cases WHERE target_type = :t AND target_id = :id"),
                    {"t": c.target_type, "id": c.target_id},
                )
            )
            .mappings()
            .first()
        )
        return None if row is None else dict(row)


async def scan(sessions: async_sessionmaker[AsyncSession], c: Content, verdict: Verdict) -> str:
    async with sessions.begin() as session:
        assert await cases.save_content(session, c)
        return await cases.apply_verdict(session, c, verdict)


async def test_block_removes_at_once_and_publishes(sessions: async_sessionmaker[AsyncSession]) -> None:
    c = content("comment")
    assert await scan(sessions, c, BLOCK) == "removed"
    case = await case_of(sessions, c)
    assert case is not None
    assert case["status"] == "removed"
    assert case["priority"] == 1
    assert case["auto_verdict"] == "block"
    events = await outbox(sessions, "poro.moderation.content.removed", str(c.target_id))
    assert len(events) == 1
    assert events[0]["data"] == {
        "case_id": str(case["id"]),
        "target_type": "comment",
        "target_id": str(c.target_id),
        "owner_id": str(c.owner_id),
        "reason": "hate",
        "decided_by": "auto",
        "removed_at": events[0]["data"]["removed_at"],
    }
    assert await scan(sessions, c, BLOCK) == "noted", "a removed case is not removed twice"
    assert len(await outbox(sessions, "poro.moderation.content.removed", str(c.target_id))) == 1


async def test_review_queues_and_allow_creates_nothing(sessions: async_sessionmaker[AsyncSession]) -> None:
    flagged, clean = content(), content()
    assert await scan(sessions, flagged, REVIEW) == "queued"
    assert await scan(sessions, clean, ALLOW) == "ignored"
    assert await case_of(sessions, clean) is None
    case = await case_of(sessions, flagged)
    assert case is not None
    assert (case["status"], case["priority"], case["auto_reason"]) == ("open", 1, "harassment")
    assert await scan(sessions, flagged, ALLOW) == "noted"
    case = await case_of(sessions, flagged)
    assert case is not None
    assert case["auto_verdict"] == "allow"


async def test_blocking_profiles_go_to_a_human(sessions: async_sessionmaker[AsyncSession]) -> None:
    c = content("user")
    assert await scan(sessions, c, BLOCK) == "queued"
    case = await case_of(sessions, c)
    assert case is not None
    assert case["status"] == "open"
    assert await outbox(sessions, "poro.moderation.content.removed", str(c.target_id)) == []


async def test_older_snapshots_are_ignored(sessions: async_sessionmaker[AsyncSession]) -> None:
    now = datetime.now(UTC)
    c = content(at=now)
    old = Content(c.target_type, c.target_id, c.owner_id, c.video_id, "ancien", now - timedelta(minutes=1))
    async with sessions.begin() as session:
        assert await cases.save_content(session, c)
        assert not await cases.save_content(session, old)


async def test_reports_dedupe_and_raise_priority(sessions: async_sessionmaker[AsyncSession]) -> None:
    c = content()
    async with sessions.begin() as session:
        await cases.save_content(session, c)
    reporters = [uuid7() for _ in range(3)]
    results = []
    for reporter in reporters:
        async with sessions.begin() as session:
            results.append(await cases.report(session, reporter, "video", c.target_id, "spam", None, 3))
    async with sessions.begin() as session:
        again = await cases.report(session, reporters[0], "video", c.target_id, "hate", "encore", 3)
    assert [r.created for r in results] == [True, True, True]
    assert again.created is False
    assert again.report_id == results[0].report_id
    case = await case_of(sessions, c)
    assert case is not None
    assert (case["reports_count"], case["priority"], case["owner_id"]) == (3, 1, c.owner_id)

    with pytest.raises(DomainError) as err:
        async with sessions.begin() as session:
            await cases.report(session, c.owner_id, "video", c.target_id, "spam", None, 3)
    assert err.value.code == "cannot_report_self"
    me = uuid7()
    with pytest.raises(DomainError):
        async with sessions.begin() as session:
            await cases.report(session, me, "user", me, "spam", None, 3)


async def test_report_before_the_content_arrives(sessions: async_sessionmaker[AsyncSession]) -> None:
    target = uuid7()
    async with sessions.begin() as session:
        result = await cases.report(session, uuid7(), "video", target, "nudity", None, 3)
    with pytest.raises(DomainError) as err:
        async with sessions.begin() as session:
            await cases.decide(session, result.case_id, uuid7(), "remove", "nudity", None)
    assert err.value.code == "target_unknown"
    later = Content("video", target, uuid7(), target, "titre", datetime.now(UTC))
    assert await scan(sessions, later, REVIEW) == "queued"
    async with sessions.begin() as session:
        view = await cases.decide(session, result.case_id, uuid7(), "remove", "nudity", None)
    assert view["status"] == "removed"
    assert view["owner_id"] == str(later.owner_id)


async def test_moderator_decisions(sessions: async_sessionmaker[AsyncSession]) -> None:
    video = content()
    await scan(sessions, video, REVIEW)
    case = await case_of(sessions, video)
    assert case is not None
    case_id = case["id"]
    assert isinstance(case_id, uuid.UUID)
    moderator = uuid7()

    async def decide(action: str, reason: str | None = None) -> dict[str, object]:
        async with sessions.begin() as session:
            return await cases.decide(session, case_id, moderator, action, reason, "vu")

    with pytest.raises(DomainError, match="reason"):
        await decide("remove")
    removed = await decide("remove", "violence")
    assert removed["status"] == "removed"
    with pytest.raises(DomainError) as err:
        await decide("dismiss")
    assert err.value.code == "invalid_transition"
    restored = await decide("restore")
    assert restored["status"] == "restored"
    assert [d["action"] for d in restored["decisions"]] == ["remove", "restore"]  # type: ignore[union-attr]
    assert restored["content"] == {
        "text": "texte",
        "video_id": str(video.target_id),
        "seen_at": restored["content"]["seen_at"],
    }  # type: ignore[index]
    restored_events = await outbox(sessions, "poro.moderation.content.restored", str(video.target_id))
    assert restored_events[0]["data"]["target_type"] == "video"
    assert await scan(sessions, video, BLOCK) == "noted", "a human restore wins over the filter"

    comment = content("comment")
    await scan(sessions, comment, BLOCK)
    comment_case = await case_of(sessions, comment)
    assert comment_case is not None
    with pytest.raises(DomainError) as err:
        async with sessions.begin() as session:
            await cases.decide(session, comment_case["id"], moderator, "restore", None, None)  # type: ignore[arg-type]
    assert err.value.code == "restore_not_supported"

    profile = content("user")
    await scan(sessions, profile, REVIEW)
    profile_case = await case_of(sessions, profile)
    assert profile_case is not None
    with pytest.raises(DomainError) as err:
        async with sessions.begin() as session:
            await cases.decide(session, profile_case["id"], moderator, "remove", "hate", None)  # type: ignore[arg-type]
    assert err.value.code == "action_not_allowed"
    async with sessions.begin() as session:
        dismissed = await cases.decide(session, profile_case["id"], moderator, "dismiss", None, None)  # type: ignore[arg-type]
    assert (dismissed["status"], dismissed["priority"]) == ("dismissed", "normal")
    assert await scan(sessions, profile, REVIEW) == "queued", "new flagged text reopens a dismissed case"

    with pytest.raises(DomainError) as err:
        async with sessions.begin() as session:
            await cases.decide(session, uuid7(), moderator, "dismiss", None, None)
    assert err.value.code == "case_not_found"


async def test_queue_pagination(sessions: async_sessionmaker[AsyncSession]) -> None:
    for _ in range(5):
        await scan(sessions, content("comment", body="x"), REVIEW)
    seen: list[str] = []
    cursor = None
    async with sessions() as session:
        while True:
            page = await cases.list_cases(session, "open", "comment", cases.HIGH, cursor, 2)
            seen.extend(item["id"] for item in page["items"])
            cursor = page["next_cursor"]
            if cursor is None:
                break
    assert len(seen) == len(set(seen)) >= 5
    with pytest.raises(DomainError):
        cases.decode_cursor("bad!")
    for raw in ("9|2026-01-01T00:00:00+00:00|x", "1|nope|x", f"1|2026-01-01T00:00:00|{uuid7()}"):
        import base64

        with pytest.raises(DomainError):
            cases.decode_cursor(base64.urlsafe_b64encode(raw.encode()).decode())
