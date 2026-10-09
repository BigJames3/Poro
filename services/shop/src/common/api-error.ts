import { HttpException } from '@nestjs/common';

/**
 * An error whose code and message are safe to show to clients.
 * The cause is logged for 5xx responses and never rendered.
 */
export class ApiError extends HttpException {
  constructor(
    status: number,
    readonly code: string,
    message: string,
    cause?: unknown,
  ) {
    super(message, status, cause === undefined ? undefined : { cause });
  }
}

export function codeForStatus(status: number): string {
  switch (status) {
    case 400:
      return 'invalid_request';
    case 401:
      return 'unauthorized';
    case 403:
      return 'forbidden';
    case 404:
      return 'not_found';
    case 405:
      return 'method_not_allowed';
    case 409:
      return 'conflict';
    case 413:
      return 'payload_too_large';
    case 415:
      return 'unsupported_media_type';
    case 429:
      return 'rate_limited';
    case 503:
      return 'unavailable';
    default:
      return status >= 500 ? 'internal_error' : 'error';
  }
}

const GENERIC_MESSAGES: Record<string, string> = {
  invalid_request: 'invalid request',
  unauthorized: 'unauthorized',
  forbidden: 'forbidden',
  not_found: 'not found',
  method_not_allowed: 'method not allowed',
  conflict: 'conflict',
  payload_too_large: 'payload too large',
  unsupported_media_type: 'unsupported media type',
  rate_limited: 'too many requests',
  unavailable: 'service unavailable',
  internal_error: 'internal server error',
  error: 'error',
};

export function genericMessage(code: string): string {
  return GENERIC_MESSAGES[code] ?? 'error';
}
