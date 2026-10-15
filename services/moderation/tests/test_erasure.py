"""Account erasure (ADR-0018) against a real Postgres: what is pseudonymized,
emptied or kept, replays, retries, late events and invalid payloads."""

import asyncio
import uuid
from datetime import UTC, datetime
from typing import Any

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from moderation import cases, erasure
from moderation.config import DEFAULT_RULES
from moderation.erasure import GROUP, Eraser
from moderation.events.consumer import PermanentError
from moderation.events.envelope import TYPE_USER_DELETED, Envelope, new_envelope
from moderation.ids import uuid7
from moderation.rules.engine import RuleEngine
from moderation.scanner import Scanner


@pytest.fixture(scope="module")
def scanner(sessions: async_sessionmaker[AsyncSession]) -> Scanner:
    return Scanner(sessions, RuleEngine.from_file(DEFAULT_RULES))


@pytest.fixture(scope="module")
def eraser(sessions: async_sessionmaker[AsyncSession]) -> Eraser:
    return Eraser(sessions)


def now() -> str:
    return datetime.now(UTC).isoformat()


def deleted(user: uuid.UUID, **data: Any) -> Envelope:
    return new_envelope(TYPE_USER_DELETED, str(user), {"user_id": str(user), "deleted_at": now(), **data})


def comment_created(comment: uuid.UUID, author: uuid.UUID, body: str) -> Envelope:
    return new_envelope(
        "poro.social.comment.created",
        str(comment),
        {
            "comment_id": str(comment),
            "user_id": str(author),
            "video_id": str(uuid7()),
            "video_owner_id": str(uuid7()),
            "parent_id": None,
            "parent_author_id": None,
            "excerpt": body[:140],
            "text": body,
            "created_at": now(),
        },
    )


def profile_updated(user: uuid.UUID, name: str) -> Envelope:
    return new_envelope(
        "poro.user.profile.updated",
        str(user),
        {"user_id": str(user), "username": name, "display_name": name.title(), "updated_at": now()},
    )


async def rows(sessions: async_sessionmaker[AsyncSession], sql: str, **params: Any) -> list[dict[str, Any]]:
    async with sessions() as session:
        return [dict(r) for r in (await session.execute(text(sql), params)).mappings()]


class World:
    """An account with reports, content, cases and a moderator decision, next
    to another account that must stay untouched."""

    def __init__(self) -> None:
        self.user, self.other, self.witness = uuid7(), uuid7(), uuid7()
        self.user_comment, self.other_comment = uuid7(), uuid7()
        self.user_report: uuid.UUID
        self.other_report: uuid.UUID
        self.user_case: uuid.UUID
        self.account_case: uuid.UUID
        self.other_case: uuid.UUID

    async def build(self, sessions: async_sessionmaker[AsyncSession], scanner: Scanner) -> "World":
        # The account's own content: a blocking comment (case removed, auto
        # decision) and its profile.
        await scanner.handle(comment_created(self.user_comment, self.user, "Je vais te tuer"))
        await scanner.handle(profile_updated(self.user, "awa_" + self.user.hex[:6]))
        await scanner.handle(comment_created(self.other_comment, self.other, "Bravo pour la danse"))
        async with sessions.begin() as session:
            # The account reports the other's comment, and is reported back.
            mine = await cases.report(
                session,
                self.user,
                "comment",
                self.other_comment,
                "harassment",
                "appelez-moi au 0707070707",
                3,
            )
            theirs = await cases.report(
                session, self.witness, "comment", self.other_comment, "spam", "doublon", 3
            )
            account = await cases.report(session, self.other, "user", self.user, "spam", "faux compte", 3)
            self.user_report, self.other_report = mine.report_id, theirs.report_id
            self.other_case, self.account_case = mine.case_id, account.case_id
            # The account was also a moderator: its decision stays in the log.
            await cases.decide(session, account.case_id, self.user, "dismiss", None, "rien à signaler")
        case = await rows(sessions, "SELECT id FROM cases WHERE target_id = :id", id=self.user_comment)
        self.user_case = case[0]["id"]
        return self


