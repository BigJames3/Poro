import { isUuid } from '../common/uuid';
import { PermanentError } from './event-consumer';

/** Readers for event data: a missing or malformed field dead-letters the event. */
export type Data = Record<string, unknown>;

export function uuidField(data: Data, field: string): string {
  const value = data[field];
  if (!isUuid(value)) {
    throw new PermanentError(`${field} must be a UUID`);
  }
  return value.toLowerCase();
}

export function stringField(data: Data, field: string): string {
  const value = data[field];
  if (typeof value !== 'string' || value === '') {
    throw new PermanentError(`${field} is required`);
  }
  return value;
}

export function optionalString(data: Data, field: string): string | null {
  const value = data[field];
  return typeof value === 'string' && value !== '' ? value : null;
}

export function amountField(data: Data, field: string): bigint {
  const value = data[field];
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new PermanentError(`${field} must be a non-negative integer`);
  }
  return BigInt(value);
}

export function timeField(data: Data, field: string): Date {
  const value = data[field];
  const at = typeof value === 'string' ? new Date(value) : undefined;
  if (!at || Number.isNaN(at.getTime())) {
    throw new PermanentError(`${field} must be a date-time`);
  }
  return at;
}

export function arrayField(data: Data, field: string): Data[] {
  const value = data[field];
  if (!Array.isArray(value) || !value.every((item) => typeof item === 'object' && item !== null)) {
    throw new PermanentError(`${field} must be an array of objects`);
  }
  return value as Data[];
}
