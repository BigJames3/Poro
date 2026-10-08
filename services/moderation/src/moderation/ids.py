"""UUIDv7 identifiers, ordered by creation time like the other services."""

import os
import re
import time
import uuid

_UUID = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", re.IGNORECASE)


def uuid7(now_ms: int | None = None) -> uuid.UUID:
    """Return an RFC 9562 UUIDv7: 48-bit Unix milliseconds, then random bits."""
    ms = int(time.time() * 1000) if now_ms is None else now_ms
    raw = bytearray(ms.to_bytes(6, "big") + os.urandom(10))
    raw[6] = (raw[6] & 0x0F) | 0x70
    raw[8] = (raw[8] & 0x3F) | 0x80
    return uuid.UUID(bytes=bytes(raw))


def parse_uuid(value: object) -> uuid.UUID | None:
    """Return the UUID in value, or None when it is not a canonical UUID string."""
    if not isinstance(value, str) or not _UUID.match(value):
        return None
    return uuid.UUID(value)
