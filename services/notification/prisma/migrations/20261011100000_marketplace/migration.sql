-- AlterTable
ALTER TABLE "preferences" ADD COLUMN     "orders" BOOLEAN NOT NULL DEFAULT true;

-- CreateTable
CREATE TABLE "shop_projections" (
    "shop_id" UUID NOT NULL,
    "name" VARCHAR(60) NOT NULL,
    "handle" VARCHAR(30) NOT NULL,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "shop_projections_pkey" PRIMARY KEY ("shop_id")
);

-- Order notifications: new types and the order entity.
ALTER TABLE "notifications" DROP CONSTRAINT "notifications_type_check";
ALTER TABLE "notifications" ADD CONSTRAINT "notifications_type_check" CHECK ("type" IN ('like', 'comment', 'reply', 'follow', 'video_ready', 'order_new', 'order_confirmed', 'order_shipped', 'order_cancelled', 'order_completed'));
ALTER TABLE "notifications" DROP CONSTRAINT "notifications_entity_type_check";
ALTER TABLE "notifications" ADD CONSTRAINT "notifications_entity_type_check" CHECK ("entity_type" IN ('video', 'comment', 'user', 'order'));
