import { Logger } from '@nestjs/common';
import { Consumer, Kafka, Producer, logLevel } from 'kafkajs';

import { Envelope, decodeEnvelope, dlqTopic } from './envelope';

/** Header names set on dead-lettered records, shared with shared-go/kafka. */
export const HEADER_ERROR = 'x-poro-error';
export const HEADER_ORIGINAL_TOPIC = 'x-poro-original-topic';
export const HEADER_ORIGINAL_PARTITION = 'x-poro-original-partition';
export const HEADER_ORIGINAL_OFFSET = 'x-poro-original-offset';
export const HEADER_CONSUMER_GROUP = 'x-poro-consumer-group';

/** Retrying cannot fix this event: dead-letter it at once. */
export class PermanentError extends Error {
  constructor(message: string, cause?: unknown) {
    super(message, { cause });
    this.name = 'PermanentError';
  }
}

export class ConsumerStoppedError extends Error {
  constructor() {
    super('consumer stopped');
    this.name = 'ConsumerStoppedError';
  }
}

export type EventHandler = (env: Envelope) => Promise<void>;

export interface ConsumedMessage {
  topic: string;
  partition: number;
  offset: string;
  key: Buffer | null;
  value: Buffer | null;
}

export interface DeadLetterSink {
  send(record: {
    topic: string;
    messages: { key: Buffer | null; value: Buffer | null; headers: Record<string, string> }[];
  }): Promise<unknown>;
}

export interface ConsumerOptions {
  group: string; // poro-{service}-{purpose}
  topics: string[];
  maxAttempts?: number;
  backoffMs?: number;
}

/**
 * Applies one record: retries transient failures with exponential backoff, then
 * parks the record on <topic>.dlq. It only throws once stop() was called, so the
 * offset of a record that was neither applied nor parked is never committed.
 */
export class EventProcessor {
  private readonly abort = new AbortController();
  private readonly maxAttempts: number;
  private readonly backoffMs: number;

  constructor(
    private readonly options: ConsumerOptions,
    private readonly handler: EventHandler,
    private readonly dlq: DeadLetterSink,
    private readonly logger: Logger,
  ) {
    this.maxAttempts = options.maxAttempts ?? 3;
    this.backoffMs = options.backoffMs ?? 500;
  }

  stop(): void {
    this.abort.abort();
  }

  async process(msg: ConsumedMessage): Promise<void> {
    const where = { topic: msg.topic, partition: msg.partition, offset: msg.offset };
    let env: Envelope;
    try {
      env = decodeEnvelope(msg.value);
    } catch (err) {
      this.logger.error({ msg: 'undecodable event, dead-lettering', ...where, err });
      return this.deadLetter(msg, err);
    }
    const context = { ...where, event_id: env.id, event_type: env.type };

    let backoff = this.backoffMs;
    for (let attempt = 1; ; attempt++) {
      try {
        await this.handler(env);
        this.logger.debug({ msg: 'event processed', ...context });
        return;
      } catch (err) {
        this.throwIfStopped();
        if (err instanceof PermanentError || attempt >= this.maxAttempts) {
          this.logger.error({
            msg: 'event failed, dead-lettering',
            ...context,
            attempts: attempt,
            err,
          });
          return this.deadLetter(msg, err);
        }
        this.logger.warn({ msg: 'event failed, retrying', ...context, attempt, err });
        await this.sleep(backoff);
        backoff *= 2;
      }
    }
  }

  private async deadLetter(msg: ConsumedMessage, cause: unknown): Promise<void> {
    const reason = (cause instanceof Error ? cause.message : String(cause)).slice(0, 1000);
    const record = {
      topic: dlqTopic(msg.topic),
      messages: [
        {
          key: msg.key,
          value: msg.value,
          headers: {
            [HEADER_ERROR]: reason,
            [HEADER_ORIGINAL_TOPIC]: msg.topic,
            [HEADER_ORIGINAL_PARTITION]: String(msg.partition),
            [HEADER_ORIGINAL_OFFSET]: msg.offset,
            [HEADER_CONSUMER_GROUP]: this.options.group,
          },
        },
      ],
    };
    let backoff = this.backoffMs;
    for (;;) {
      try {
        await this.dlq.send(record);
        return;
      } catch (err) {
        this.throwIfStopped();
        this.logger.error({ msg: 'dead letter publish failed', topic: record.topic, err });
        await this.sleep(backoff);
        backoff = Math.min(backoff * 2, 30_000);
      }
    }
  }

