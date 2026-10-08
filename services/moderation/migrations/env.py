"""Alembic environment: migrations are plain SQL, applied with DATABASE_URL."""

import asyncio
import os

from alembic import context
from sqlalchemy.engine import Connection

from moderation.config import load_config
from moderation.db import create_engine


def _run(connection: Connection) -> None:
    context.configure(connection=connection, target_metadata=None, transaction_per_migration=True)
    with context.begin_transaction():
        context.run_migrations()


async def _main() -> None:
    engine = create_engine(load_config(os.environ))
    async with engine.connect() as connection:
        await connection.run_sync(_run)
        await connection.commit()
    await engine.dispose()


asyncio.run(_main())
