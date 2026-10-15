"""Account erasure (ADR-0018): erased accounts and snapshots without an owner.

Revision ID: 0002
Revises: 0001
"""

from alembic import op

revision = "0002"
down_revision = "0001"
branch_labels = None
depends_on = None


def _execute(script: str) -> None:
    """asyncpg runs one statement per call."""
    for statement in script.split(";"):
        if statement.strip():
            op.execute(statement)


def upgrade() -> None:
    _execute("""
-- Accounts erased by poro.auth.user.deleted. Only the opaque id is kept, so
-- that a late event never stores their text again.
CREATE TABLE erased_users (
    user_id   UUID PRIMARY KEY,
    erased_at TIMESTAMPTZ NOT NULL
);

-- The snapshots of an erased account keep their target, without text or owner.
ALTER TABLE content_snapshots ALTER COLUMN owner_id DROP NOT NULL;
CREATE INDEX content_snapshots_owner_idx ON content_snapshots (owner_id);
""")


def downgrade() -> None:
    # Snapshots without an owner are empty: they are dropped to restore the constraint.
    _execute("""
DROP INDEX IF EXISTS content_snapshots_owner_idx;
DELETE FROM content_snapshots WHERE owner_id IS NULL;
ALTER TABLE content_snapshots ALTER COLUMN owner_id SET NOT NULL;
DROP TABLE IF EXISTS erased_users;
""")
