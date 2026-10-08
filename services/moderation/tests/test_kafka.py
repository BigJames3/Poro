"""End to end over a real broker: events from social reach the scanner, which
removes a blocking comment, and poison messages land in the DLQ."""

import asyncio
import dataclasses
import json
from collections.abc import AsyncIterator, Awaitable, Callable, Iterator
from datetime import UTC, datetime

import httpx
import pytest
from aiokafka import AIOKafkaConsumer, AIOKafkaProducer
from aiokafka.admin import AIOKafkaAdminClient, NewTopic
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker
from testcontainers.community.kafka import RedpandaContainer

from moderation.app import create_app
from moderation.config import Config
from moderation.events.consumer import HEADER_CONSUMER_GROUP, HEADER_ERROR
from moderation.ids import uuid7
from moderation.scanner import TOPICS
from tests.conftest import Keys


@pytest.fixture(scope="module")
def broker() -> Iterator[str]:
    redpanda = RedpandaContainer("redpandadata/redpanda:v24.2.7").start(timeout=120)
    try:
        yield redpanda.get_bootstrap_server().replace("localhost", "127.0.0.1")
    finally:
        redpanda.stop()


@pytest.fixture(scope="module")
async def running(broker: str, config: Config, keys: Keys) -> AsyncIterator[AIOKafkaProducer]:
    admin = AIOKafkaAdminClient(bootstrap_servers=broker)
    await admin.start()
    await admin.create_topics(
        [
            NewTopic(t, num_partitions=3, replication_factor=1)
            for topic in TOPICS
            for t in (topic, f"{topic}.dlq")
        ]
    )
    await admin.close()
    app = create_app(
        dataclasses.replace(config, kafka_brokers=[broker], kafka_consumer_enabled=True),
        httpx.AsyncClient(transport=keys.transport()),
    )
    producer = AIOKafkaProducer(bootstrap_servers=broker)
    await producer.start()
    async with app.router.lifespan_context(app):
        yield producer
    await producer.stop()


async def eventually[T](read: Callable[[], Awaitable[T | None]], timeout_s: float = 60) -> T:
    deadline = asyncio.get_running_loop().time() + timeout_s
    while True:
        value = await read()
        if value is not None:
            return value
        if asyncio.get_running_loop().time() > deadline:
            raise AssertionError("condition not met in time")
        await asyncio.sleep(0.25)


def go_envelope(event_type: str, data: dict[str, object]) -> bytes:
    return json.dumps(
        {
            "id": str(uuid7()),
            "type": event_type,
            "version": 1,
            "source": "social",
            "subject": "s",
            "occurred_at": "2026-10-08T10:00:00.123456789Z",
            "data": data,
        }
    ).encode()


async def test_blocking_comment_is_removed(
    running: AIOKafkaProducer, sessions: async_sessionmaker[AsyncSession]
) -> None:
    comment, video = str(uuid7()), str(uuid7())
    value = go_envelope(
        "poro.social.comment.created",
        {
            "comment_id": comment,
            "user_id": str(uuid7()),
            "video_id": video,
            "video_owner_id": str(uuid7()),
            "parent_id": None,
            "parent_author_id": None,
            "excerpt": "Je vais te tuer",
            "text": "Je vais te tuer",
            "created_at": datetime.now(UTC).isoformat(),
        },
    )
    await running.send_and_wait("poro.social.comment.created", value, key=video.encode())
    await running.send_and_wait("poro.social.comment.created", value, key=video.encode())

    async def removed() -> int | None:
        async with sessions() as session:
            count: int = (
                await session.execute(
                    text("SELECT count(*) FROM outbox_events WHERE event_key = :id AND topic = :t"),
                    {"id": comment, "t": "poro.moderation.content.removed"},
                )
            ).scalar_one()
        return count or None

    assert await eventually(removed) == 1


async def test_poison_goes_to_the_dlq(broker: str, running: AIOKafkaProducer) -> None:
    topic = "poro.video.ready"
    await running.send_and_wait(topic, go_envelope(topic, {"video_id": "nope"}), key=b"poison")
    reader = AIOKafkaConsumer(
        f"{topic}.dlq",
        bootstrap_servers=broker,
        group_id=f"dlq-reader-{uuid7()}",
        auto_offset_reset="earliest",
    )
    await reader.start()
    try:
        msg = await asyncio.wait_for(reader.getone(), timeout=60)
    finally:
        await reader.stop()
    headers = dict(msg.headers)
    assert msg.key == b"poison"
    assert headers[HEADER_ERROR] == b"video_id must be a UUID"
    assert headers[HEADER_CONSUMER_GROUP] == b"poro-moderation-scanner"
