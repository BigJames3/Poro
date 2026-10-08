"""Async SQLAlchemy engine on asyncpg."""

from sqlalchemy.ext.asyncio import AsyncEngine, async_sessionmaker, create_async_engine
from sqlalchemy.ext.asyncio import AsyncSession as AsyncSession

from moderation.config import Config


def create_engine(config: Config) -> AsyncEngine:
    return create_async_engine(
        config.database_url,
        pool_size=10,
        max_overflow=0,
        pool_pre_ping=True,
        connect_args={"ssl": config.database_ssl},
    )


def session_factory(engine: AsyncEngine) -> async_sessionmaker[AsyncSession]:
    return async_sessionmaker(engine, expire_on_commit=False)
