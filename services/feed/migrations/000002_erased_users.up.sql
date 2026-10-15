-- Accounts erased by poro.auth.user.deleted (ADR-0018). Only the opaque id is
-- kept, so that a late profile or follow event never projects them again.
CREATE TABLE erased_users (
    user_id   UUID PRIMARY KEY,
    erased_at TIMESTAMPTZ NOT NULL
);
