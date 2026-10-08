"""Moderation cases: automatic verdicts, user reports and moderator decisions.
Every write runs in the caller's transaction, with the outbox event it needs."""

import base64
import binascii
import json
import uuid
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from moderation.events.envelope import TYPE_CONTENT_REMOVED, TYPE_CONTENT_RESTORED, new_envelope
from moderation.events.store import enqueue
from moderation.ids import parse_uuid, uuid7
from moderation.rules.engine import Action, Verdict

TARGET_TYPES = ("video", "comment", "user")
REMOVABLE = ("video", "comment")
NORMAL, HIGH = 0, 1
PRIORITIES = {NORMAL: "normal", HIGH: "high"}


class DomainError(Exception):
    """An error whose code and message are safe to show to clients."""

    def __init__(self, status: int, code: str, message: str) -> None:
        super().__init__(message)
        self.status = status
        self.code = code
        self.message = message


@dataclass(frozen=True)
class Content:
    """The latest text of a target, as its source event described it."""

    target_type: str
    target_id: uuid.UUID
    owner_id: uuid.UUID
    video_id: uuid.UUID | None
    body: str
    source_at: datetime


@dataclass(frozen=True)
class ReportResult:
    report_id: uuid.UUID
    case_id: uuid.UUID
    created: bool


def _now() -> datetime:
    return datetime.now(UTC)


async def _lock_case(session: AsyncSession, target_type: str, target_id: uuid.UUID) -> dict[str, Any] | None:
    row = (
        (
            await session.execute(
                text("SELECT * FROM cases WHERE target_type = :t AND target_id = :id FOR UPDATE"),
                {"t": target_type, "id": target_id},
            )
        )
        .mappings()
        .first()
    )
    return dict(row) if row else None


async def _get_or_create_case(
    session: AsyncSession, target_type: str, target_id: uuid.UUID, owner_id: uuid.UUID | None, now: datetime
) -> dict[str, Any]:
    await session.execute(
        text(
            "INSERT INTO cases (id, target_type, target_id, owner_id, created_at, updated_at) "
            "VALUES (:id, :t, :target, :owner, :now, :now) ON CONFLICT (target_type, target_id) DO NOTHING"
        ),
        {"id": uuid7(), "t": target_type, "target": target_id, "owner": owner_id, "now": now},
    )
    case = await _lock_case(session, target_type, target_id)
    if case is None:
        raise RuntimeError("case vanished after upsert")
    if case["owner_id"] is None and owner_id is not None:
        await session.execute(
            text("UPDATE cases SET owner_id = :owner WHERE id = :id"), {"owner": owner_id, "id": case["id"]}
        )
        case["owner_id"] = owner_id
    return case


async def _record_decision(
    session: AsyncSession,
    case_id: uuid.UUID,
    action: str,
    reason: str | None,
    note: str | None,
    moderator_id: uuid.UUID | None,
    now: datetime,
) -> None:
    await session.execute(
        text(
            "INSERT INTO decisions (id, case_id, action, reason, note, decided_by, moderator_id, created_at) "
            "VALUES (:id, :case, :action, :reason, :note, :by, :moderator, :now)"
        ),
        {
            "id": uuid7(),
            "case": case_id,
            "action": action,
            "reason": reason,
            "note": note,
            "by": "auto" if moderator_id is None else "moderator",
            "moderator": moderator_id,
            "now": now,
        },
    )


async def _emit_removed(
    session: AsyncSession, case: dict[str, Any], reason: str, decided_by: str, now: datetime
) -> None:
    target = str(case["target_id"])
    await enqueue(
        session,
        new_envelope(
            TYPE_CONTENT_REMOVED,
            target,
            {
                "case_id": str(case["id"]),
                "target_type": case["target_type"],
                "target_id": target,
                "owner_id": str(case["owner_id"]),
                "reason": reason,
                "decided_by": decided_by,
                "removed_at": now.isoformat().replace("+00:00", "Z"),
            },
            now,
        ),
    )


