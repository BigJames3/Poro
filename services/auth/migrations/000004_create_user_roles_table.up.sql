BEGIN;

CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role),
    CONSTRAINT user_roles_role_check CHECK (role IN ('PERSONAL', 'CREATOR', 'BUSINESS', 'ENTERPRISE', 'ADMIN', 'MODERATOR', 'SUPPORT'))
);

CREATE INDEX idx_user_roles_role ON user_roles (role);

INSERT INTO user_roles (user_id, role, granted_at)
SELECT id, role, created_at FROM users;

INSERT INTO user_roles (user_id, role, granted_at)
SELECT id, 'PERSONAL', created_at FROM users
ON CONFLICT (user_id, role) DO NOTHING;

ALTER TABLE users DROP COLUMN role;

COMMIT;
