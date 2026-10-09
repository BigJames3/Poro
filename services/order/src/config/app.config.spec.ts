import { loadConfig } from './app.config';

const prod = {
  APP_ENV: 'prod',
  DATABASE_URL: 'postgresql://poro:secret@db:5432/poro_order?sslmode=verify-full',
  REDIS_URL: 'rediss://redis:6379',
  KAFKA_BROKERS: 'kafka-1:9092, kafka-2:9092',
  CORS_ORIGINS: 'https://app.poro.africa',
  PAYMENT_METHODS: 'cash_on_delivery, wave',
  TRUST_PROXY: '1',
};

describe('loadConfig', () => {
  it('has safe dev defaults', () => {
    const config = loadConfig({ DATABASE_URL: 'postgresql://poro:x@localhost:5433/poro_order' });
    expect(config).toMatchObject({
      env: 'dev',
      port: 8091,
      logLevel: 'debug',
      jwksUrl: 'http://localhost:8081/.well-known/jwks.json',
      kafkaBrokers: ['localhost:9092'],
      kafkaConsumerEnabled: true,
      redisUrl: undefined,
      corsOrigins: [],
      trustProxy: false,
      paymentMethods: ['cash_on_delivery'],
      paymentTimeoutMinutes: 30,
      autoCompleteDays: 7,
      contactRetentionDays: 90,
      schedulerEnabled: true,
      schedulerIntervalMs: 60_000,
    });
  });

  it('accepts a hardened prod environment', () => {
    const config = loadConfig(prod);
    expect(config.kafkaBrokers).toEqual(['kafka-1:9092', 'kafka-2:9092']);
    expect(config.trustProxy).toBe(1);
    expect(config.logLevel).toBe('info');
    expect(config.paymentMethods).toEqual(['cash_on_delivery', 'wave']);
    expect(loadConfig({ ...prod, TRUST_PROXY: 'loopback' }).trustProxy).toBe('loopback');
  });

  it.each([
    [{ APP_ENV: 'production' }, 'APP_ENV'],
    [{ PORT: '70000' }, 'PORT'],
    [{ LOG_LEVEL: 'verbose' }, 'LOG_LEVEL'],
    [{ DATABASE_URL: 'mysql://x' }, 'DATABASE_URL must be'],
    [{ DATABASE_URL: 'postgresql://poro:x@db/poro_order?sslmode=disable' }, 'sslmode'],
    [{ REDIS_URL: undefined }, 'REDIS_URL is required'],
    [{ REDIS_URL: 'http://redis' }, 'REDIS_URL must be'],
    [{ KAFKA_BROKERS: ' , ' }, 'KAFKA_BROKERS'],
    [{ CORS_ORIGINS: '*' }, 'CORS_ORIGINS'],
    [{ CORS_ORIGINS: 'http://app.poro.africa' }, 'https outside dev'],
    [{ JWKS_URL: 'nope' }, 'JWKS_URL'],
    [{ PAYMENT_METHODS: 'cash_on_delivery,paypal' }, 'unknown method'],
    [{ PAYMENT_METHODS: 'simulated' }, 'only allowed in dev'],
    [{ PAYMENT_METHODS: ' , ' }, 'at least one method'],
    [{ PAYMENT_TIMEOUT_MINUTES: '0' }, 'PAYMENT_TIMEOUT_MINUTES'],
    [{ AUTO_COMPLETE_DAYS: 'x' }, 'AUTO_COMPLETE_DAYS'],
  ])('rejects %p', (override, message) => {
    expect(() => loadConfig({ ...prod, ...override })).toThrow(message);
  });
});