async def save_content(session: AsyncSession, content: Content) -> bool:
    """Store the snapshot; False when a newer one is already stored."""
    row = (
        await session.execute(
            text(
                "INSERT INTO content_snapshots (target_type, target_id, owner_id, video_id, body, source_at) "
                "VALUES (:t, :id, :owner, :video, :body, :at) "
                "ON CONFLICT (target_type, target_id) DO UPDATE SET owner_id = EXCLUDED.owner_id, "
                "video_id = EXCLUDED.video_id, body = EXCLUDED.body, source_at = EXCLUDED.source_at "
                "WHERE content_snapshots.source_at <= EXCLUDED.source_at RETURNING 1"
            ),
            {
                "t": content.target_type,
                "id": content.target_id,
                "owner": content.owner_id,
                "video": content.video_id,
                "body": content.body,
                "at": content.source_at,
            },
        )
    ).first()
    return row is not None


async def apply_verdict(
    session: AsyncSession, content: Content, verdict: Verdict, now: datetime | None = None
) -> str:
    """Act on an automatic verdict. Returns what happened, for metrics:
    removed, queued, noted or ignored."""
    now = now or _now()
    action = verdict.action
    # Accounts cannot be removed here: a blocking profile goes to a human.
    if action is Action.BLOCK and content.target_type not in REMOVABLE:
        action = Action.REVIEW
    if action is Action.ALLOW and await _lock_case(session, content.target_type, content.target_id) is None:
        return "ignored"
    # Also fills in the owner of a case opened by a report before the content arrived.
    case = await _get_or_create_case(session, content.target_type, content.target_id, content.owner_id, now)

    fields = {
        "id": case["id"],
        "verdict": verdict.action.value,
        "reason": verdict.reason,
        "matches": json.dumps(verdict.matches),
        "now": now,
    }
    note = (
        "UPDATE cases SET auto_verdict = :verdict, auto_reason = :reason, "
        "auto_matches = CAST(:matches AS jsonb)"
    )
    status = case["status"]
    if action is Action.BLOCK and status in ("open", "dismissed"):
        await session.execute(
            text(
                note + ", status = 'removed', priority = 1, reason = :reason, decided_at = :now, "
                "updated_at = :now WHERE id = :id"
            ),
            fields,
        )
        await _record_decision(
            session, case["id"], "remove", verdict.reason, ", ".join(verdict.matches)[:500], None, now
        )
        await _emit_removed(session, case, verdict.reason or "other", "auto", now)
        return "removed"
    if action is not Action.ALLOW and status in ("open", "dismissed"):
        await session.execute(
            text(note + ", status = 'open', priority = 1, updated_at = :now WHERE id = :id"), fields
        )
        return "queued"
    if action is not Action.ALLOW:
        # Removed or restored by a human already: keep the decision, flag it.
        await session.execute(text(note + ", priority = 1, updated_at = :now WHERE id = :id"), fields)
        return "noted"
    await session.execute(text(note + " WHERE id = :id"), fields)
    return "noted"


async def report(
    session: AsyncSession,
    reporter_id: uuid.UUID,
    target_type: str,
    target_id: uuid.UUID,
    reason: str,
    comment: str | None,
    threshold: int,
    now: datetime | None = None,
) -> ReportResult:
    """File a report; a second report of the same target by the same person
    returns the first one. Threshold distinct reporters raise the priority."""
    now = now or _now()
    owner = target_id if target_type == "user" else await _owner_of(session, target_type, target_id)
    if owner == reporter_id:
        raise DomainError(422, "cannot_report_self", "you cannot report your own content")
    case = await _get_or_create_case(session, target_type, target_id, owner, now)
    report_id = uuid7()
    inserted = (
        await session.execute(
            text(
                "INSERT INTO reports (id, case_id, reporter_id, reason, comment, created_at) "
                "VALUES (:id, :case, :reporter, :reason, :comment, :now) "
                "ON CONFLICT (case_id, reporter_id) DO NOTHING RETURNING id"
            ),
            {
                "id": report_id,
                "case": case["id"],
                "reporter": reporter_id,
                "reason": reason,
                "comment": comment,
                "now": now,
            },
        )
    ).first()
    if inserted is None:
        existing = (
            await session.execute(
                text("SELECT id FROM reports WHERE case_id = :case AND reporter_id = :reporter"),
                {"case": case["id"], "reporter": reporter_id},
            )
        ).scalar_one()
        return ReportResult(existing, case["id"], False)
    await session.execute(
        text(
            "UPDATE cases SET reports_count = reports_count + 1, updated_at = :now, "
            "priority = CASE WHEN reports_count + 1 >= :threshold THEN 1 ELSE priority END WHERE id = :id"
        ),
        {"id": case["id"], "now": now, "threshold": threshold},
    )
    return ReportResult(report_id, case["id"], True)


