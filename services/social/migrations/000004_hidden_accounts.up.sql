-- Accounts whose deletion is pending (poro.auth.user.deletion_scheduled):
-- their videos, comments, likes and follows are left out of the public reads until the deletion is cancelled or
-- the account erased. Latest changed_at wins (see shared-go/hiding).
CREATE TABLE hidden_accounts (
    user_id    UUID PRIMARY KEY,
    hidden     BOOLEAN NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL
);
