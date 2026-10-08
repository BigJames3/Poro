"""Kafka consumer group member with retries and a dead-letter queue, with the
same behaviour and DLQ headers as shared-go/kafka."""

import asyncio
import contextlib
import logging
from collections.abc import Awaitable, Callable, Sequence
from dataclasses import dataclass
from typing import Any, Protocol

from aiokafka import AIOKafkaConsumer, AIOKafkaProducer, TopicPartition

from moderation.events.envelope import Envelope, decode_envelope, dlq_topic

HEADER_ERROR = "x-poro-error"
HEADER_ORIGINAL_TOPIC = "x-poro-original-topic"
HEADER_ORIGINAL_PARTITION = "x-poro-original-partition"
HEADER_ORIGINAL_OFFSET = "x-poro-original-offset"
HEADER_CONSUMER_GROUP = "x-poro-consumer-group"

Handler = Callable[[Envelope], Awaitable[None]]
log = logging.getLogger(__name__)


class PermanentError(Exception):
    """Retrying cannot fix this event: dead-letter it at once."""


class StoppedError(Exception):
    """The consumer is stopping; the record must not be committed."""


@dataclass(frozen=True)
class Record:
    topic: str
    partition: int
    offset: int
    key: bytes | None
    value: bytes | None


class DeadLetterSink(Protocol):
    async def send_and_wait(
        self, topic: str, *, value: bytes | None, key: bytes | None, headers: list[tuple[str, bytes]]
    ) -> Any: ...


class Processor:
    """Applies one record: retries transient failures with exponential backoff,
    then parks the record on <topic>.dlq. It only raises once stop() was called,
    so the offset of a record neither applied nor parked is never committed."""

    def __init__(
        self,
        group: str,
        handler: Handler,
        dlq: DeadLetterSink,
        max_attempts: int = 3,
        backoff_s: float = 0.5,
    ) -> None:
        self._group = group
        self._handler = handler
        self._dlq = dlq
        self._max_attempts = max_attempts
        self._backoff_s = backoff_s
        self._stopped = asyncio.Event()

    def stop(self) -> None:
        self._stopped.set()

    async def process(self, record: Record) -> None:
        where = {"topic": record.topic, "partition": record.partition, "offset": record.offset}
        try:
            env = decode_envelope(record.value)
        except ValueError as err:
            log.error("undecodable event, dead-lettering", extra={**where, "error": str(err)})
            await self._dead_letter(record, err)
            return
        backoff = self._backoff_s
        attempt = 0
        while True:
            attempt += 1
            try:
                await self._handler(env)
                return
            except Exception as err:  # noqa: BLE001 -- any handler failure is retried, then parked.
                self._raise_if_stopped()
                if isinstance(err, PermanentError) or attempt >= self._max_attempts:
                    log.error(
                        "event failed, dead-lettering",
                        extra={**where, "event_id": str(env.id), "attempts": attempt, "error": str(err)},
                    )
                    await self._dead_letter(record, err)
                    return
                log.warning("event failed, retrying", extra={**where, "attempt": attempt, "error": str(err)})
                await self._sleep(backoff)
                backoff *= 2

    async def _dead_letter(self, record: Record, cause: BaseException) -> None:
        headers = [
            (HEADER_ERROR, str(cause)[:1000].encode()),
            (HEADER_ORIGINAL_TOPIC, record.topic.encode()),
            (HEADER_ORIGINAL_PARTITION, str(record.partition).encode()),
            (HEADER_ORIGINAL_OFFSET, str(record.offset).encode()),
            (HEADER_CONSUMER_GROUP, self._group.encode()),
        ]
        backoff = self._backoff_s
        while True:
            try:
                await self._dlq.send_and_wait(
                    dlq_topic(record.topic), value=record.value, key=record.key, headers=headers
                )
                return
            except Exception as err:  # noqa: BLE001 -- the DLQ publish is retried until stop.
                self._raise_if_stopped()
                log.error("dead letter publish failed", extra={"topic": record.topic, "error": str(err)})
                await self._sleep(backoff)
                backoff = min(backoff * 2, 30.0)

    def _raise_if_stopped(self) -> None:
        if self._stopped.is_set():
            raise StoppedError

    async def _sleep(self, seconds: float) -> None:
        try:
            await asyncio.wait_for(self._stopped.wait(), timeout=seconds)
        except TimeoutError:
            return
        raise StoppedError


class KafkaConsumer:
    """Runs a consumer group member in the background. A broker outage never
    stops the service: connection failures are retried until stop()."""

    def __init__(
        self,
        brokers: Sequence[str],
        group: str,
        topics: Sequence[str],
        handler: Handler,
        retry_s: float = 5.0,
    ) -> None:
        self._brokers = list(brokers)
        self._group = group
        self._topics = list(topics)
        self._handler = handler
        self._retry_s = retry_s
        self._task: asyncio.Task[None] | None = None
        self._processor: Processor | None = None
        self._stopping = asyncio.Event()

    def start(self) -> None:
        if self._task is None:
            self._task = asyncio.create_task(self._loop(), name=f"kafka:{self._group}")

    async def stop(self) -> None:
        self._stopping.set()
        if self._processor is not None:
            self._processor.stop()
        if self._task is not None:
            self._task.cancel()
            with contextlib.suppress(asyncio.CancelledError):
                await self._task

    async def _loop(self) -> None:
        while not self._stopping.is_set():
            try:
                await self._run()
            except asyncio.CancelledError:
                raise
            except Exception as err:  # noqa: BLE001 -- a broker outage must not stop the service.
                log.warning(
                    "kafka consumer stopped, retrying", extra={"group": self._group, "error": str(err)}
                )
                try:
                    await asyncio.wait_for(self._stopping.wait(), timeout=self._retry_s)
                except TimeoutError:
                    continue

    async def _run(self) -> None:
        producer = AIOKafkaProducer(bootstrap_servers=self._brokers, enable_idempotence=True, acks="all")
        consumer = AIOKafkaConsumer(
            *self._topics,
            bootstrap_servers=self._brokers,
            group_id=self._group,
            enable_auto_commit=False,
            auto_offset_reset="earliest",
        )
        await producer.start()
        try:
            await consumer.start()
            try:
                processor = Processor(self._group, self._handler, producer)
                self._processor = processor
                log.info("kafka consumer started", extra={"group": self._group, "topics": self._topics})
                async for msg in consumer:
                    await processor.process(Record(msg.topic, msg.partition, msg.offset, msg.key, msg.value))
                    await consumer.commit({TopicPartition(msg.topic, msg.partition): msg.offset + 1})
            finally:
                await consumer.stop()
        finally:
            await producer.stop()
