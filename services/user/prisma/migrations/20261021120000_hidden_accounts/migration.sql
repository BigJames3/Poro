-- Accounts whose deletion is pending (poro.auth.user.deletion_scheduled): their
-- profile answers 404 until the deletion is cancelled or the account erased.
-- Latest changed_at wins, the two events travel on different topics.
CREATE TABLE "hidden_accounts" (
    "user_id" UUID NOT NULL,
    "hidden" BOOLEAN NOT NULL,
    "changed_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "hidden_accounts_pkey" PRIMARY KEY ("user_id")
);
