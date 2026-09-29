CREATE TABLE IF NOT EXISTS otp_codes (
    id UUID PRIMARY KEY,
    phone VARCHAR(20) NOT NULL,
    code_hash TEXT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    expires_at TIMESTAMPTZ NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_otp_codes_phone ON otp_codes (phone, created_at DESC);
CREATE INDEX idx_otp_codes_expires_at ON otp_codes (expires_at);
