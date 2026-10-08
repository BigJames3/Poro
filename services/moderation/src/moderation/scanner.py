"""Consumer group poro-moderation-scanner: snapshots the text of videos,
comments and profiles and applies the rule engine to it, once per event."""

import logging
import uuid
from collections.abc import Callable
from datetime import datetime
from typing import Any

from prometheus_client import Counter
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from moderation.cases import Content, apply_verdict, save_content
from moderation.events.consumer import PermanentError
from moderation.events.envelope import (
    TYPE_COMMENT_CREATED,
    TYPE_COMMENT_UPDATED,
    TYPE_PROFILE_UPDATED,
    TYPE_VIDEO_READY,
    Envelope,
    parse_time,
)
from moderation.events.store import claim
from moderation.ids import parse_uuid
from moderation.rules.engine import RuleEngine

GROUP = "poro-moderation-scanner"
TOPICS = (TYPE_VIDEO_READY, TYPE_COMMENT_CREATED, TYPE_COMMENT_UPDATED, TYPE_PROFILE_UPDATED)
log = logging.getLogger(__name__)

VERDICTS = Counter(
    "moderation_scans_total",
    "Automatic scans by target type, verdict and outcome.",
    ["service", "target_type", "verdict", "outcome"],
)


def _uuid(data: dict[str, Any], field: str) -> uuid.UUID:
    value = parse_uuid(data.get(field))
    if value is None:
        raise PermanentError(f"{field} must be a UUID")
    return value


def _time(data: dict[str, Any], *fields: str) -> datetime:
    for field in fields:
        value = parse_time(data.get(field))
        if value is not None:
            return value
    raise PermanentError(f"{fields[0]} must be a date-time")


def _text(data: dict[str, Any], field: str) -> str:
    value = data.get(field)
    if value is None:
        return ""
    if not isinstance(value, str):
        raise PermanentError(f"{field} must be a string")
    return value


def _video(data: dict[str, Any]) -> Content:
    video = _uuid(data, "video_id")
    hashtags = data.get("hashtags") or []
    if not isinstance(hashtags, list) or not all(isinstance(h, str) for h in hashtags):
        raise PermanentError("hashtags must be a list of strings")
    body = "\n".join(
        part for part in (_text(data, "title"), _text(data, "description"), " ".join(hashtags)) if part
    )
    return Content(
        "video", video, _uuid(data, "user_id"), video, body, _time(data, "published_at", "ready_at")
    )


def _comment(data: dict[str, Any], time_field: str) -> Content:
    # Events published before text existed only carry the 140-character excerpt.
    body = _text(data, "text") or _text(data, "excerpt")
    return Content(
        "comment",
        _uuid(data, "comment_id"),
        _uuid(data, "user_id"),
        _uuid(data, "video_id"),
        body,
        _time(data, time_field),
    )


def _profile(data: dict[str, Any]) -> Content:
    user = _uuid(data, "user_id")
    body = "\n".join(p for p in (_text(data, "display_name"), _text(data, "username")) if p)
    return Content("user", user, user, None, body, _time(data, "updated_at"))


_DECODERS: dict[str, Callable[[dict[str, Any]], Content]] = {
    TYPE_VIDEO_READY: _video,
    TYPE_COMMENT_CREATED: lambda d: _comment(d, "created_at"),
    TYPE_COMMENT_UPDATED: lambda d: _comment(d, "updated_at"),
    TYPE_PROFILE_UPDATED: _profile,
}


class Scanner:
    def __init__(self, sessions: async_sessionmaker[AsyncSession], rules: RuleEngine) -> None:
        self._sessions = sessions
        self._rules = rules

    async def handle(self, env: Envelope) -> None:
        if env.version != 1:
            raise PermanentError(f"unsupported event {env.type} v{env.version}")
        decoder = _DECODERS.get(env.type)
        if decoder is None:
            raise PermanentError(f"unsupported event {env.type}")
        content = decoder(env.data)
        verdict = self._rules.check(content.body)
        async with self._sessions.begin() as session:
            if not await claim(session, GROUP, env.id):
                return
            if not await save_content(session, content):
                outcome = "stale"
            else:
                outcome = await apply_verdict(session, content, verdict)
        VERDICTS.labels("moderation", content.target_type, verdict.action.value, outcome).inc()
        if outcome == "removed":
            log.info(
                "content removed automatically",
                extra={
                    "target_type": content.target_type,
                    "target_id": str(content.target_id),
                    "reason": verdict.reason,
                },
            )
