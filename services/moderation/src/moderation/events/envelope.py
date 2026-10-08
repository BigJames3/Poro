"""Event envelope shared with github.com/poro/shared-go/events."""

import json
import re
import uuid
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

from moderation.ids import parse_uuid, uuid7

SOURCE = "moderation"
# Same rule as shared-go/events: poro.{domain}.{action} or poro.{domain}.{entity}.{action}.
_TYPE = re.compile(r"^poro(\.[a-z][a-z0-9_]*){2,3}$")

TYPE_VIDEO_READY = "poro.video.ready"
TYPE_COMMENT_CREATED = "poro.social.comment.created"
TYPE_COMMENT_UPDATED = "poro.social.comment.updated"
TYPE_PROFILE_UPDATED = "poro.user.profile.updated"
TYPE_CONTENT_REMOVED = "poro.moderation.content.removed"
TYPE_CONTENT_RESTORED = "poro.moderation.content.restored"


class InvalidEnvelopeError(ValueError):
    def __init__(self, reason: str) -> None:
        super().__init__(f"invalid event envelope: {reason}")


@dataclass(frozen=True)
class Envelope:
    id: uuid.UUID
    type: str
    version: int
    source: str
    subject: str
    occurred_at: datetime
    data: dict[str, Any]

    def to_json(self) -> dict[str, Any]:
        return {
            "id": str(self.id),
            "type": self.type,
            "version": self.version,
            "source": self.source,
            "subject": self.subject,
            "occurred_at": self.occurred_at.isoformat().replace("+00:00", "Z"),
            "data": self.data,
        }


def new_envelope(event_type: str, subject: str, data: dict[str, Any], at: datetime | None = None) -> Envelope:
    occurred = (at or datetime.now(UTC)).astimezone(UTC)
    if not _TYPE.match(event_type):
        raise InvalidEnvelopeError("type must match poro.{domain}[.{entity}].{action}")
    return Envelope(uuid7(), event_type, 1, SOURCE, subject, occurred, data)


def parse_time(value: object) -> datetime | None:
    """Parse an RFC 3339 timestamp, including Go's nanosecond precision."""
    if not isinstance(value, str):
        return None
    text = value.strip()
    if text.endswith(("Z", "z")):
        text = text[:-1] + "+00:00"
    # Python keeps microseconds: drop the extra digits Go writes.
    text = re.sub(r"(\.\d{6})\d+", r"\1", text)
    try:
        parsed = datetime.fromisoformat(text)
    except ValueError:
        return None
    return parsed.astimezone(UTC) if parsed.tzinfo else None


def decode_envelope(raw: bytes | None) -> Envelope:
    if raw is None:
        raise InvalidEnvelopeError("empty message")
    try:
        doc = json.loads(raw)
    except (ValueError, UnicodeDecodeError) as err:
        raise InvalidEnvelopeError(str(err)) from err
    if not isinstance(doc, dict):
        raise InvalidEnvelopeError("not a JSON object")
    event_id = parse_uuid(doc.get("id"))
    if event_id is None:
        raise InvalidEnvelopeError("missing id")
    event_type = doc.get("type")
    if not isinstance(event_type, str) or not _TYPE.match(event_type):
        raise InvalidEnvelopeError("type must match poro.{domain}[.{entity}].{action}")
    version = doc.get("version")
    if not isinstance(version, int) or isinstance(version, bool) or version < 1:
        raise InvalidEnvelopeError("version must be at least 1")
    source, subject = doc.get("source"), doc.get("subject")
    if not isinstance(source, str) or not source:
        raise InvalidEnvelopeError("missing source")
    if not isinstance(subject, str) or not subject:
        raise InvalidEnvelopeError("missing subject")
    occurred = parse_time(doc.get("occurred_at"))
    if occurred is None:
        raise InvalidEnvelopeError("missing occurred_at")
    data = doc.get("data")
    if not isinstance(data, dict):
        raise InvalidEnvelopeError("data must be a JSON object")
    return Envelope(event_id, event_type, version, source, subject, occurred, data)


def dlq_topic(topic: str) -> str:
    return f"{topic}.dlq"
