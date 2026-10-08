"""Fixed-window rate limits per account, in Redis (shared by replicas) or in
memory when REDIS_URL is unset (dev only)."""

import time
from typing import Protocol

from redis.asyncio import Redis

from moderation.cases import DomainError


class Limiter(Protocol):
    async def hit(self, key: str, limit: int, window_s: int) -> int:
        """Count one hit; return the seconds to wait, or 0 when allowed."""
        ...


class RedisLimiter:
    def __init__(self, redis: Redis) -> None:
        self._redis = redis

    async def hit(self, key: str, limit: int, window_s: int) -> int:
        window = int(time.time()) // window_s
        name = f"moderation:rl:{key}:{window}"
        async with self._redis.pipeline(transaction=True) as pipe:
            pipe.incr(name)
            pipe.expire(name, window_s)
            count, _ = await pipe.execute()
        if int(count) > limit:
            return window_s - int(time.time()) % window_s
        return 0


class MemoryLimiter:
    def __init__(self) -> None:
        self._counts: dict[str, tuple[int, int]] = {}

    async def hit(self, key: str, limit: int, window_s: int) -> int:
        now = int(time.time())
        window = now // window_s
        name = f"{key}:{window_s}"
        if len(self._counts) > 10_000:
            self._counts = {k: v for k, v in self._counts.items() if v[0] == now // int(k.rsplit(":", 1)[1])}
        seen_window, count = self._counts.get(name, (window, 0))
        count = count + 1 if seen_window == window else 1
        self._counts[name] = (window, count)
        if count > limit:
            return window_s - now % window_s
        return 0


class RateLimitedError(DomainError):
    def __init__(self, retry_after: int) -> None:
        super().__init__(429, "rate_limited", "too many requests")
        self.retry_after = retry_after


async def enforce(limiter: Limiter, key: str, limit: int, window_s: int) -> None:
    wait = await limiter.hit(key, limit, window_s)
    if wait > 0:
        raise RateLimitedError(wait)
