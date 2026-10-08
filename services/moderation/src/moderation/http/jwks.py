"""Offline verification of auth access tokens against the auth JWKS, with the
same rules as shared-go/jwtauth: RS256, issuer poro-auth, audience poro-api."""

import asyncio
import time
import uuid
from dataclasses import dataclass
from typing import Any

import httpx
import jwt
from jwt.algorithms import RSAAlgorithm

from moderation.ids import parse_uuid

ISSUER = "poro-auth"
AUDIENCE = "poro-api"
MAX_JWKS_BYTES = 64 * 1024
MIN_RSA_BITS = 2048


class KeysUnavailableError(Exception):
    """The JWKS could not be fetched and no cached key matches."""

    def __init__(self) -> None:
        super().__init__("jwks unavailable")


class InvalidTokenError(Exception):
    """The token is not acceptable. The message is for logs only."""


@dataclass(frozen=True)
class Principal:
    user_id: uuid.UUID
    roles: frozenset[str]
    session_id: str

    def has_any(self, *roles: str) -> bool:
        return any(role in self.roles for role in roles)


class JwksKeyStore:
    """Caches the auth public keys; refreshes when stale or on an unknown kid,
    at most once per min_gap. Last good keys keep working while auth is down."""

    def __init__(
        self,
        url: str,
        client: httpx.AsyncClient,
        max_age_s: float = 300,
        min_gap_s: float = 30,
    ) -> None:
        self._url = url
        self._client = client
        self._max_age_s = max_age_s
        self._min_gap_s = min_gap_s
        self._keys: dict[str, Any] = {}
        self._fetched_at = 0.0
        self._last_attempt = float("-inf")
        self._last_error: Exception | None = None
        self._lock = asyncio.Lock()

    async def key(self, kid: str) -> Any:
        now = time.monotonic()
        stale = now - self._fetched_at > self._max_age_s
        if (stale or kid not in self._keys) and now - self._last_attempt >= self._min_gap_s:
            await self._refresh()
        if kid in self._keys:
            return self._keys[kid]
        if self._last_error is not None:
            raise KeysUnavailableError from self._last_error
        raise InvalidTokenError(f"unknown signing key {kid!r}")

    async def _refresh(self) -> None:
        async with self._lock:
            if time.monotonic() - self._last_attempt < self._min_gap_s:
                return
            try:
                self._keys = await self._fetch()
                self._fetched_at = time.monotonic()
                self._last_error = None
            except Exception as err:  # noqa: BLE001 -- kept and reported when no cached key matches.
                self._last_error = err
            finally:
                self._last_attempt = time.monotonic()

    async def _fetch(self) -> dict[str, Any]:
        res = await self._client.get(self._url, headers={"accept": "application/json"}, timeout=3.0)
        if res.status_code != 200:
            raise RuntimeError(f"jwks: status {res.status_code}")
        if len(res.content) > MAX_JWKS_BYTES:
            raise RuntimeError("jwks: document too large")
        doc = res.json()
        keys: dict[str, Any] = {}
        for jwk in doc.get("keys", []) if isinstance(doc, dict) else []:
            if not isinstance(jwk, dict) or jwk.get("kty") != "RSA" or not jwk.get("kid"):
                continue
            if jwk.get("use", "sig") != "sig" or jwk.get("alg", "RS256") != "RS256":
                continue
            try:
                key = RSAAlgorithm.from_jwk(jwk)
            except (ValueError, jwt.InvalidKeyError):
                continue
            if getattr(key, "key_size", 0) >= MIN_RSA_BITS:
                keys[str(jwk["kid"])] = key
        if not keys:
            raise RuntimeError("jwks: no usable RS256 key")
        return keys


class TokenVerifier:
    def __init__(self, keys: JwksKeyStore) -> None:
        self._keys = keys

    async def verify(self, token: str) -> Principal:
        try:
            header = jwt.get_unverified_header(token)
        except jwt.PyJWTError as err:
            raise InvalidTokenError(str(err)) from err
        kid = header.get("kid")
        if not isinstance(kid, str) or not kid:
            raise InvalidTokenError("missing kid")
        key = await self._keys.key(kid)
        try:
            claims = jwt.decode(
                token,
                key,
                algorithms=["RS256"],
                issuer=ISSUER,
                audience=AUDIENCE,
                leeway=5,
                options={"require": ["sub", "exp", "jti", "sid"]},
            )
        except jwt.PyJWTError as err:
            raise InvalidTokenError(str(err)) from err
        user_id = parse_uuid(claims.get("sub"))
        roles = claims.get("roles")
        if user_id is None or not isinstance(roles, list) or not all(isinstance(r, str) for r in roles):
            raise InvalidTokenError("malformed claims")
        return Principal(user_id, frozenset(roles), str(claims["sid"]))
