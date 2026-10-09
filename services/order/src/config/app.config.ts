import { ConfigType, registerAs } from '@nestjs/config';

export const APP_ENVS = ['dev', 'staging', 'prod'] as const;
export type AppEnv = (typeof APP_ENVS)[number];

const LOG_LEVELS = ['trace', 'debug', 'info', 'warn', 'error', 'fatal'] as const;
type LogLevel = (typeof LOG_LEVELS)[number];

export interface AppConfig {
  env: AppEnv;
  port: number;
  logLevel: LogLevel;
  databaseUrl: string;
  jwksUrl: string;
  kafkaBrokers: string[];
  kafkaConsumerEnabled: boolean;
  redisUrl: string | undefined;
  corsOrigins: string[];
  trustProxy: boolean | number | string;
  /** Payment methods buyers may choose at checkout. */
  paymentMethods: PaymentMethod[];
  /** An order awaiting payment is cancelled after this long. */
  paymentTimeoutMinutes: number;
  /** A shipped order completes on its own after this many days. */
  autoCompleteDays: number;
  /** Delivery contacts of finished orders are erased after this many days. */
  contactRetentionDays: number;
  schedulerEnabled: boolean;
  schedulerIntervalMs: number;
}

export const PAYMENT_METHODS = ['cash_on_delivery', 'wave', 'simulated'] as const;
export type PaymentMethod = (typeof PAYMENT_METHODS)[number];

function positiveInt(
  raw: string | undefined,
  fallback: number,
  name: string,
  errors: string[],
): number {
  const value = Number(raw ?? String(fallback));
  if (!Number.isInteger(value) || value < 1) {
    errors.push(`${name} must be a positive integer`);
  }
  return value;
}

function list(raw: string | undefined): string[] {
  return (raw ?? '')
    .split(',')
    .map((part) => part.trim())
    .filter((part) => part !== '');
}

function parseUrl(raw: string): URL | undefined {
  try {
    return new URL(raw);
  } catch {
    return undefined;
  }
}

function parseTrustProxy(raw: string | undefined): boolean | number | string {
  const value = (raw ?? '').trim();
  if (value === '' || value === 'false') {
    return false;
  }
  return /^\d+$/.test(value) ? Number(value) : value;
}

/**
 * Reads and validates the environment. Staging and prod refuse the shortcuts
 * that are only safe on a laptop: plaintext Postgres, in-memory rate limits,
 * wildcard CORS and the simulated payment method.
 */
export function loadConfig(env: NodeJS.ProcessEnv): AppConfig {
  const errors: string[] = [];
  const appEnv = (env.APP_ENV ?? 'dev') as AppEnv;
  if (!APP_ENVS.includes(appEnv)) {
    errors.push(`APP_ENV must be dev, staging or prod, got "${appEnv}"`);
  }
  const dev = appEnv === 'dev';

  const port = Number(env.PORT ?? '8091');
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    errors.push('PORT must be an integer between 1 and 65535');
  }
  const logLevel = (env.LOG_LEVEL ?? (dev ? 'debug' : 'info')) as LogLevel;
  if (!LOG_LEVELS.includes(logLevel)) {
    errors.push(`LOG_LEVEL must be one of ${LOG_LEVELS.join(', ')}`);
  }

  const databaseUrl = env.DATABASE_URL ?? '';
  const db = parseUrl(databaseUrl);
  if (!db || !db.protocol.startsWith('postgres')) {
    errors.push('DATABASE_URL must be a postgresql:// URL');
  } else if (
    !dev &&
    !['require', 'verify-ca', 'verify-full'].includes(db.searchParams.get('sslmode') ?? '')
  ) {
    errors.push('DATABASE_URL must set sslmode=require, verify-ca or verify-full outside dev');
  }

  const jwksUrl = env.JWKS_URL ?? 'http://localhost:8081/.well-known/jwks.json';
  if (!parseUrl(jwksUrl)) {
    errors.push('JWKS_URL must be a URL');
  }

  const kafkaBrokers = list(env.KAFKA_BROKERS ?? 'localhost:9092');
  if (kafkaBrokers.length === 0) {
    errors.push('KAFKA_BROKERS is required');
  }
  const kafkaConsumerEnabled = (env.KAFKA_CONSUMER_ENABLED ?? 'true') !== 'false';

  const redisUrl = env.REDIS_URL?.trim() === '' ? undefined : env.REDIS_URL?.trim();
  if (redisUrl !== undefined && !parseUrl(redisUrl)?.protocol.startsWith('redis')) {
    errors.push('REDIS_URL must be a redis:// or rediss:// URL');
  }
  if (!dev && redisUrl === undefined) {
    errors.push('REDIS_URL is required outside dev: rate limits must be shared by replicas');
  }

  const corsOrigins = list(env.CORS_ORIGINS);
  for (const origin of corsOrigins) {
    const url = parseUrl(origin);
    if (origin === '*' || !url || url.origin !== origin) {
      errors.push(
        `CORS_ORIGINS: "${origin}" must be an exact origin such as https://app.poro.africa`,
      );
    } else if (!dev && url.protocol !== 'https:') {
      errors.push(`CORS_ORIGINS: "${origin}" must use https outside dev`);
    }
  }

  const paymentMethods = list(env.PAYMENT_METHODS ?? 'cash_on_delivery') as PaymentMethod[];
  if (paymentMethods.length === 0) {
    errors.push('PAYMENT_METHODS needs at least one method');
  }
  for (const method of paymentMethods) {
    if (!PAYMENT_METHODS.includes(method)) {
      errors.push(`PAYMENT_METHODS: unknown method "${method}"`);
    } else if (method === 'simulated' && !dev) {
      errors.push('PAYMENT_METHODS: simulated is only allowed in dev');
    }
  }
  const paymentTimeoutMinutes = positiveInt(
    env.PAYMENT_TIMEOUT_MINUTES,
    30,
    'PAYMENT_TIMEOUT_MINUTES',
    errors,
  );
  const autoCompleteDays = positiveInt(env.AUTO_COMPLETE_DAYS, 7, 'AUTO_COMPLETE_DAYS', errors);
  const contactRetentionDays = positiveInt(
    env.CONTACT_RETENTION_DAYS,
    90,
    'CONTACT_RETENTION_DAYS',
    errors,
  );
  const schedulerIntervalMs = positiveInt(
    env.SCHEDULER_INTERVAL_MS,
    60_000,
    'SCHEDULER_INTERVAL_MS',
    errors,
  );

  if (errors.length > 0) {
    throw new Error(`invalid config: ${errors.join('; ')}`);
  }
  return {
    env: appEnv,
    port,
    logLevel,
    databaseUrl,
    jwksUrl,
    kafkaBrokers,
    kafkaConsumerEnabled,
    redisUrl,
    corsOrigins,
    trustProxy: parseTrustProxy(env.TRUST_PROXY),
    paymentMethods,
    paymentTimeoutMinutes,
    autoCompleteDays,
    contactRetentionDays,
    schedulerEnabled: (env.SCHEDULER_ENABLED ?? 'true') !== 'false',
    schedulerIntervalMs,
  };
}

export const appConfig = registerAs('app', () => loadConfig(process.env));
export type AppConfigType = ConfigType<typeof appConfig>;
