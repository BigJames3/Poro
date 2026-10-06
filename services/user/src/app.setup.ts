import { ValidationError, ValidationPipe } from '@nestjs/common';
import { NestExpressApplication } from '@nestjs/platform-express';
import helmet from 'helmet';
import { Logger } from 'nestjs-pino';

import { ApiError } from './common/api-error';
import { bodyParserErrors } from './common/http-exception.filter';
import { requestId } from './common/request';
import { appConfig, type AppConfigType } from './config/app.config';
import { MetricsService } from './metrics/metrics';

const JSON_BODY_LIMIT = '16kb';

function firstMessage(errors: ValidationError[]): string {
  for (const error of errors) {
    const message = Object.values(error.constraints ?? {})[0];
    if (message) {
      return message;
    }
    const nested = firstMessage(error.children ?? []);
    if (nested !== 'invalid request') {
      return nested;
    }
  }
  return 'invalid request';
}

/** Applies the HTTP middleware stack. Create the app with { bodyParser: false }. */
export function configureApp(app: NestExpressApplication): void {
  const config = app.get<AppConfigType>(appConfig.KEY);
  app.useLogger(app.get(Logger));
  app.disable('x-powered-by');
  app.set('trust proxy', config.trustProxy);

  app.use(requestId);
  app.use(app.get(MetricsService).middleware());
  app.use(helmet());
  app.useBodyParser('json', { limit: JSON_BODY_LIMIT });
  app.use(bodyParserErrors);
  if (config.corsOrigins.length > 0) {
    app.enableCors({
      origin: config.corsOrigins,
      methods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'],
      allowedHeaders: ['Authorization', 'Content-Type', 'X-Request-ID'],
      exposedHeaders: ['X-Request-ID'],
      maxAge: 600,
    });
  }
  app.useGlobalPipes(
    new ValidationPipe({
      whitelist: true,
      forbidNonWhitelisted: true,
      transform: true,
      exceptionFactory: (errors) => new ApiError(400, 'invalid_request', firstMessage(errors)),
    }),
  );
  app.enableShutdownHooks();
}
