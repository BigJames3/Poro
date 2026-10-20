-- CreateTable
CREATE TABLE "erased_users" (
    "user_id" UUID NOT NULL,
    "erased_at" TIMESTAMPTZ(6) NOT NULL,

    CONSTRAINT "erased_users_pkey" PRIMARY KEY ("user_id")
);