async def _owner_of(session: AsyncSession, target_type: str, target_id: uuid.UUID) -> uuid.UUID | None:
    owner: uuid.UUID | None = (
        await session.execute(
            text("SELECT owner_id FROM content_snapshots WHERE target_type = :t AND target_id = :id"),
            {"t": target_type, "id": target_id},
        )
    ).scalar_one_or_none()
    return owner


async def decide(
    session: AsyncSession,
    case_id: uuid.UUID,
    moderator_id: uuid.UUID,
    action: str,
    reason: str | None,
    note: str | None,
    now: datetime | None = None,
) -> dict[str, Any]:
    """Apply a moderator decision and return the case after it."""
    now = now or _now()
    row = (
        (await session.execute(text("SELECT * FROM cases WHERE id = :id FOR UPDATE"), {"id": case_id}))
        .mappings()
        .first()
    )
    if row is None:
        raise DomainError(404, "case_not_found", "case not found")
    case = dict(row)
    status, target_type = case["status"], case["target_type"]
    if action in ("remove", "restore") and target_type not in REMOVABLE:
        raise DomainError(422, "action_not_allowed", "accounts can only be dismissed here")
    if action == "restore" and target_type != "video":
        raise DomainError(422, "restore_not_supported", "removed comments cannot be restored")
    allowed = {"remove": ("open", "dismissed", "restored"), "dismiss": ("open",), "restore": ("removed",)}
    if status not in allowed[action]:
        raise DomainError(409, "invalid_transition", f"cannot {action} a {status} case")
    if action == "remove":
        if reason is None:
            raise DomainError(422, "reason_required", "a removal needs a reason")
        if case["owner_id"] is None:
            raise DomainError(409, "target_unknown", "the content has not reached moderation yet")
    new_status = {"remove": "removed", "dismiss": "dismissed", "restore": "restored"}[action]
    await session.execute(
        text(
            "UPDATE cases SET status = :status, reason = COALESCE(:reason, reason), decided_at = :now, "
            "updated_at = :now, priority = :priority WHERE id = :id"
        ),
        {
            "status": new_status,
            "reason": reason,
            "now": now,
            "id": case_id,
            # A dismissed case leaves the urgent queue.
            "priority": NORMAL if action == "dismiss" else case["priority"],
        },
    )
    await _record_decision(session, case_id, action, reason, note, moderator_id, now)
    if action == "remove":
        await _emit_removed(session, case, reason or "other", "moderator", now)
    elif action == "restore":
        target = str(case["target_id"])
        await enqueue(
            session,
            new_envelope(
                TYPE_CONTENT_RESTORED,
                target,
                {
                    "case_id": str(case_id),
                    "target_type": target_type,
                    "target_id": target,
                    "owner_id": str(case["owner_id"]),
                    "restored_at": now.isoformat().replace("+00:00", "Z"),
                },
                now,
            ),
        )
    return await get_case(session, case_id)


def case_view(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": str(row["id"]),
        "target_type": row["target_type"],
        "target_id": str(row["target_id"]),
        "owner_id": None if row["owner_id"] is None else str(row["owner_id"]),
        "status": row["status"],
        "priority": PRIORITIES[row["priority"]],
        "reports_count": row["reports_count"],
        "auto_verdict": row["auto_verdict"],
        "auto_reason": row["auto_reason"],
        "auto_matches": row["auto_matches"],
        "reason": row["reason"],
        "created_at": row["created_at"].isoformat(),
        "updated_at": row["updated_at"].isoformat(),
        "decided_at": None if row["decided_at"] is None else row["decided_at"].isoformat(),
    }


