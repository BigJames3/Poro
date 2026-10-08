"""Shared fixtures: one Postgres container per run, migrated with Alembic, and
an application wired to it with a mock JWKS endpoint."""

import json
import os
import subprocess
import sys
import time
import uuid
from collections.abc import AsyncIterator, Iterator
from pathlib import Path
from typing import Any

import httpx
import jwt
import pytest
from cryptography.hazmat.primitives.asymmetric import rsa
from fastapi import FastAPI
from jwt.algorithms import RSAAlgorithm
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncEngine, AsyncSession, async_sessionmaker
from testcontainers.community.postgres import PostgresContainer

from moderation.app import create_app
from moderation.config import Config, load_config
from moderation.db import create_engine, session_factory
from moderation.ids import uuid7

ROOT = Path(__file__).resolve().parents[1]


def run_alembic(database_url: str, *args: str) -> None:
    subprocess.run(  # noqa: S603 -- fixed command line.
        [sys.executable, "-m", "alembic", *args],
        cwd=ROOT,
        env={**os.environ, "DATABASE_URL": database_url},
        check=True,
        capture_output=True,
    )


@pytest.fixture(scope="session")
def database_url() -> Iterator[str]:
    with PostgresContainer(
        "postgres:16-alpine", username="poro", password="poro_test", dbname="poro_moderation"
    ) as pg:
        url = f"postgresql://poro:poro_test@127.0.0.1:{pg.get_exposed_port(5432)}/poro_moderation?sslmode=disable"
        run_alembic(url, "upgrade", "head")
        yield url


@pytest.fixture(scope="session")
def config(database_url: str) -> Config:
    return load_config(
        {"DATABASE_URL": database_url, "KAFKA_CONSUMER_ENABLED": "false", "LOG_LEVEL": "WARNING"}
    )


@pytest.fixture(scope="session")
async def engine(config: Config) -> AsyncIterator[AsyncEngine]:
    engine = create_engine(config)
    yield engine
    await engine.dispose()


@pytest.fixture(scope="session")
def sessions(engine: AsyncEngine) -> async_sessionmaker[AsyncSession]:
    return session_factory(engine)


class Keys:
    """An RSA signing key served as a JWKS, like the auth service."""

    def __init__(self) -> None:
        self.private = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        self.kid = "test-key"
        jwk: dict[str, Any] = json.loads(RSAAlgorithm.to_jwk(self.private.public_key()))
        self.jwks: dict[str, Any] = {"keys": [{**jwk, "kid": self.kid, "use": "sig", "alg": "RS256"}]}
        self.status = 200
        self.requests = 0

    def transport(self) -> httpx.MockTransport:
        def handle(_: httpx.Request) -> httpx.Response:
            self.requests += 1
            return httpx.Response(self.status, json=self.jwks)

        return httpx.MockTransport(handle)

    def token(
        self, sub: uuid.UUID | str | None = None, roles: list[str] | None = None, **overrides: Any
    ) -> str:
        now = int(time.time())
        claims: dict[str, Any] = {
            "sub": str(sub or uuid7()),
            "roles": roles if roles is not None else ["PERSONAL"],
            "sid": str(uuid7()),
            "jti": str(uuid7()),
            "iss": "poro-auth",
            "aud": "poro-api",
            "iat": now,
            "exp": now + 900,
        }
        kid = overrides.pop("kid", self.kid)
        claims.update(overrides)
        claims = {k: v for k, v in claims.items() if v is not None}
        headers = {} if kid is None else {"kid": kid}
        return jwt.encode(claims, self.private, algorithm="RS256", headers=headers)


@pytest.fixture(scope="session")
def keys() -> Keys:
    return Keys()


@pytest.fixture(scope="session")
async def app(config: Config, keys: Keys) -> AsyncIterator[FastAPI]:
    client = httpx.AsyncClient(transport=keys.transport())
    application = create_app(config, client)
    async with application.router.lifespan_context(application):
        yield application
    await client.aclose()


@pytest.fixture
async def http(app: FastAPI) -> AsyncIterator[httpx.AsyncClient]:
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://moderation") as client:
        yield client


async def outbox(sessions: async_sessionmaker[AsyncSession], topic: str, key: str) -> list[dict[str, Any]]:
    async with sessions() as session:
        rows = await session.execute(
            text(
                "SELECT payload FROM outbox_events WHERE topic = :t AND event_key = :k "
                "ORDER BY created_at, id"
            ),
            {"t": topic, "k": key},
        )
        return [r[0] for r in rows]
