import { PermanentError } from '../events/event-consumer';
import { TYPE_AUTH_USER_CREATED } from '../events/catalog';
import { uuidv7 } from '../common/uuid';
import { UserCreatedConsumer } from './user-created.consumer';

function consumer(): UserCreatedConsumer {
  return new UserCreatedConsumer(
    {} as never,
    { kafkaConsumerEnabled: false, kafkaBrokers: [] } as never,
  );
}

describe('UserCreatedConsumer.handle', () => {
  it('rejects unsupported type or version without touching the database', async () => {
    const { handle } = consumer();
    await expect(
      handle({
        id: uuidv7(),
        type: TYPE_AUTH_USER_CREATED,
        version: 2,
        source: 'poro-auth',
        subject: 'x',
        occurred_at: new Date().toISOString(),
        data: {},
      }),
    ).rejects.toBeInstanceOf(PermanentError);
    await expect(
      handle({
        id: uuidv7(),
        type: 'poro.auth.user.deleted',
        version: 1,
        source: 'poro-auth',
        subject: 'x',
        occurred_at: new Date().toISOString(),
        data: {},
      }),
    ).rejects.toBeInstanceOf(PermanentError);
  });

  it('rejects a non-uuid user_id', async () => {
    await expect(
      consumer().handle({
        id: uuidv7(),
        type: TYPE_AUTH_USER_CREATED,
        version: 1,
        source: 'poro-auth',
        subject: 'x',
        occurred_at: new Date().toISOString(),
        data: { user_id: 'not-a-uuid' },
      }),
    ).rejects.toMatchObject({ message: 'user_id must be a UUID' });
  });

  it('does not start Kafka when disabled', () => {
    expect(() => consumer().onApplicationBootstrap()).not.toThrow();
  });
});
