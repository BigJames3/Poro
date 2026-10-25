-- AlterTable
ALTER TABLE "notifications" ADD COLUMN "photo_post" BOOLEAN NOT NULL DEFAULT false;

-- CreateTable
CREATE TABLE "photo_posts" (
    "video_id" UUID NOT NULL,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "photo_posts_pkey" PRIMARY KEY ("video_id")
);
