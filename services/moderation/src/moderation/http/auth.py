"""FastAPI dependencies: the authenticated caller and the moderator role."""

import logging
from typing import Annotated

from fastapi import Depends, Request

from moderation.cases import DomainError
from moderation.http.jwks import InvalidTokenError, KeysUnavailableError, Principal, TokenVerifier

MODERATOR_ROLES = ("MODERATOR", "ADMIN")
log = logging.getLogger(__name__)


def _bearer(header: str | None) -> str | None:
    if header is None:
        return None
    parts = header.strip().split()
    if len(parts) != 2 or parts[0].lower() != "bearer" or not parts[1]:
        return None
    return parts[1]


async def current_user(request: Request) -> Principal:
    token = _bearer(request.headers.get("authorization"))
    if token is None:
        raise DomainError(401, "unauthorized", "unauthorized")
    verifier: TokenVerifier = request.app.state.verifier
    try:
        principal = await verifier.verify(token)
    except KeysUnavailableError as err:
        raise DomainError(503, "unavailable", "authentication temporarily unavailable") from err
    except InvalidTokenError as err:
        log.debug("token rejected", extra={"reason": str(err)})
        raise DomainError(401, "unauthorized", "unauthorized") from err
    request.state.principal = principal
    return principal


async def moderator(user: Annotated[Principal, Depends(current_user)]) -> Principal:
    if not user.has_any(*MODERATOR_ROLES):
        raise DomainError(403, "forbidden", "moderator role required")
    return user


CurrentUser = Annotated[Principal, Depends(current_user)]
Moderator = Annotated[Principal, Depends(moderator)]
