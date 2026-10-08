-- CreateTable
CREATE TABLE "devices" (
    "id" UUID NOT NULL,
    "user_id" UUID NOT NULL,
    "fcm_token" VARCHAR(512) NOT NULL,
    "platform" VARCHAR(10) NOT NULL,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "devices_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "notifications" (
    "id" UUID NOT NULL,
    "user_id" UUID NOT NULL,
    "type" VARCHAR(20) NOT NULL,
    "title" VARCHAR(200) NOT NULL,
    "body" VARCHAR(500) NOT NULL,
    "data" JSONB NOT NULL,
    "actor_id" UUID,
    "actor_count" INTEGER NOT NULL DEFAULT 1,
    "entity_type" VARCHAR(20) NOT NULL,
    "entity_id" UUID NOT NULL,
    "video_id" UUID,
    "group_key" VARCHAR(200) NOT NULL,
    "read_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "last_activity_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "notifications_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "notification_actors" (
    "notification_id" UUID NOT NULL,
    "actor_id" UUID NOT NULL,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "notification_actors_pkey" PRIMARY KEY ("notification_id","actor_id")
);

-- CreateTable
CREATE TABLE "preferences" (
    "user_id" UUID NOT NULL,
    "push_enabled" BOOLEAN NOT NULL DEFAULT true,
    "likes" BOOLEAN NOT NULL DEFAULT true,
    "comments" BOOLEAN NOT NULL DEFAULT true,
    "follows" BOOLEAN NOT NULL DEFAULT true,
    "video_ready" BOOLEAN NOT NULL DEFAULT true,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "preferences_pkey" PRIMARY KEY ("user_id")
);

-- CreateTable
CREATE TABLE "user_projections" (
    "user_id" UUID NOT NULL,
    "username" VARCHAR(30),
    "display_name" VARCHAR(50),
    "avatar_url" VARCHAR(2048),
    "updated_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "user_projections_pkey" PRIMARY KEY ("user_id")
);

-- CreateTable
CREATE TABLE "inbox_events" (
    "consumer" VARCHAR(100) NOT NULL,
    "event_id" UUID NOT NULL,
    "processed_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "inbox_events_pkey" PRIMARY KEY ("consumer","event_id")
);

-- CreateIndex
CREATE UNIQUE INDEX "devices_fcm_token_key" ON "devices"("fcm_token");

-- CreateIndex
CREATE INDEX "devices_user_id_updated_at_idx" ON "devices"("user_id", "updated_at");

-- CreateIndex
CREATE INDEX "notifications_user_id_last_activity_at_id_idx" ON "notifications"("user_id", "last_activity_at" DESC, "id" DESC);

-- CreateIndex
CREATE INDEX "notifications_user_id_read_at_idx" ON "notifications"("user_id", "read_at");

-- CreateIndex
CREATE INDEX "notifications_entity_type_entity_id_idx" ON "notifications"("entity_type", "entity_id");

-- CreateIndex
CREATE INDEX "notifications_video_id_idx" ON "notifications"("video_id");

-- CreateIndex
CREATE INDEX "notifications_last_activity_at_idx" ON "notifications"("last_activity_at");

-- CreateIndex
CREATE UNIQUE INDEX "notifications_user_id_group_key_key" ON "notifications"("user_id", "group_key");

-- CreateIndex
CREATE INDEX "inbox_events_processed_at_idx" ON "inbox_events"("processed_at");

-- AddForeignKey
ALTER TABLE "notification_actors" ADD CONSTRAINT "notification_actors_notification_id_fkey" FOREIGN KEY ("notification_id") REFERENCES "notifications"("id") ON DELETE CASCADE ON UPDATE CASCADE;


-- Defense in depth: the service only writes these values.
ALTER TABLE "devices" ADD CONSTRAINT "devices_platform_check" CHECK ("platform" IN ('android', 'ios', 'web'));
ALTER TABLE "notifications" ADD CONSTRAINT "notifications_type_check" CHECK ("type" IN ('like', 'comment', 'reply', 'follow', 'video_ready'));
ALTER TABLE "notifications" ADD CONSTRAINT "notifications_entity_type_check" CHECK ("entity_type" IN ('video', 'comment', 'user'));
ALTER TABLE "notifications" ADD CONSTRAINT "notifications_actor_count_check" CHECK ("actor_count" >= 1);