async def test_erasure_forgets_the_account_and_keeps_the_log(
    sessions: async_sessionmaker[AsyncSession], scanner: Scanner, eraser: Eraser
) -> None:
    w = await World().build(sessions, scanner)
    decisions_before = await rows(
        sessions,
        "SELECT * FROM decisions WHERE case_id IN (:a, :b, :c)",
        a=w.user_case,
        b=w.account_case,
        c=w.other_case,
    )
    assert len(decisions_before) == 2

    env = deleted(w.user)
    await eraser.handle(env)

    # reports: the account's report is pseudonymized and loses its comment.
    assert await rows(sessions, "SELECT id FROM reports WHERE reporter_id = :u", u=w.user) == []
    [mine] = await rows(sessions, "SELECT * FROM reports WHERE id = :id", id=w.user_report)
    assert mine["reporter_id"] not in (w.user, w.other, w.witness)
    assert mine["comment"] is None
    assert mine["reason"] == "harassment"
    [theirs] = await rows(sessions, "SELECT * FROM reports WHERE id = :id", id=w.other_report)
    assert (theirs["reporter_id"], theirs["comment"]) == (w.witness, "doublon")
    [against] = await rows(sessions, "SELECT * FROM reports WHERE case_id = :c", c=w.account_case)
    assert (against["reporter_id"], against["comment"]) == (w.other, "faux compte")

    # content_snapshots: no text and no owner left for the account.
    assert await rows(sessions, "SELECT 1 FROM content_snapshots WHERE owner_id = :u", u=w.user) == []
    emptied = await rows(
        sessions,
        "SELECT body, owner_id FROM content_snapshots WHERE target_id IN (:c, :u)",
        c=w.user_comment,
        u=w.user,
    )
    assert emptied == [{"body": "", "owner_id": None}] * 2
    [kept] = await rows(sessions, "SELECT * FROM content_snapshots WHERE target_id = :c", c=w.other_comment)
    assert (kept["body"], kept["owner_id"]) == ("Bravo pour la danse", w.other)

    # cases: kept with opaque ids, without the account as owner.
    assert await rows(sessions, "SELECT 1 FROM cases WHERE owner_id = :u", u=w.user) == []
    [user_case] = await rows(sessions, "SELECT * FROM cases WHERE id = :id", id=w.user_case)
    assert (user_case["status"], user_case["owner_id"]) == ("removed", None)
    [account_case] = await rows(sessions, "SELECT * FROM cases WHERE id = :id", id=w.account_case)
    assert (account_case["target_id"], account_case["owner_id"]) == (w.user, None)
    [other_case] = await rows(sessions, "SELECT * FROM cases WHERE id = :id", id=w.other_case)
    assert other_case["owner_id"] == w.other
    assert other_case["reports_count"] == 2

    # decisions: the moderation log is kept as it was, erased moderator included.
    decisions_after = await rows(
        sessions,
        "SELECT * FROM decisions WHERE case_id IN (:a, :b, :c)",
        a=w.user_case,
        b=w.account_case,
        c=w.other_case,
    )
    assert decisions_after == decisions_before
    assert any(d["moderator_id"] == w.user for d in decisions_after)

    erased = await rows(sessions, "SELECT user_id FROM erased_users WHERE user_id = :u", u=w.user)
    assert erased == [{"user_id": w.user}], "only the opaque id is kept"

    # Replays and a second erasure event change nothing more.
    again = deleted(w.user)
    await eraser.handle(env)
    await eraser.handle(again)
    [replayed] = await rows(sessions, "SELECT * FROM reports WHERE id = :id", id=w.user_report)
    assert replayed["reporter_id"] == mine["reporter_id"], "the pseudonym is drawn once"
    claimed = await rows(
        sessions,
        "SELECT event_id FROM processed_events WHERE consumer = :g AND event_id IN (:a, :b)",
        g=GROUP,
        a=env.id,
        b=again.id,
    )
    assert len(claimed) == 2


