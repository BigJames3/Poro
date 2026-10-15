-- Erased accounts (ADR-0018). The id alone keeps a replayed or late
-- poro.auth.user.created or poro.video.ready from recreating the projection.
CREATE TABLE erased_users (
    user_id   UUID PRIMARY KEY,
    erased_at TIMESTAMPTZ NOT NULL
);

-- A deleted comment may lose its text: the comments of an erased account are
-- kept without it, so the replies of others keep their thread.
ALTER TABLE comments DROP CONSTRAINT comments_content_chk;
ALTER TABLE comments ADD CONSTRAINT comments_content_chk
    CHECK (char_length(content) <= 1000 AND (char_length(content) >= 1 OR deleted_at IS NOT NULL));