  private throwIfStopped(): void {
    if (this.abort.signal.aborted) {
      throw new ConsumerStoppedError();
    }
  }

  private sleep(ms: number): Promise<void> {
    return new Promise((resolve, reject) => {
      const signal = this.abort.signal;
      if (signal.aborted) {
        reject(new ConsumerStoppedError());
        return;
      }
      const timer = setTimeout(() => {
        signal.removeEventListener('abort', onAbort);
        resolve();
      }, ms);
      const onAbort = (): void => {
        clearTimeout(timer);
        reject(new ConsumerStoppedError());
      };
      signal.addEventListener('abort', onAbort, { once: true });
    });
  }
}

/**
 * Runs a consumer group member in the background. A broker outage never stops
 * the service: connection failures are retried until stop().
 */
export class KafkaEventConsumer {
  private readonly logger: Logger;
  private readonly kafka: Kafka;
  private consumer: Consumer | undefined;
  private producer: Producer | undefined;
  private processor: EventProcessor | undefined;
  private stopped = false;
  private running: Promise<void> | undefined;

  constructor(
    brokers: string[],
    private readonly options: ConsumerOptions,
    private readonly handler: EventHandler,
    private readonly retryDelayMs = 5_000,
  ) {
    this.logger = new Logger(`KafkaConsumer:${options.group}`);
    this.kafka = new Kafka({
      clientId: options.group,
      brokers,
      logLevel: logLevel.WARN,
      logCreator: () => (entry) => {
        this.logger.warn({ msg: `kafkajs: ${entry.log.message}`, namespace: entry.namespace });
      },
    });
  }

  start(): void {
    this.running ??= this.loop();
  }

  async stop(): Promise<void> {
    this.stopped = true;
    this.processor?.stop();
    await this.disconnect();
    await this.running;
  }

  private async loop(): Promise<void> {
    while (!this.stopped) {
      try {
        await this.connect();
        return;
      } catch (err) {
        this.logger.warn({ msg: 'kafka consumer not started, retrying', err });
        await this.disconnect();
        await new Promise((resolve) => setTimeout(resolve, this.retryDelayMs));
      }
    }
  }

  private async connect(): Promise<void> {
    const producer = this.kafka.producer({ allowAutoTopicCreation: false, idempotent: true });
    const consumer = this.kafka.consumer({
      groupId: this.options.group,
      allowAutoTopicCreation: false,
    });
    this.producer = producer;
    this.consumer = consumer;
    const processor = new EventProcessor(this.options, this.handler, producer, this.logger);
    this.processor = processor;

    await producer.connect();
    await consumer.connect();
    await consumer.subscribe({ topics: this.options.topics, fromBeginning: true });
    consumer.on(consumer.events.CRASH, (event) => {
      if (!event.payload.restart && !this.stopped) {
        this.logger.error({ msg: 'kafka consumer crashed, restarting', err: event.payload.error });
        this.running = this.disconnect().then(() => this.loop());
      }
    });
    await consumer.run({
      autoCommit: true,
      eachMessage: ({ topic, partition, message }) =>
        processor.process({
          topic,
          partition,
          offset: message.offset,
          key: message.key,
          value: message.value,
        }),
    });
    this.logger.log({ msg: 'kafka consumer started', topics: this.options.topics });
  }

  private async disconnect(): Promise<void> {
    const { consumer, producer } = this;
    this.consumer = undefined;
    this.producer = undefined;
    await consumer?.disconnect().catch(() => undefined);
    await producer?.disconnect().catch(() => undefined);
  }
}
