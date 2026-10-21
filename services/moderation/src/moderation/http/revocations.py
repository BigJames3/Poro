"""Registry of the device sessions auth revoked (ADR-0016).

Auth writes one key per revoked session in Redis, for as long as its access
tokens live; every service refuses those tokens. The key layout is shared with
shared-go/revocation and the NestJS services.
"""

from redis.asyncio import Redis
from redis.exceptions import RedisError

KEY_PREFIX = "poro:revoked:sid:"


class RevocationUnavailableError(Exception):
    """The registry could not be read: the request is refused with 503."""

    def __init__(self) -> None:
        super().__init__("revocation registry unavailable")


class RedisRevocations:
    def __init__(self, redis: Redis) -> None:
        self._redis = redis

    @classmethod
    def connect(cls, url: str) -> "RedisRevocations":
        """Every authenticated request reads the registry, so an outage must
        fail fast: short timeouts, no retry."""
        return cls(Redis.from_url(url, socket_timeout=0.5, socket_connect_timeout=1.0))

    async def is_revoked(self, session_id: str) -> bool:
        try:
            return bool(await self._redis.exists(KEY_PREFIX + session_id.lower()))
        except (RedisError, OSError) as err:
            raise RevocationUnavailableError() from err

    async def close(self) -> None:
        await self._redis.aclose()
