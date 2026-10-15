import { NestExpressApplication } from '@nestjs/platform-express';
import { Test } from '@nestjs/testing';

import { AppModule } from '../../src/app.module';
import { configureApp } from '../../src/app.setup';

/** Builds the real application. env overrides are applied before config loads. */
export async function createApp(env: Record<string, string>): Promise<NestExpressApplication> {
  Object.assign(
    process.env,
    {
      APP_ENV: 'dev',
      LOG_LEVEL: 'fatal',
      KAFKA_CONSUMER_ENABLED: 'false',
      S3_ACCESS_KEY_ID: 'test-key',
      S3_SECRET_ACCESS_KEY: 'test-secret',
    },
    env,
  );
  const moduleRef = await Test.createTestingModule({ imports: [AppModule] }).compile();
  const app = moduleRef.createNestApplication<NestExpressApplication>({ bodyParser: false });
  configureApp(app);
  await app.init();
  return app;
}
