import json
from datetime import UTC, datetime

import pytest

from moderation.events.envelope import (
    InvalidEnvelopeError,
    decode_envelope,
    dlq_topic,
    new_envelope,
    parse_time,
)
from moderation.ids import parse_uuid, uuid7


def test_round_trip() -> None:
    at = datetime(2026, 10, 8, 12, 0, 0, 123000, tzinfo=UTC)
    env = new_envelope("poro.moderation.content.removed", "subject", {"a": 1}, at)
    assert env.source == "moderation"
    doc = env.to_json()
    assert doc["occurred_at"] == "2026-10-08T12:00:00.123000Z"
    assert decode_envelope(json.dumps(doc).encode()) == env
    assert dlq_topic("poro.video.ready") == "poro.video.ready.dlq"


def test_go_nanoseconds() -> None:
    doc = {
        "id": str(uuid7()),
        "type": "poro.video.ready",
        "version": 1,
        "source": "media-worker",
        "subject": "s",
        "occurred_at": "2026-10-08T12:00:00.123456789Z",
        "data": {},
    }
    assert decode_envelope(json.dumps(doc).encode()).occurred_at.microsecond == 123456
    assert parse_time("2026-10-08T12:00:00+02:00") == datetime(2026, 10, 8, 10, 0, tzinfo=UTC)
    assert parse_time("2026-10-08T12:00:00") is None
    assert parse_time("yesterday") is None
    assert parse_time(42) is None


VALID = {
    "id": str(uuid7()),
    "type": "poro.social.comment.created",
    "version": 1,
    "source": "social",
    "subject": "s",
    "occurred_at": "2026-10-08T12:00:00Z",
    "data": {},
}


@pytest.mark.parametrize(
    "raw",
    [
        None,
        b"{",
        b"42",
        json.dumps({**VALID, "id": "x"}).encode(),
        json.dumps({**VALID, "type": "user.created"}).encode(),
        json.dumps({**VALID, "type": "poro.a.b.c.d"}).encode(),
        json.dumps({**VALID, "version": 0}).encode(),
        json.dumps({**VALID, "version": True}).encode(),
        json.dumps({**VALID, "source": ""}).encode(),
        json.dumps({**VALID, "subject": ""}).encode(),
        json.dumps({**VALID, "occurred_at": "nope"}).encode(),
        json.dumps({**VALID, "data": []}).encode(),
    ],
)
def test_rejects(raw: bytes | None) -> None:
    with pytest.raises(InvalidEnvelopeError):
        decode_envelope(raw)


def test_new_envelope_checks_type() -> None:
    with pytest.raises(InvalidEnvelopeError):
        new_envelope("moderation.removed", "s", {})


def test_uuid7() -> None:
    first, second = uuid7(1_000), uuid7(2_000)
    assert first.version == 7
    assert str(first) < str(second)
    assert int(first.hex[:12], 16) == 1_000
    assert parse_uuid(str(first)) == first
    assert parse_uuid("nope") is None
    assert parse_uuid(7) is None
