import { Logger } from '@nestjs/common';

import { uuidv7 } from '../common/uuid';
import { newEnvelope } from './envelope';
import {
  ConsumedMessage,
  ConsumerStoppedError,
  DeadLetterSink,
  EventProcessor,
  HEADER_CONSUMER_GROUP,
  HEADER_ERROR,
  HEADER_ORIGINAL_OFFSET,
  PermanentError,
} from './event-consumer';

type Sent = Parameters<DeadLetterSink['send']>[0];

class FakeDlq implements DeadLetterSink {
  sent: Sent[] = [];
  failures = 0;
  send(record: Sent): Promise<unknown> {
    if (this.failures > 0) {
      this.failures--;
      return Promise.reject(new Error('broker down'));
    }
    this.sent.push(record);
    return Promise.resolve();
  }
}

function message(value: Buffer | null): ConsumedMessage {
  return {
    topic: 'poro.order.order.created',
    partition: 2,
    offset: '41',
    key: Buffer.from('k'),
    value,
  };
}

function eventMessage(): ConsumedMessage {
  const env = newEnvelope('poro.order.order.created', 1, uuidv7(), { user_id: uuidv7() });
  return message(Buffer.from(JSON.stringify(env)));
}

describe('EventProcessor', () => {
  const logger = new Logger('test');
  const options = { group: 'poro-shop-stock', topics: ['poro.order.order.created'], backoffMs: 1 };

  it('retries transient failures', async () => {
    const dlq = new FakeDlq();
    let calls = 0;
    const processor = new EventProcessor(
      options,
      () => (++calls < 3 ? Promise.reject(new Error('db')) : Promise.resolve()),
      dlq,
      logger,
    );
    await processor.process(eventMessage());
    expect(calls).toBe(3);
    expect(dlq.sent).toHaveLength(0);
  });

  it('dead-letters after the last attempt with the original coordinates', async () => {
    const dlq = new FakeDlq();
    dlq.failures = 2;
    let calls = 0;
    const processor = new EventProcessor(
      options,
      () => {
        calls++;
        return Promise.reject(new Error('x'.repeat(2000)));
      },
      dlq,
      logger,
    );
    const msg = eventMessage();
    await processor.process(msg);
    expect(calls).toBe(3);
    expect(dlq.sent).toHaveLength(1);
    const [record] = dlq.sent;
    expect(record.topic).toBe('poro.order.order.created.dlq');
    expect(record.messages[0].value).toBe(msg.value);
    expect(record.messages[0].headers[HEADER_ERROR]).toHaveLength(1000);
    expect(record.messages[0].headers[HEADER_ORIGINAL_OFFSET]).toBe('41');
    expect(record.messages[0].headers[HEADER_CONSUMER_GROUP]).toBe('poro-shop-stock');
  });

  it('dead-letters permanent failures and undecodable records at once', async () => {
    const dlq = new FakeDlq();
    let calls = 0;
    const processor = new EventProcessor(
      options,
      () => {
        calls++;
        return Promise.reject(new PermanentError('bad data'));
      },
      dlq,
      logger,
    );
    await processor.process(eventMessage());
    await processor.process(message(Buffer.from('not json')));
    expect(calls).toBe(1);
    expect(dlq.sent.map((r) => r.messages[0].headers[HEADER_ERROR])).toEqual([
      'bad data',
      expect.stringContaining('invalid event envelope'),
    ]);
  });

  it('stops without committing when shut down mid-retry', async () => {
    const dlq = new FakeDlq();
    const processor = new EventProcessor(
      { ...options, backoffMs: 60_000 },
      () => Promise.reject(new Error('db')),
      dlq,
      logger,
    );
    const pending = processor.process(eventMessage());
    await new Promise((resolve) => setTimeout(resolve, 10));
    processor.stop();
    await expect(pending).rejects.toBeInstanceOf(ConsumerStoppedError);
    expect(dlq.sent).toHaveLength(0);
  });
});
