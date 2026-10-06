BEGIN;

ALTER TABLE refresh_tokens ADD COLUMN family_id UUID;
UPDATE refresh_tokens SET family_id = id;
ALTER TABLE refresh_tokens ALTER COLUMN family_id SET NOT NULL;

ALTER TABLE refresh_tokens ADD COLUMN replaced_by UUID REFERENCES refresh_tokens (id) ON DELETE SET NULL;

CREATE INDEX idx_refresh_tokens_family_id ON refresh_tokens (family_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_refresh_tokens_replaced_by ON refresh_tokens (replaced_by) WHERE replaced_by IS NOT NULL;

COMMIT;
