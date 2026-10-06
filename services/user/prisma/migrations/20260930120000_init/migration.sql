-- CreateTable
CREATE TABLE "profiles" (
    "user_id" UUID NOT NULL,
    "username" VARCHAR(30),
    "display_name" VARCHAR(50),
    "bio" VARCHAR(160),
    "avatar_key" VARCHAR(200),
    "country_code" CHAR(2),
    "is_creator" BOOLEAN NOT NULL DEFAULT false,
    "creator_since" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ(6) NOT NULL,
    "deleted_at" TIMESTAMPTZ(6),

    CONSTRAINT "profiles_pkey" PRIMARY KEY ("user_id")
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
CREATE UNIQUE INDEX "profiles_username_key" ON "profiles"("username");

-- CreateIndex
CREATE INDEX "profiles_is_creator_creator_since_idx" ON "profiles"("is_creator", "creator_since");

-- CreateIndex
CREATE INDEX "outbox_events_published_at_created_at_id_idx" ON "outbox_events"("published_at", "created_at", "id");

-- CreateIndex
CREATE INDEX "processed_events_processed_at_idx" ON "processed_events"("processed_at");

-- Defense in depth: the service normalizes usernames before writing them.
ALTER TABLE "profiles" ADD CONSTRAINT "profiles_username_format" CHECK ("username" ~ '^[a-z0-9._]{3,30}$');
ALTER TABLE "profiles" ADD CONSTRAINT "profiles_country_code_format" CHECK ("country_code" ~ '^[A-Z]{2}$');
