import { ConfigType, registerAs } from '@nestjs/config';

export const APP_ENVS = ['dev', 'staging', 'prod'] as const;
export type AppEnv = (typeof APP_ENVS)[number];

const LOG_LEVELS = ['trace', 'debug', 'info', 'warn', 'error', 'fatal'] as const;
type LogLevel = (typeof LOG_LEVELS)[number];

/** A Firebase service account, decoded from FCM_CREDENTIALS_B64. */
export interface FcmCredentials {
  projectId: string;
  clientEmail: string;
  privateKey: string;
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
  fcm: FcmCredentials | undefined;
  retentionDays: number;
  purgeEnabled: boolean;
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
 * Decodes the base64 JSON of a Firebase service account. Unset means push is
 * disabled: in-app notifications keep working.
 */
function parseFcmCredentials(
  raw: string | undefined,
  errors: string[],
): FcmCredentials | undefined {
  const value = (raw ?? '').trim();
  if (value === '') {
    return undefined;
  }
  let doc: Record<string, unknown>;
  try {
    doc = JSON.parse(Buffer.from(value, 'base64').toString('utf8')) as Record<string, unknown>;
  } catch {
    errors.push('FCM_CREDENTIALS_B64 must be the base64 of a service account JSON');
    return undefined;
  }
  const { project_id: projectId, client_email: clientEmail, private_key: privateKey } = doc;
  if (
    typeof projectId !== 'string' ||
    projectId === '' ||
    typeof clientEmail !== 'string' ||
    clientEmail === '' ||
    typeof privateKey !== 'string' ||
    !privateKey.includes('PRIVATE KEY')
  ) {
    errors.push('FCM_CREDENTIALS_B64 must hold project_id, client_email and private_key');
    return undefined;
  }
  return { projectId, clientEmail, privateKey };
}

/**
 * Reads and validates the environment. Staging and prod refuse the shortcuts
 * that are only safe on a laptop: plaintext Postgres, in-memory rate limits
 * and wildcard CORS.
 */
export function loadConfig(env: NodeJS.ProcessEnv): AppConfig {
  const errors: string[] = [];
  const appEnv = (env.APP_ENV ?? 'dev') as AppEnv;
  if (!APP_ENVS.includes(appEnv)) {
    errors.push(`APP_ENV must be dev, staging or prod, got "${appEnv}"`);
  }
  const dev = appEnv === 'dev';

  const port = Number(env.PORT ?? '8086');
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

  const fcm = parseFcmCredentials(env.FCM_CREDENTIALS_B64, errors);

  const retentionDays = Number(env.NOTIFICATION_RETENTION_DAYS ?? '90');
  if (!Number.isInteger(retentionDays) || retentionDays < 7 || retentionDays > 3650) {
    errors.push('NOTIFICATION_RETENTION_DAYS must be an integer between 7 and 3650');
  }
  const purgeEnabled = (env.NOTIFICATION_PURGE_ENABLED ?? 'true') !== 'false';

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
    fcm,
    retentionDays,
    purgeEnabled,
  };
}

export const appConfig = registerAs('app', () => loadConfig(process.env));
export type AppConfigType = ConfigType<typeof appConfig>;
