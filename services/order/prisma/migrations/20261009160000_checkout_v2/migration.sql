-- Checkout v2: address book, previews confirmed explicitly, location of the delivery.

-- AlterTable
ALTER TABLE "orders" ADD COLUMN "landmark" VARCHAR(300),
ADD COLUMN "latitude" DOUBLE PRECISION,
ADD COLUMN "longitude" DOUBLE PRECISION,
ADD COLUMN "location_accuracy_m" INTEGER;

-- CreateTable
CREATE TABLE "addresses" (
    "id" UUID NOT NULL,
    "buyer_id" UUID NOT NULL,
    "label" VARCHAR(40),
    "full_name" VARCHAR(80) NOT NULL,
    "phone" VARCHAR(16) NOT NULL,
    "city" VARCHAR(80) NOT NULL,
    "address" VARCHAR(300),
    "landmark" VARCHAR(300),
    "latitude" DOUBLE PRECISION,
    "longitude" DOUBLE PRECISION,
    "location_accuracy_m" INTEGER,
    "is_default" BOOLEAN NOT NULL DEFAULT false,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "addresses_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "checkout_previews" (
    "id" UUID NOT NULL,
    "buyer_id" UUID NOT NULL,
    "cart_hash" CHAR(64) NOT NULL,
    "request" JSONB NOT NULL,
    "expires_at" TIMESTAMPTZ(6) NOT NULL,
    "checkout_id" UUID,
    "confirmed_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "checkout_previews_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX "addresses_buyer_id_created_at_idx" ON "addresses"("buyer_id", "created_at");

-- CreateIndex
CREATE INDEX "checkout_previews_buyer_id_created_at_idx" ON "checkout_previews"("buyer_id", "created_at");

-- CreateIndex
CREATE INDEX "checkout_previews_expires_at_idx" ON "checkout_previews"("expires_at");

-- Defense in depth: the service validates these values before writing them.
CREATE UNIQUE INDEX "addresses_one_default" ON "addresses"("buyer_id") WHERE "is_default";
ALTER TABLE "addresses" ADD CONSTRAINT "addresses_location" CHECK (
    ("latitude" IS NULL) = ("longitude" IS NULL)
    AND ("latitude" IS NULL OR ("latitude" BETWEEN -90 AND 90 AND "longitude" BETWEEN -180 AND 180))
    AND ("address" IS NOT NULL OR "latitude" IS NOT NULL));
ALTER TABLE "orders" ADD CONSTRAINT "orders_location" CHECK (
    ("latitude" IS NULL) = ("longitude" IS NULL)
    AND ("latitude" IS NULL OR ("latitude" BETWEEN -90 AND 90 AND "longitude" BETWEEN -180 AND 180)));