async def get_case(session: AsyncSession, case_id: uuid.UUID) -> dict[str, Any]:
    row = (
        (await session.execute(text("SELECT * FROM cases WHERE id = :id"), {"id": case_id}))
        .mappings()
        .first()
    )
    if row is None:
        raise DomainError(404, "case_not_found", "case not found")
    view = case_view(dict(row))
    snapshot = (
        (
            await session.execute(
                text(
                    "SELECT body, video_id, source_at FROM content_snapshots "
                    "WHERE target_type = :t AND target_id = :id"
                ),
                {"t": row["target_type"], "id": row["target_id"]},
            )
        )
        .mappings()
        .first()
    )
    view["content"] = (
        None
        if snapshot is None
        else {
            "text": snapshot["body"],
            "video_id": None if snapshot["video_id"] is None else str(snapshot["video_id"]),
            "seen_at": snapshot["source_at"].isoformat(),
        }
    )
    reports = (
        await session.execute(
            text(
                "SELECT id, reporter_id, reason, comment, created_at FROM reports WHERE case_id = :id "
                "ORDER BY created_at DESC, id DESC LIMIT 50"
            ),
            {"id": case_id},
        )
    ).mappings()
    view["reports"] = [
        {
            "id": str(r["id"]),
            "reporter_id": str(r["reporter_id"]),
            "reason": r["reason"],
            "comment": r["comment"],
            "created_at": r["created_at"].isoformat(),
        }
        for r in reports
    ]
    decisions = (
        await session.execute(
            text(
                "SELECT action, reason, note, decided_by, moderator_id, created_at FROM decisions "
                "WHERE case_id = :id ORDER BY created_at, id"
            ),
            {"id": case_id},
        )
    ).mappings()
    view["decisions"] = [
        {
            "action": d["action"],
            "reason": d["reason"],
            "note": d["note"],
            "decided_by": d["decided_by"],
            "moderator_id": None if d["moderator_id"] is None else str(d["moderator_id"]),
            "created_at": d["created_at"].isoformat(),
        }
        for d in decisions
    ]
    return view


def encode_cursor(row: dict[str, Any]) -> str:
    raw = f"{row['priority']}|{row['updated_at'].isoformat()}|{row['id']}"
    return base64.urlsafe_b64encode(raw.encode()).decode().rstrip("=")


def decode_cursor(raw: str) -> tuple[int, datetime, uuid.UUID]:
    invalid = DomainError(400, "invalid_cursor", "invalid cursor")
    try:
        decoded = base64.urlsafe_b64decode(raw + "=" * (-len(raw) % 4)).decode()
    except (binascii.Error, UnicodeDecodeError, ValueError) as err:
        raise invalid from err
    parts = decoded.split("|")
    if len(parts) != 3 or parts[0] not in ("0", "1"):
        raise invalid
    try:
        at = datetime.fromisoformat(parts[1])
    except ValueError as err:
        raise invalid from err
    case_id = parse_uuid(parts[2])
    if case_id is None or at.tzinfo is None:
        raise invalid
    return int(parts[0]), at, case_id


async def list_cases(
    session: AsyncSession,
    status: str,
    target_type: str | None,
    priority: int | None,
    cursor: str | None,
    limit: int,
) -> dict[str, Any]:
    """High priority first, then the most recent activity."""
    where = ["status = :status"]
    params: dict[str, Any] = {"status": status, "limit": limit + 1}
    if target_type is not None:
        where.append("target_type = :target_type")
        params["target_type"] = target_type
    if priority is not None:
        where.append("priority = :priority")
        params["priority"] = priority
    if cursor is not None:
        params["c_priority"], params["c_at"], params["c_id"] = decode_cursor(cursor)
        where.append("(priority, updated_at, id) < (:c_priority, :c_at, :c_id)")
    sql = (
        "SELECT * FROM cases WHERE "  # noqa: S608 -- the clauses are fixed strings; values are bound.
        + " AND ".join(where)
        + " ORDER BY priority DESC, updated_at DESC, id DESC LIMIT :limit"
    )
    rows = [dict(r) for r in (await session.execute(text(sql), params)).mappings()]
    page = rows[:limit]
    return {
        "items": [case_view(r) for r in page],
        "next_cursor": encode_cursor(page[-1]) if len(rows) > limit and page else None,
    }
