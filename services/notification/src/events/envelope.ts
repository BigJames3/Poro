import { isUuid, uuidv7 } from '../common/uuid';

/**
 * Event envelope shared with github.com/poro/shared-go/events. Additive changes
 * to data keep the version; a breaking change increments it.
 */
export interface Envelope<T = unknown> {
  id: string;
  type: string;
  version: number;
  source: string;
  subject: string;
  occurred_at: string;
  data: T;
}

export const EVENT_SOURCE = 'poro-notification';
// Same rule as shared-go/events: poro.{domain}.{action} or poro.{domain}.{entity}.{action}.
const TYPE_PATTERN = /^poro(\.[a-z][a-z0-9_]*){2,3}$/;

export class InvalidEnvelopeError extends Error {
  constructor(reason: string) {
    super(`invalid event envelope: ${reason}`);
    this.name = 'InvalidEnvelopeError';
  }
}

export function newEnvelope<T extends object>(
  type: string,
  version: number,
  subject: string,
  data: T,
  occurredAt: Date = new Date(),
): Envelope<T> {
  const env: Envelope<T> = {
    id: uuidv7(),
    type,
    version,
    source: EVENT_SOURCE,
    subject,
    occurred_at: occurredAt.toISOString(),
    data,
  };
  validateEnvelope(env);
  return env;
}

export function validateEnvelope(env: Partial<Envelope>): asserts env is Envelope {
  if (!isUuid(env.id)) {
    throw new InvalidEnvelopeError('missing id');
  }
  if (typeof env.type !== 'string' || !TYPE_PATTERN.test(env.type)) {
    throw new InvalidEnvelopeError('type must match poro.{domain}[.{entity}].{action}');
  }
  if (typeof env.version !== 'number' || !Number.isInteger(env.version) || env.version < 1) {
    throw new InvalidEnvelopeError('version must be at least 1');
  }
  if (typeof env.source !== 'string' || env.source === '') {
    throw new InvalidEnvelopeError('missing source');
  }
  if (typeof env.subject !== 'string' || env.subject === '') {
    throw new InvalidEnvelopeError('missing subject');
  }
  if (typeof env.occurred_at !== 'string' || Number.isNaN(Date.parse(env.occurred_at))) {
    throw new InvalidEnvelopeError('missing occurred_at');
  }
  if (typeof env.data !== 'object' || env.data === null || Array.isArray(env.data)) {
    throw new InvalidEnvelopeError('data must be a JSON object');
  }
}

export function decodeEnvelope(value: Buffer | null): Envelope {
  if (value === null) {
    throw new InvalidEnvelopeError('empty message');
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(value.toString('utf8'));
  } catch (err) {
    throw new InvalidEnvelopeError((err as Error).message);
  }
  if (typeof parsed !== 'object' || parsed === null) {
    throw new InvalidEnvelopeError('not a JSON object');
  }
  const env = parsed as Partial<Envelope>;
  validateEnvelope(env);
  return env;
}

export function dlqTopic(topic: string): string {
  return `${topic}.dlq`;
}
