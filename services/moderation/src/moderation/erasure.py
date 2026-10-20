"""Consumer group poro-moderation-account-erasure: forgets the personal data
of an erased account (ADR-0018), once per event."""

import logging

from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from moderation.cases import erase_account
from moderation.events.consumer import PermanentError
from moderation.events.envelope import TYPE_USER_DELETED, Envelope, parse_time
from moderation.events.store import claim
from moderation.ids import parse_uuid

GROUP = "poro-moderation-account-erasure"
TOPICS = (TYPE_USER_DELETED,)
log = logging.getLogger(__name__)


class Eraser:
    def __init__(self, sessions: async_sessionmaker[AsyncSession]) -> None:
        self._sessions = sessions

    async def handle(self, env: Envelope) -> None:
        if env.type != TYPE_USER_DELETED or env.version != 1:
            raise PermanentError(f"unsupported event {env.type} v{env.version}")
        user = parse_uuid(env.data.get("user_id"))
        if user is None or user.int == 0:
            raise PermanentError("user_id must be a UUID")
        # Moderation holds no object in storage: the claim and the erasure
        # commit together.
        async with self._sessions.begin() as session:
            if not await claim(session, GROUP, env.id):
                return
            await erase_account(session, user, parse_time(env.data.get("deleted_at")))
        log.info("account erased", extra={"user_id": str(user)})
