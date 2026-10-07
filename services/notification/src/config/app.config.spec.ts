import { loadConfig } from './app.config';

const prod = {
  APP_ENV: 'prod',
  DATABASE_URL: 'postgresql://poro:secret@db:5432/poro_notification?sslmode=verify-full',
  REDIS_URL: 'rediss://redis:6379',
  KAFKA_BROKERS: 'kafka-1:9092, kafka-2:9092',
  CORS_ORIGINS: 'https://app.poro.africa',
  TRUST_PROXY: '1',
};

function credentials(doc: Record<string, unknown>): string {
  return Buffer.from(JSON.stringify(doc)).toString('base64');
}

const serviceAccount = {
  type: 'service_account',
  project_id: 'poro-test',
  client_email: 'push@poro-test.iam.gserviceaccount.com',
  private_key: '-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----\n',
};

describe('loadConfig', () => {
  it('has safe dev defaults', () => {
    const config = loadConfig({
      DATABASE_URL: 'postgresql://poro:x@localhost:5433/poro_notification',
    });
    expect(config).toMatchObject({
      env: 'dev',
      port: 8086,
      logLevel: 'debug',
      jwksUrl: 'http://localhost:8081/.well-known/jwks.json',
      kafkaBrokers: ['localhost:9092'],
      kafkaConsumerEnabled: true,
      redisUrl: undefined,
      corsOrigins: [],
      trustProxy: false,
      fcm: undefined,
      retentionDays: 90,
      purgeEnabled: true,
    });
  });

  it('accepts a hardened prod environment', () => {
    const config = loadConfig({
      ...prod,
      FCM_CREDENTIALS_B64: credentials(serviceAccount),
      NOTIFICATION_RETENTION_DAYS: '30',
      NOTIFICATION_PURGE_ENABLED: 'false',
    });
    expect(config.kafkaBrokers).toEqual(['kafka-1:9092', 'kafka-2:9092']);
    expect(config.trustProxy).toBe(1);
    expect(config.logLevel).toBe('info');
    expect(config.fcm).toEqual({
      projectId: 'poro-test',
      clientEmail: 'push@poro-test.iam.gserviceaccount.com',
      privateKey: serviceAccount.private_key,
    });
    expect(config.retentionDays).toBe(30);
    expect(config.purgeEnabled).toBe(false);
    expect(loadConfig({ ...prod, TRUST_PROXY: 'loopback' }).trustProxy).toBe('loopback');
  });

  it.each([
    [{ APP_ENV: 'production' }, 'APP_ENV'],
    [{ PORT: '70000' }, 'PORT'],
    [{ LOG_LEVEL: 'verbose' }, 'LOG_LEVEL'],
    [{ DATABASE_URL: 'mysql://x' }, 'DATABASE_URL must be'],
    [
      {
        DATABASE_URL: 'postgresql://poro:x@db/poro_notification?sslmode=disable',
      },
      'sslmode',
    ],
    [{ REDIS_URL: undefined }, 'REDIS_URL is required'],
    [{ REDIS_URL: 'http://redis' }, 'REDIS_URL must be'],
    [{ KAFKA_BROKERS: ' , ' }, 'KAFKA_BROKERS'],
    [{ CORS_ORIGINS: '*' }, 'CORS_ORIGINS'],
    [{ CORS_ORIGINS: 'http://app.poro.africa' }, 'https outside dev'],
    [{ JWKS_URL: 'nope' }, 'JWKS_URL'],
    [{ FCM_CREDENTIALS_B64: '%%%' }, 'FCM_CREDENTIALS_B64 must be the base64'],
    [
      {
        FCM_CREDENTIALS_B64: credentials({
          ...serviceAccount,
          private_key: 'x',
        }),
      },
      'FCM_CREDENTIALS_B64 must hold',
    ],
    [
      {
        FCM_CREDENTIALS_B64: credentials({ ...serviceAccount, project_id: '' }),
      },
      'FCM_CREDENTIALS_B64 must hold',
    ],
    [{ NOTIFICATION_RETENTION_DAYS: '3' }, 'NOTIFICATION_RETENTION_DAYS'],
    [{ NOTIFICATION_RETENTION_DAYS: 'x' }, 'NOTIFICATION_RETENTION_DAYS'],
  ])('rejects %p', (override, message) => {
    expect(() => loadConfig({ ...prod, ...override })).toThrow(message);
  });
});
