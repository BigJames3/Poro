import { NestExpressApplication } from '@nestjs/platform-express';
import { Test } from '@nestjs/testing';

import { AppModule } from '../../src/app.module';
import { configureApp } from '../../src/app.setup';
import { PUSH_SENDER, type PushSender } from '../../src/push/push.sender';

/**
 * Builds the real application. env overrides are applied before config loads;
 * pushSender replaces FCM.
 */
export async function createApp(
  env: Record<string, string>,
  pushSender?: PushSender,
): Promise<NestExpressApplication> {
  Object.assign(
    process.env,
    {
      APP_ENV: 'dev',
      LOG_LEVEL: 'fatal',
      KAFKA_CONSUMER_ENABLED: 'false',
      NOTIFICATION_PURGE_ENABLED: 'false',
    },
    env,
  );
  let builder = Test.createTestingModule({ imports: [AppModule] });
  if (pushSender !== undefined) {
    builder = builder.overrideProvider(PUSH_SENDER).useValue(pushSender);
  }
  const moduleRef = await builder.compile();
  const app = moduleRef.createNestApplication<NestExpressApplication>({ bodyParser: false });
  configureApp(app);
  await app.init();
  return app;
}
