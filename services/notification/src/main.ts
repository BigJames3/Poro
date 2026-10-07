import { tracing } from './tracing';

import { NestFactory } from '@nestjs/core';
import { NestExpressApplication } from '@nestjs/platform-express';

import { AppModule } from './app.module';
import { configureApp } from './app.setup';
import { appConfig, type AppConfigType } from './config/app.config';

async function bootstrap(): Promise<void> {
  const app = await NestFactory.create<NestExpressApplication>(AppModule, {
    bodyParser: false,
    bufferLogs: true,
  });
  configureApp(app);
  const config = app.get<AppConfigType>(appConfig.KEY);
  await app.listen(config.port, '0.0.0.0');
}

process.on('SIGTERM', () => {
  void tracing?.shutdown();
});

bootstrap().catch((err: unknown) => {
  process.stderr.write(
    `notification service: ${err instanceof Error ? (err.stack ?? err.message) : String(err)}\n`,
  );
  process.exit(1);
});
