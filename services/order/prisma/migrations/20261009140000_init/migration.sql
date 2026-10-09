-- CreateSchema
CREATE SCHEMA IF NOT EXISTS "public";

-- CreateTable
CREATE TABLE "catalog_shops" (
    "id" UUID NOT NULL,
    "owner_id" UUID NOT NULL,
    "name" VARCHAR(60) NOT NULL,
    "handle" VARCHAR(30) NOT NULL,
    "currency" CHAR(3) NOT NULL,
    "status" VARCHAR(16) NOT NULL,
    "source_updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "catalog_shops_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "catalog_products" (
    "id" UUID NOT NULL,
    "shop_id" UUID NOT NULL,
    "title" VARCHAR(120) NOT NULL,
    "currency" CHAR(3) NOT NULL,
    "status" VARCHAR(16) NOT NULL,
    "image_url" VARCHAR(500),
    "source_updated_at" TIMESTAMPTZ(6) NOT NULL,
    "deleted_at" TIMESTAMPTZ(6),

    CONSTRAINT "catalog_products_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "catalog_variants" (
    "id" UUID NOT NULL,
    "product_id" UUID NOT NULL,
    "title" VARCHAR(60) NOT NULL,
    "price" BIGINT NOT NULL,
    "in_stock" BOOLEAN NOT NULL,
    "removed" BOOLEAN NOT NULL DEFAULT false,

    CONSTRAINT "catalog_variants_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "cart_items" (
    "buyer_id" UUID NOT NULL,
    "variant_id" UUID NOT NULL,
    "quantity" INTEGER NOT NULL,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "cart_items_pkey" PRIMARY KEY ("buyer_id","variant_id")
);

-- CreateTable
CREATE TABLE "checkouts" (
    "id" UUID NOT NULL,
    "buyer_id" UUID NOT NULL,
    "idempotency_key" VARCHAR(100) NOT NULL,
    "request_hash" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "checkouts_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "orders" (
    "id" UUID NOT NULL,
    "checkout_id" UUID NOT NULL,
    "buyer_id" UUID NOT NULL,
    "shop_id" UUID NOT NULL,
    "shop_name" VARCHAR(60) NOT NULL,
    "seller_id" UUID NOT NULL,
    "currency" CHAR(3) NOT NULL,
    "payment_method" VARCHAR(32) NOT NULL,
    "status" VARCHAR(20) NOT NULL,
    "subtotal_seen" BIGINT NOT NULL,
    "total" BIGINT,
    "contact_name" VARCHAR(80),
    "contact_phone" VARCHAR(16),
    "city" VARCHAR(80),
    "address" VARCHAR(300),
    "contact_erased_at" TIMESTAMPTZ(6),
    "tracking" VARCHAR(200),
    "cancel_reason" VARCHAR(32),
    "paid" BOOLEAN NOT NULL DEFAULT false,
    "last_payment_error" VARCHAR(32),
    "expires_at" TIMESTAMPTZ(6),
    "placed_at" TIMESTAMPTZ(6),
    "paid_at" TIMESTAMPTZ(6),
    "shipped_at" TIMESTAMPTZ(6),
    "completed_at" TIMESTAMPTZ(6),
    "cancelled_at" TIMESTAMPTZ(6),
    "refunded_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "orders_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "order_lines" (
    "order_id" UUID NOT NULL,
    "position" INTEGER NOT NULL,
    "product_id" UUID NOT NULL,
    "variant_id" UUID NOT NULL,
    "title" VARCHAR(200) NOT NULL,
    "quantity" INTEGER NOT NULL,
    "unit_price_seen" BIGINT NOT NULL,
    "unit_price" BIGINT,

    CONSTRAINT "order_lines_pkey" PRIMARY KEY ("order_id","position")
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
CREATE INDEX "catalog_products_shop_id_idx" ON "catalog_products"("shop_id");

-- CreateIndex
CREATE INDEX "catalog_variants_product_id_idx" ON "catalog_variants"("product_id");

-- CreateIndex
CREATE UNIQUE INDEX "checkouts_buyer_id_idempotency_key_key" ON "checkouts"("buyer_id", "idempotency_key");

-- CreateIndex
CREATE INDEX "orders_buyer_id_created_at_id_idx" ON "orders"("buyer_id", "created_at" DESC, "id" DESC);

-- CreateIndex
CREATE INDEX "orders_seller_id_created_at_id_idx" ON "orders"("seller_id", "created_at" DESC, "id" DESC);

-- CreateIndex
CREATE INDEX "orders_status_expires_at_idx" ON "orders"("status", "expires_at");

-- CreateIndex
CREATE INDEX "orders_status_shipped_at_idx" ON "orders"("status", "shipped_at");

-- CreateIndex
CREATE INDEX "outbox_events_published_at_created_at_id_idx" ON "outbox_events"("published_at", "created_at", "id");

-- CreateIndex
CREATE INDEX "processed_events_processed_at_idx" ON "processed_events"("processed_at");

-- AddForeignKey
ALTER TABLE "catalog_variants" ADD CONSTRAINT "catalog_variants_product_id_fkey" FOREIGN KEY ("product_id") REFERENCES "catalog_products"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "orders" ADD CONSTRAINT "orders_checkout_id_fkey" FOREIGN KEY ("checkout_id") REFERENCES "checkouts"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "order_lines" ADD CONSTRAINT "order_lines_order_id_fkey" FOREIGN KEY ("order_id") REFERENCES "orders"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- Defense in depth: the service validates these values before writing them.
ALTER TABLE "cart_items" ADD CONSTRAINT "cart_items_quantity" CHECK ("quantity" BETWEEN 1 AND 99);
ALTER TABLE "orders" ADD CONSTRAINT "orders_status" CHECK ("status" IN (
    'pending', 'awaiting_payment', 'confirmed', 'paid', 'shipped', 'completed', 'cancelled'));
ALTER TABLE "orders" ADD CONSTRAINT "orders_payment_method" CHECK ("payment_method" IN ('cash_on_delivery', 'wave', 'simulated'));
ALTER TABLE "orders" ADD CONSTRAINT "orders_currency" CHECK ("currency" IN ('XOF', 'XAF', 'NGN'));
ALTER TABLE "orders" ADD CONSTRAINT "orders_amounts" CHECK ("subtotal_seen" >= 0 AND ("total" IS NULL OR "total" >= 0));
ALTER TABLE "orders" ADD CONSTRAINT "orders_no_self_purchase" CHECK ("buyer_id" <> "seller_id");
ALTER TABLE "order_lines" ADD CONSTRAINT "order_lines_quantity" CHECK ("quantity" BETWEEN 1 AND 99);
