import asyncio
import json
from typing import Any

import pytest

from moderation.events.consumer import (
    HEADER_CONSUMER_GROUP,
    HEADER_ERROR,
    HEADER_ORIGINAL_OFFSET,
    PermanentError,
    Processor,
    Record,
    StoppedError,
)
from moderation.events.envelope import Envelope
from moderation.ids import uuid7

GROUP = "poro-moderation-scanner"


class Sink:
    def __init__(self, failures: int = 0) -> None:
        self.sent: list[tuple[str, bytes | None, bytes | None, dict[str, bytes]]] = []
        self.failures = failures

    async def send_and_wait(
        self, topic: str, *, value: bytes | None, key: bytes | None, headers: list[tuple[str, bytes]]
    ) -> Any:
        if self.failures > 0:
            self.failures -= 1
            raise ConnectionError("broker down")
        self.sent.append((topic, value, key, dict(headers)))


def record(value: bytes | None) -> Record:
    return Record("poro.video.ready", 2, 41, b"key", value)


def valid() -> bytes:
    return json.dumps(
        {
            "id": str(uuid7()),
            "type": "poro.video.ready",
            "version": 1,
            "source": "test",
            "subject": "s",
            "occurred_at": "2026-10-08T12:00:00Z",
            "data": {},
        }
    ).encode()


async def test_applies_and_retries_transient_failures() -> None:
    calls: list[Envelope] = []

    async def flaky(env: Envelope) -> None:
        calls.append(env)
        if len(calls) < 2:
            raise ConnectionError("db hiccup")

    sink = Sink()
    await Processor(GROUP, flaky, sink, backoff_s=0.001).process(record(valid()))
    assert len(calls) == 2
    assert sink.sent == []


async def test_dead_letters_permanent_failures_and_garbage() -> None:
    async def permanent(_: Envelope) -> None:
        raise PermanentError("video_id must be a UUID")

    sink = Sink(failures=1)
    processor = Processor(GROUP, permanent, sink, backoff_s=0.001)
    await processor.process(record(valid()))
    await processor.process(record(b"{not json"))
    assert [s[0] for s in sink.sent] == ["poro.video.ready.dlq", "poro.video.ready.dlq"]
    _, _, key, headers = sink.sent[0]
    assert key == b"key"
    assert headers[HEADER_ERROR] == b"video_id must be a UUID"
    assert headers[HEADER_ORIGINAL_OFFSET] == b"41"
    assert headers[HEADER_CONSUMER_GROUP] == GROUP.encode()
    assert sink.sent[1][3][HEADER_ERROR].startswith(b"invalid event envelope")


async def test_gives_up_after_max_attempts() -> None:
    attempts = 0

    async def broken(_: Envelope) -> None:
        nonlocal attempts
        attempts += 1
        raise RuntimeError("still broken")

    sink = Sink()
    await Processor(GROUP, broken, sink, max_attempts=3, backoff_s=0.001).process(record(valid()))
    assert attempts == 3
    assert len(sink.sent) == 1


async def test_stop_interrupts_without_committing() -> None:
    async def broken(_: Envelope) -> None:
        raise RuntimeError("down")

    processor = Processor(GROUP, broken, Sink(), backoff_s=10)
    task = asyncio.create_task(processor.process(record(valid())))
    await asyncio.sleep(0.01)
    processor.stop()
    with pytest.raises(StoppedError):
        await task
    stopped = Processor(GROUP, broken, Sink(failures=5), backoff_s=10)
    stopped.stop()
    with pytest.raises(StoppedError):
        await stopped.process(record(valid()))
