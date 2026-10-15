"""Rate limiter backends, JWKS parsing, logs and migrations."""

import json
import logging
from typing import Any

import fakeredis
import httpx
import pytest

from moderation.http.jwks import InvalidTokenError, JwksKeyStore, KeysUnavailableError
from moderation.http.ratelimit import MemoryLimiter, RedisLimiter
from moderation.http.revocations import KEY_PREFIX, RedisRevocations, RevocationUnavailableError
from moderation.logs import JsonFormatter, configure
from tests.conftest import Keys, run_alembic


async def test_redis_and_memory_limiters() -> None:
    redis = fakeredis.FakeAsyncRedis()
    for limiter in (RedisLimiter(redis), MemoryLimiter()):
        results = [await limiter.hit("k", 2, 60) for _ in range(3)]
        assert results[:2] == [0, 0]
        assert 0 < results[2] <= 60
        # Another window on another key keeps its own count.
        assert [await limiter.hit("h", 1, 3600) for _ in range(2)][1] > 0
        assert await limiter.hit("other", 2, 60) == 0
    await redis.aclose()


@pytest.mark.parametrize(
    "doc",
    [
        {"keys": []},
        {"keys": [{"kty": "EC", "kid": "a"}]},
        {"keys": [{"kty": "RSA", "kid": "a", "n": "AQAB", "e": "AQAB"}]},
        [],
    ],
)
async def test_jwks_without_usable_keys(doc: Any) -> None:
    client = httpx.AsyncClient(transport=httpx.MockTransport(lambda _: httpx.Response(200, json=doc)))
    store = JwksKeyStore("http://auth/jwks", client)
    with pytest.raises(KeysUnavailableError):
        await store.key("a")
    await client.aclose()


async def test_jwks_filters_weak_and_foreign_keys(keys: Keys) -> None:
    good = keys.jwks["keys"][0]
    doc = {"keys": [{**good, "kid": "enc", "use": "enc"}, {**good, "kid": "other", "alg": "RS512"}, good]}
    client = httpx.AsyncClient(transport=httpx.MockTransport(lambda _: httpx.Response(200, json=doc)))
    store = JwksKeyStore("http://auth/jwks", client)
    assert await store.key(keys.kid) is not None
    store._last_attempt = 0.0
    with pytest.raises(InvalidTokenError, match="unknown signing key"):
        await store.key("enc")
    await client.aclose()


async def test_memory_limiter_prunes_old_windows() -> None:
    limiter = MemoryLimiter()
    limiter._counts = {f"old{i}:60": (0, 1) for i in range(10_001)}
    assert await limiter.hit("fresh", 5, 60) == 0
    assert list(limiter._counts) == ["fresh:60"]


async def test_jwks_too_large() -> None:
    client = httpx.AsyncClient(
        transport=httpx.MockTransport(lambda _: httpx.Response(200, content=b" " * 70_000))
    )
    with pytest.raises(KeysUnavailableError):
        await JwksKeyStore("http://auth/jwks", client).key("a")
    await client.aclose()


def test_json_logs(capsys: pytest.CaptureFixture[str]) -> None:
    configure("INFO")
    log = logging.getLogger("moderation.test")
    log.info("hello", extra={"case_id": "c1"})
    try:
        raise ValueError("boom")
    except ValueError:
        log.exception("failed")
    lines = [json.loads(line) for line in capsys.readouterr().err.splitlines()]
    assert lines[0]["msg"] == "hello"
    assert lines[0]["case_id"] == "c1"
    assert lines[0]["service"] == "moderation"
    assert "boom" in lines[1]["error"]
    assert isinstance(JsonFormatter(), logging.Formatter)


def test_migrations_go_down_and_up(database_url: str) -> None:
    run_alembic(database_url, "downgrade", "base")
    run_alembic(database_url, "upgrade", "head")


async def test_revocation_registry() -> None:
    server = fakeredis.FakeServer()
    registry = RedisRevocations(fakeredis.FakeAsyncRedis(server=server))
    writer = fakeredis.FakeAsyncRedis(server=server)
    revoked, other = "0192A3B4-C5D6-7E8F-9A0B-1C2D3E4F5A6B", "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6c"
    assert KEY_PREFIX == "poro:revoked:sid:", "the layout auth writes"
    await writer.set(KEY_PREFIX + revoked.lower(), "1", ex=960)
    assert await registry.is_revoked(revoked)
    assert not await registry.is_revoked(other)

    server.connected = False
    with pytest.raises(RevocationUnavailableError):
        await registry.is_revoked(other)
    await registry.close()
    await writer.aclose()


def test_revocation_registry_connects_from_url() -> None:
    registry = RedisRevocations.connect("redis://127.0.0.1:1/15")
    assert isinstance(registry, RedisRevocations)
