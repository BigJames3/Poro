-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "public";

-- CreateTable
CREATE TABLE "shops" (
    "id" UUID NOT NULL,
    "owner_id" UUID NOT NULL,
    "name" VARCHAR(60) NOT NULL,
    "handle" VARCHAR(30) NOT NULL,
    "description" VARCHAR(500),
    "logo_key" VARCHAR(200),
    "country_code" CHAR(2) NOT NULL,
    "currency" CHAR(3) NOT NULL,
    "status" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "shops_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "products" (
    "id" UUID NOT NULL,
    "shop_id" UUID NOT NULL,
    "title" VARCHAR(120) NOT NULL,
    "description" VARCHAR(2000),
    "status" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,
    "deleted_at" TIMESTAMPTZ(6),

    CONSTRAINT "products_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "product_images" (
    "id" UUID NOT NULL,
    "product_id" UUID NOT NULL,
    "key" VARCHAR(200) NOT NULL,
    "position" INTEGER NOT NULL,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "product_images_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "variants" (
    "id" UUID NOT NULL,
    "product_id" UUID NOT NULL,
    "title" VARCHAR(60) NOT NULL,
    "price" BIGINT NOT NULL,
    "stock_on_hand" INTEGER NOT NULL DEFAULT 0,
    "stock_reserved" INTEGER NOT NULL DEFAULT 0,
    "position" INTEGER NOT NULL,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,
    "deleted_at" TIMESTAMPTZ(6),

    CONSTRAINT "variants_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "reservations" (
    "order_id" UUID NOT NULL,
    "shop_id" UUID NOT NULL,
    "status" VARCHAR(16) NOT NULL,
    "currency" CHAR(3),
    "subtotal" BIGINT,
    "lines" JSONB NOT NULL DEFAULT '[]',
    "reason" VARCHAR(32),
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "reservations_pkey" PRIMARY KEY ("order_id")
);

-- CreateTable
CREATE TABLE "outbox_events" (
    "id" UUID NOT NULL,
    "topic" VARCHAR(200) NOT NULL,
    "event_key" VARCHAR(200) NOT NULL,
    "payload" JSONB NOT NULL,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "published_at" TIMESTAMPTZ(6),
    "attempts" INTEGER NOT NULL DEFAULT 0,
    "last_error" TEXT,

    CONSTRAINT "outbox_events_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "processed_events" (
    "consumer" VARCHAR(100) NOT NULL,
    "event_id" UUID NOT NULL,
    "processed_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "processed_events_pkey" PRIMARY KEY ("consumer","event_id")
);

-- CreateIndex
CREATE UNIQUE INDEX "shops_owner_id_key" ON "shops"("owner_id");

-- CreateIndex
CREATE UNIQUE INDEX "shops_handle_key" ON "shops"("handle");

-- CreateIndex
CREATE INDEX "products_shop_id_status_created_at_id_idx" ON "products"("shop_id", "status", "created_at" DESC, "id" DESC);

-- CreateIndex
CREATE INDEX "product_images_product_id_position_idx" ON "product_images"("product_id", "position");

-- CreateIndex
CREATE INDEX "variants_product_id_position_idx" ON "variants"("product_id", "position");

-- CreateIndex
CREATE INDEX "reservations_shop_id_created_at_idx" ON "reservations"("shop_id", "created_at");

-- CreateIndex
CREATE INDEX "outbox_events_published_at_created_at_id_idx" ON "outbox_events"("published_at", "created_at", "id");

-- CreateIndex
CREATE INDEX "processed_events_processed_at_idx" ON "processed_events"("processed_at");

-- AddForeignKey
ALTER TABLE "products" ADD CONSTRAINT "products_shop_id_fkey" FOREIGN KEY ("shop_id") REFERENCES "shops"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "product_images" ADD CONSTRAINT "product_images_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "products"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "variants" ADD CONSTRAINT "variants_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "products"("id") ON DELETE RESTRICT ON UPDATE CASCADE;


-- Defense in depth: the service validates these values before writing them.
ALTER TABLE "shops" ADD CONSTRAINT "shops_handle_format" CHECK ("handle" ~ '^[a-z0-9._]{3,30}$');
ALTER TABLE "shops" ADD CONSTRAINT "shops_country_currency" CHECK (
    ("country_code" IN ('CI', 'SN') AND "currency" = 'XOF')
    OR ("country_code" = 'CM' AND "currency" = 'XAF')
    OR ("country_code" = 'NG' AND "currency" = 'NGN'));
ALTER TABLE "shops" ADD CONSTRAINT "shops_status" CHECK ("status" IN ('active', 'suspended', 'closed'));
ALTER TABLE "products" ADD CONSTRAINT "products_status" CHECK ("status" IN ('draft', 'active', 'archived'));
ALTER TABLE "variants" ADD CONSTRAINT "variants_price_positive" CHECK ("price" > 0 AND "price" <= 1000000000000);
-- Reserved stock never exceeds the stock on hand: a reservation cannot oversell.
ALTER TABLE "variants" ADD CONSTRAINT "variants_stock" CHECK ("stock_reserved" >= 0 AND "stock_on_hand" >= "stock_reserved");
ALTER TABLE "reservations" ADD CONSTRAINT "reservations_status" CHECK ("status" IN ('held', 'released', 'consumed', 'rejected', 'cancelled'));
