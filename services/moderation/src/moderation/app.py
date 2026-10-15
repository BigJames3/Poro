"""Application factory: wires the database, Redis, JWKS, the consumers and the routes."""

import logging
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

import httpx
from fastapi import FastAPI
from redis.asyncio import Redis

from moderation import SERVICE_NAME, SERVICE_VERSION
from moderation.api import platform, routes
from moderation.config import Config
from moderation.db import create_engine, session_factory
from moderation.erasure import GROUP as ERASURE_GROUP
from moderation.erasure import TOPICS as ERASURE_TOPICS
from moderation.erasure import Eraser
from moderation.events.consumer import KafkaConsumer
from moderation.http.errors import RequestIdMiddleware, install_error_handlers
from moderation.http.jwks import JwksKeyStore, TokenVerifier
from moderation.http.metrics import MetricsMiddleware
from moderation.http.ratelimit import Limiter, MemoryLimiter, RedisLimiter
from moderation.http.revocations import RedisRevocations
from moderation.rules.engine import RuleEngine
from moderation.scanner import GROUP, TOPICS, Scanner

log = logging.getLogger(__name__)


def create_app(config: Config, http_client: httpx.AsyncClient | None = None) -> FastAPI:
    """http_client fetches the JWKS; tests pass one with a mock transport."""

    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        engine = create_engine(config)
        redis = None if config.redis_url is None else Redis.from_url(config.redis_url)
        client = http_client or httpx.AsyncClient()
        rules = RuleEngine.from_file(config.rules_path)
        limiter: Limiter = MemoryLimiter() if redis is None else RedisLimiter(redis)
        app.state.config = config
        app.state.engine = engine
        app.state.sessions = session_factory(engine)
        app.state.redis = redis
        app.state.limiter = limiter
        revocations = None
        if config.revocation_redis_url is None:
            log.warning(
                "REVOCATION_REDIS_URL is not set: revoked sessions stay accepted until their tokens expire"
            )
        else:
            revocations = RedisRevocations.connect(config.revocation_redis_url)
        app.state.verifier = TokenVerifier(JwksKeyStore(config.jwks_url, client), revocations)
        app.state.scanner = Scanner(app.state.sessions, rules)
        app.state.eraser = Eraser(app.state.sessions)
        consumers: list[KafkaConsumer] = []
        if config.kafka_consumer_enabled:
            consumers = [
                KafkaConsumer(config.kafka_brokers, GROUP, TOPICS, app.state.scanner.handle),
                KafkaConsumer(config.kafka_brokers, ERASURE_GROUP, ERASURE_TOPICS, app.state.eraser.handle),
            ]
            for consumer in consumers:
                consumer.start()
        else:
            log.warning("kafka consumers disabled by KAFKA_CONSUMER_ENABLED=false")
        try:
            yield
        finally:
            for consumer in consumers:
                await consumer.stop()
            if http_client is None:
                await client.aclose()
            if redis is not None:
                await redis.aclose()
            if revocations is not None:
                await revocations.close()
            await engine.dispose()

    app = FastAPI(
        title="PORO Moderation API",
        version=SERVICE_VERSION,
        lifespan=lifespan,
        docs_url=None,
        redoc_url=None,
        openapi_url=None,
    )
    app.add_middleware(MetricsMiddleware)
    app.add_middleware(RequestIdMiddleware)
    install_error_handlers(app)
    app.include_router(platform.router)
    app.include_router(routes.router)
    app.state.service = SERVICE_NAME
    return app
