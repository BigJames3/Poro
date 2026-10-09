import { ConfigType, registerAs } from '@nestjs/config';

export const APP_ENVS = ['dev', 'staging', 'prod'] as const;
export type AppEnv = (typeof APP_ENVS)[number];

const LOG_LEVELS = ['trace', 'debug', 'info', 'warn', 'error', 'fatal'] as const;
type LogLevel = (typeof LOG_LEVELS)[number];

export interface S3Settings {
  endpoint: string;
  publicEndpoint: string;
  region: string;
  bucket: string;
  accessKeyId: string;
  secretAccessKey: string;
  forcePathStyle: boolean;
}

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
  s3: S3Settings;
  mediaPublicBaseUrl: string;
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
 * plain-HTTP storage and wildcard CORS.
 */
export function loadConfig(env: NodeJS.ProcessEnv): AppConfig {
  const errors: string[] = [];
  const appEnv = (env.APP_ENV ?? 'dev') as AppEnv;
  if (!APP_ENVS.includes(appEnv)) {
    errors.push(`APP_ENV must be dev, staging or prod, got "${appEnv}"`);
  }
  const dev = appEnv === 'dev';

  const port = Number(env.PORT ?? '8090');
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

  const s3Endpoint = env.S3_ENDPOINT ?? 'http://localhost:9000';
  const s3: S3Settings = {
    endpoint: s3Endpoint,
    publicEndpoint: env.S3_PUBLIC_ENDPOINT ?? s3Endpoint,
    region: env.S3_REGION ?? 'us-east-1',
    bucket: env.S3_BUCKET ?? 'poro-shop',
    accessKeyId: env.S3_ACCESS_KEY_ID ?? (dev ? 'poro_dev' : ''),
    secretAccessKey: env.S3_SECRET_ACCESS_KEY ?? (dev ? 'poro_dev_secret' : ''),
    forcePathStyle: (env.S3_FORCE_PATH_STYLE ?? 'true') !== 'false',
  };
  for (const [name, value] of [
    ['S3_ENDPOINT', s3.endpoint],
    ['S3_PUBLIC_ENDPOINT', s3.publicEndpoint],
  ] as const) {
    const url = parseUrl(value);
    if (!url) {
      errors.push(`${name} must be a URL`);
    } else if (!dev && name === 'S3_PUBLIC_ENDPOINT' && url.protocol !== 'https:') {
      errors.push('S3_PUBLIC_ENDPOINT must use https outside dev: clients upload through it');
    }
  }
  if (!/^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$/.test(s3.bucket)) {
    errors.push('S3_BUCKET is not a valid bucket name');
  }
  if (s3.accessKeyId === '' || s3.secretAccessKey === '') {
    errors.push('S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY are required outside dev');
  }

  const mediaPublicBaseUrl = (
    env.MEDIA_PUBLIC_BASE_URL ?? `${s3.publicEndpoint.replace(/\/$/, '')}/${s3.bucket}`
  ).replace(/\/$/, '');
  if (!parseUrl(mediaPublicBaseUrl)) {
    errors.push('MEDIA_PUBLIC_BASE_URL must be a URL');
  }

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
    s3,
    mediaPublicBaseUrl,
  };
}

export const appConfig = registerAs('app', () => loadConfig(process.env));
export type AppConfigType = ConfigType<typeof appConfig>;
