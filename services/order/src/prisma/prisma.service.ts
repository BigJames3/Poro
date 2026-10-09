import { Inject, Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { PrismaPg } from '@prisma/adapter-pg';

import { appConfig, type AppConfigType } from '../config/app.config';
import { PrismaClient } from '../generated/prisma/client';

@Injectable()
export class PrismaService extends PrismaClient implements OnModuleInit, OnModuleDestroy {
  constructor(@Inject(appConfig.KEY) config: AppConfigType) {
    super({ adapter: new PrismaPg({ connectionString: config.databaseUrl, max: 10 }) });
  }

  /** Fails fast: a service that cannot reach its database must not report started. */
  async onModuleInit(): Promise<void> {
    await this.$queryRaw`SELECT 1`;
  }

  async onModuleDestroy(): Promise<void> {
    await this.$disconnect();
  }

  async ping(): Promise<void> {
    await this.$queryRaw`SELECT 1`;
  }
}
