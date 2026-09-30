BEGIN;

UPDATE users
SET email = LOWER(TRIM(email))
WHERE email IS NOT NULL AND email <> LOWER(TRIM(email));

ALTER TABLE users ADD CONSTRAINT users_email_normalized_check CHECK (email = LOWER(TRIM(email)));

COMMIT;
