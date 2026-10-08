-- Search read model, fed only by events (ADR-0011). Postgres full-text search:
-- French stemming without accents, trigrams for typos and prefixes.
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- unaccent() is only STABLE, but indexes and generated columns need IMMUTABLE.
CREATE FUNCTION search_fold(t text) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
    AS $$ SELECT lower(public.unaccent('public.unaccent'::regdictionary, t)) $$;

-- array_to_string() is STABLE too, and hashtags are plain text.
CREATE FUNCTION search_join(tags text[]) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
    AS $$ SELECT array_to_string(tags, ' ') $$;

CREATE TEXT SEARCH CONFIGURATION search_fr (COPY = french);
ALTER TEXT SEARCH CONFIGURATION search_fr
    ALTER MAPPING FOR hword, hword_part, word WITH unaccent, french_stem;

CREATE TABLE videos (
    video_id          UUID PRIMARY KEY,
    author_id         UUID NOT NULL,
    title             VARCHAR(100) NOT NULL DEFAULT '',
    description       VARCHAR(500) NOT NULL DEFAULT '',
    hashtags          TEXT[] NOT NULL DEFAULT '{}',
    thumbnail_key     TEXT,
    duration_ms       INT,
    -- NULL until poro.video.ready: a row may exist earlier for its counters.
    published_at      TIMESTAMPTZ,
    deleted_at        TIMESTAMPTZ,
    moderation_status VARCHAR(10) NOT NULL DEFAULT 'approved',
    likes_count       BIGINT NOT NULL DEFAULT 0,
    comments_count    BIGINT NOT NULL DEFAULT 0,
    shares_count      BIGINT NOT NULL DEFAULT 0,
    document          tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('search_fr', title), 'A') ||
        setweight(to_tsvector('search_fr', search_join(hashtags)), 'A') ||
        setweight(to_tsvector('search_fr', description), 'B')
    ) STORED,
    CONSTRAINT videos_moderation_chk CHECK (moderation_status IN ('approved', 'rejected')),
    CONSTRAINT videos_counts_chk CHECK (likes_count >= 0 AND comments_count >= 0 AND shares_count >= 0)
);
CREATE INDEX videos_document_idx ON videos USING gin (document);
CREATE INDEX videos_title_trgm_idx ON videos USING gin (search_fold(title) gin_trgm_ops);

CREATE TABLE users (
    user_id         UUID PRIMARY KEY,
    username        VARCHAR(30),
    display_name    VARCHAR(50),
    avatar_url      TEXT,
    is_creator      BOOLEAN NOT NULL DEFAULT false,
    followers_count BIGINT NOT NULL DEFAULT 0,
    -- Snapshot time of the profile, the newest snapshot wins.
    profile_at      TIMESTAMPTZ,
    document        tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', search_fold(coalesce(username, ''))), 'A') ||
        setweight(to_tsvector('search_fr', coalesce(display_name, '')), 'A')
    ) STORED,
    CONSTRAINT users_followers_chk CHECK (followers_count >= 0)
);
CREATE INDEX users_document_idx ON users USING gin (document);
CREATE INDEX users_name_trgm_idx ON users
    USING gin (search_fold(coalesce(username, '') || ' ' || coalesce(display_name, '')) gin_trgm_ops);
CREATE INDEX users_username_prefix_idx ON users (lower(username) text_pattern_ops) WHERE username IS NOT NULL;

-- Visible videos per hashtag, kept in step with every visibility change.
CREATE TABLE hashtags (
    tag          VARCHAR(50) PRIMARY KEY,
    videos_count BIGINT NOT NULL DEFAULT 0,
    last_used_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT hashtags_count_chk CHECK (videos_count >= 0)
);
CREATE INDEX hashtags_fold_prefix_idx ON hashtags (search_fold(tag) text_pattern_ops) WHERE videos_count > 0;
CREATE INDEX hashtags_fold_trgm_idx ON hashtags USING gin (search_fold(tag) gin_trgm_ops);

CREATE TABLE processed_events (
    consumer     VARCHAR(100) NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX processed_events_processed_at_idx ON processed_events (processed_at);
