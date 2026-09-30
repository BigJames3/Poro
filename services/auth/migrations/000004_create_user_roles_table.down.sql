BEGIN;

ALTER TABLE users ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'PERSONAL';

-- Lossy: keeps only the most privileged role of each user.
UPDATE users u
SET role = r.role
FROM (
    SELECT DISTINCT ON (user_id) user_id, role
    FROM user_roles
    ORDER BY user_id, CASE role
        WHEN 'ADMIN' THEN 1
        WHEN 'MODERATOR' THEN 2
        WHEN 'SUPPORT' THEN 3
        WHEN 'ENTERPRISE' THEN 4
        WHEN 'BUSINESS' THEN 5
        WHEN 'CREATOR' THEN 6
        ELSE 7
    END
) r
WHERE u.id = r.user_id;

ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('PERSONAL', 'CREATOR', 'BUSINESS', 'ENTERPRISE', 'ADMIN', 'MODERATOR', 'SUPPORT'));
CREATE INDEX idx_users_role ON users (role);

DROP TABLE IF EXISTS user_roles;

COMMIT;
