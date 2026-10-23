-- Shop catalogue read model (PORO-026), fed by poro.shop.shop.updated,
-- poro.shop.product.updated and poro.shop.product.deleted. The newest snapshot
-- wins, and a deleted product never comes back. No semicolon in comments: the
-- migration runner splits statements on it.
CREATE TABLE shops (
    shop_id      UUID PRIMARY KEY,
    owner_id     UUID NOT NULL,
    name         VARCHAR(60) NOT NULL,
    handle       VARCHAR(30) NOT NULL,
    description  VARCHAR(500),
    logo_url     TEXT,
    country_code CHAR(2) NOT NULL,
    currency     CHAR(3) NOT NULL,
    status       VARCHAR(16) NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    document     tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('search_fr', name), 'A') ||
        setweight(to_tsvector('simple', search_fold(handle)), 'A') ||
        setweight(to_tsvector('search_fr', coalesce(description, '')), 'C')
    ) STORED
);
CREATE INDEX shops_document_idx ON shops USING gin (document);
CREATE INDEX shops_name_trgm_idx ON shops USING gin (search_fold(name || ' ' || handle) gin_trgm_ops);
CREATE INDEX shops_owner_idx ON shops (owner_id);

CREATE TABLE products (
    product_id   UUID PRIMARY KEY,
    shop_id      UUID NOT NULL,
    owner_id     UUID NOT NULL,
    title        VARCHAR(120) NOT NULL,
    description  VARCHAR(2000),
    currency     CHAR(3) NOT NULL,
    -- active, draft, archived, or deleted once poro.shop.product.deleted arrived.
    status       VARCHAR(16) NOT NULL,
    image_url    TEXT,
    -- Lowest variant price in minor units, NULL without a variant.
    min_price    BIGINT,
    in_stock     BOOLEAN NOT NULL DEFAULT false,
    updated_at   TIMESTAMPTZ NOT NULL,
    document     tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('search_fr', title), 'A') ||
        setweight(to_tsvector('search_fr', coalesce(description, '')), 'C')
    ) STORED
);
CREATE INDEX products_document_idx ON products USING gin (document);
CREATE INDEX products_title_trgm_idx ON products USING gin (search_fold(title) gin_trgm_ops);
CREATE INDEX products_shop_idx ON products (shop_id);
CREATE INDEX products_owner_idx ON products (owner_id);
