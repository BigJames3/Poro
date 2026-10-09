import type { NextFunction, Request, Response } from 'express';

import { uuidv7 } from './uuid';

export const HEADER_REQUEST_ID = 'x-request-id';
const MAX_REQUEST_ID_LENGTH = 128;
const REQUEST_ID_PATTERN = /^[A-Za-z0-9._-]+$/;

/** Verified access token claims. */
export interface AuthUser {
  userId: string;
  roles: string[];
  sessionId: string;
  tokenId: string;
}

/** req.id is declared by pino-http and set by requestId(). */
export interface PoroRequest extends Request {
  user?: AuthUser;
}

export function isValidRequestId(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    value.length > 0 &&
    value.length <= MAX_REQUEST_ID_LENGTH &&
    REQUEST_ID_PATTERN.test(value)
  );
}

/** Reuses a well-formed incoming X-Request-ID or generates a UUIDv7, and echoes it. */
export function requestId(req: PoroRequest, res: Response, next: NextFunction): void {
  const incoming = req.headers[HEADER_REQUEST_ID];
  const id = isValidRequestId(incoming) ? incoming : uuidv7();
  req.id = id;
  res.setHeader(HEADER_REQUEST_ID, id);
  next();
}

export function requestIdOf(req: PoroRequest): string {
  return typeof req.id === 'string' ? req.id : '';
}