async def test_late_events_store_nothing_about_an_erased_account(
    sessions: async_sessionmaker[AsyncSession], scanner: Scanner, eraser: Eraser
) -> None:
    w = await World().build(sessions, scanner)
    await eraser.handle(deleted(w.user))

    await scanner.handle(
        new_envelope(
            "poro.social.comment.updated",
            str(w.user_comment),
            {
                "comment_id": str(w.user_comment),
                "user_id": str(w.user),
                "video_id": str(uuid7()),
                "text": "nouveau texte, appelez le 0707070707",
                "updated_at": now(),
            },
        )
    )
    await scanner.handle(profile_updated(w.user, "awa_revient"))
    fresh = uuid7()
    await scanner.handle(comment_created(fresh, w.user, "espèce de connard"))

    assert (
        await rows(
            sessions,
            "SELECT body, owner_id FROM content_snapshots WHERE target_id IN (:c, :u)",
            c=w.user_comment,
            u=w.user,
        )
        == [{"body": "", "owner_id": None}] * 2
    )
    assert await rows(sessions, "SELECT 1 FROM content_snapshots WHERE target_id = :id", id=fresh) == []
    assert await rows(sessions, "SELECT 1 FROM cases WHERE target_id = :id", id=fresh) == []

    # A new report against the erased account never links it back as owner.
    async with sessions.begin() as session:
        result = await cases.report(session, uuid7(), "user", w.user, "spam", None, 3)
    [case] = await rows(
        sessions, "SELECT owner_id, reports_count FROM cases WHERE id = :id", id=result.case_id
    )
    assert case == {"owner_id": None, "reports_count": 2}


async def test_a_scan_racing_the_erasure_waits_for_it(
    sessions: async_sessionmaker[AsyncSession], scanner: Scanner
) -> None:
    user, comment = uuid7(), uuid7()
    async with sessions.begin() as session:
        await cases.erase_account(session, user)
        scan = asyncio.create_task(scanner.handle(comment_created(comment, user, "Je vais te tuer")))
        await asyncio.sleep(0.5)
        assert not scan.done(), "the scan waits for the erasure to commit"
    await scan
    assert await rows(sessions, "SELECT 1 FROM content_snapshots WHERE target_id = :id", id=comment) == []
    assert await rows(sessions, "SELECT 1 FROM cases WHERE target_id = :id", id=comment) == []


async def test_a_failed_erasure_is_rolled_back_then_retried(
    sessions: async_sessionmaker[AsyncSession],
    scanner: Scanner,
    eraser: Eraser,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    w = await World().build(sessions, scanner)
    real = cases.erase_account

    async def broken(session: AsyncSession, user: uuid.UUID, at: datetime | None = None) -> None:
        await real(session, user, at)
        raise ConnectionError("database went away")

    monkeypatch.setattr(erasure, "erase_account", broken)
    env = deleted(w.user)
    with pytest.raises(ConnectionError):
        await eraser.handle(env)
    assert await rows(sessions, "SELECT 1 FROM reports WHERE reporter_id = :u", u=w.user) != []
    assert await rows(sessions, "SELECT 1 FROM processed_events WHERE event_id = :id", id=env.id) == []

    monkeypatch.setattr(erasure, "erase_account", real)
    await eraser.handle(env)
    assert await rows(sessions, "SELECT 1 FROM reports WHERE reporter_id = :u", u=w.user) == []
    assert await rows(sessions, "SELECT 1 FROM content_snapshots WHERE owner_id = :u", u=w.user) == []


@pytest.mark.parametrize(
    "env",
    [
        new_envelope("poro.auth.user.created", "s", {"user_id": str(uuid7())}),
        new_envelope(TYPE_USER_DELETED, "s", {"user_id": "nope"}),
        new_envelope(TYPE_USER_DELETED, "s", {"user_id": str(uuid.UUID(int=0))}),
        new_envelope(TYPE_USER_DELETED, "s", {"user_id": 7}),
        new_envelope(TYPE_USER_DELETED, "s", {}),
    ],
)
async def test_bad_erasure_events_are_permanent(eraser: Eraser, env: Envelope) -> None:
    with pytest.raises(PermanentError):
        await eraser.handle(env)


async def test_unknown_erasure_versions_are_permanent(eraser: Eraser) -> None:
    env = deleted(uuid7())
    with pytest.raises(PermanentError):
        await eraser.handle(Envelope(env.id, env.type, 2, env.source, env.subject, env.occurred_at, env.data))
