import { loadConfig } from './app.config';

const prod = {
  APP_ENV: 'prod',
  DATABASE_URL: 'postgresql://poro:secret@db:5432/poro_shop?sslmode=verify-full',
  REDIS_URL: 'rediss://redis:6379',
  KAFKA_BROKERS: 'kafka-1:9092, kafka-2:9092',
  S3_ENDPOINT: 'http://seaweedfs:8333',
  S3_PUBLIC_ENDPOINT: 'https://storage.poro.africa',
  S3_ACCESS_KEY_ID: 'key',
  S3_SECRET_ACCESS_KEY: 'secret',
  MEDIA_PUBLIC_BASE_URL: 'https://cdn.poro.africa/shop/',
  CORS_ORIGINS: 'https://app.poro.africa',
  TRUST_PROXY: '1',
};

describe('loadConfig', () => {
  it('has safe dev defaults', () => {
    const config = loadConfig({ DATABASE_URL: 'postgresql://poro:x@localhost:5433/poro_shop' });
    expect(config).toMatchObject({
      env: 'dev',
      port: 8090,
      logLevel: 'debug',
      jwksUrl: 'http://localhost:8081/.well-known/jwks.json',
      kafkaBrokers: ['localhost:9092'],
      kafkaConsumerEnabled: true,
      redisUrl: undefined,
      corsOrigins: [],
      trustProxy: false,
      mediaPublicBaseUrl: 'http://localhost:9000/poro-shop',
    });
  });

  it('accepts a hardened prod environment', () => {
    const config = loadConfig(prod);
    expect(config.kafkaBrokers).toEqual(['kafka-1:9092', 'kafka-2:9092']);
    expect(config.trustProxy).toBe(1);
    expect(config.logLevel).toBe('info');
    expect(config.mediaPublicBaseUrl).toBe('https://cdn.poro.africa/shop');
    expect(loadConfig({ ...prod, TRUST_PROXY: 'loopback' }).trustProxy).toBe('loopback');
  });

  it.each([
    [{ APP_ENV: 'production' }, 'APP_ENV'],
    [{ PORT: '70000' }, 'PORT'],
    [{ LOG_LEVEL: 'verbose' }, 'LOG_LEVEL'],
    [{ DATABASE_URL: 'mysql://x' }, 'DATABASE_URL must be'],
    [{ DATABASE_URL: 'postgresql://poro:x@db/poro_shop?sslmode=disable' }, 'sslmode'],
    [{ REDIS_URL: undefined }, 'REDIS_URL is required'],
    [{ REDIS_URL: 'http://redis' }, 'REDIS_URL must be'],
    [{ KAFKA_BROKERS: ' , ' }, 'KAFKA_BROKERS'],
    [{ CORS_ORIGINS: '*' }, 'CORS_ORIGINS'],
    [{ CORS_ORIGINS: 'http://app.poro.africa' }, 'https outside dev'],
    [{ S3_PUBLIC_ENDPOINT: 'http://storage.poro.africa' }, 'S3_PUBLIC_ENDPOINT must use https'],
    [{ S3_ENDPOINT: 'not a url' }, 'S3_ENDPOINT'],
    [{ S3_BUCKET: 'Bad_Bucket' }, 'S3_BUCKET'],
    [{ S3_SECRET_ACCESS_KEY: undefined }, 'S3_ACCESS_KEY_ID'],
    [{ JWKS_URL: 'nope' }, 'JWKS_URL'],
    [{ MEDIA_PUBLIC_BASE_URL: 'nope' }, 'MEDIA_PUBLIC_BASE_URL'],
  ])('rejects %p', (override, message) => {
    expect(() => loadConfig({ ...prod, ...override })).toThrow(message);
  });
});
