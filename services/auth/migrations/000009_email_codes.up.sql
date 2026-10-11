BEGIN;

-- Set once the owner proved they read the address.
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;

-- Six-digit codes sent by email to verify the address or reset the password.
-- code_hash is an HMAC bound to the account, the address and the purpose.
CREATE TABLE email_codes (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose VARCHAR(10) NOT NULL,
    email VARCHAR(255) NOT NULL,
    code_hash TEXT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT email_codes_purpose_check CHECK (purpose IN ('verify', 'reset')),
    CONSTRAINT email_codes_attempts_check CHECK (attempts >= 0 AND attempts <= max_attempts)
);

CREATE INDEX idx_email_codes_user_purpose ON email_codes (user_id, purpose, created_at DESC);
CREATE INDEX idx_email_codes_expires_at ON email_codes (expires_at);

COMMIT;
