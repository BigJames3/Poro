import { ArgumentsHost, BadRequestException, NotFoundException } from '@nestjs/common';
import { ThrottlerException } from '@nestjs/throttler';
import createError from 'http-errors';

import { ApiError } from './api-error';
import { HttpExceptionFilter, bodyParserErrors } from './http-exception.filter';

interface Captured {
  status?: number;
  body?: unknown;
}

function host(captured: Captured, headersSent = false): ArgumentsHost {
  const res = {
    headersSent,
    status(code: number) {
      captured.status = code;
      return this;
    },
    json(body: unknown) {
      captured.body = body;
      return this;
    },
  };
  const req = { id: 'req-1', method: 'GET', path: '/x' };
  return {
    switchToHttp: () => ({ getRequest: () => req, getResponse: () => res }),
  } as unknown as ArgumentsHost;
}

function render(err: unknown): Captured {
  const captured: Captured = {};
  new HttpExceptionFilter().catch(err, host(captured));
  return captured;
}

function parseError(): Error {
  try {
    JSON.parse('{"a":');
  } catch (err) {
    return createError(400, err as Error, { type: 'entity.parse.failed' });
  }
  throw new Error('unreachable');
}

describe('HttpExceptionFilter', () => {
  it.each([
    [
      new ApiError(409, 'username_taken', 'username already taken'),
      409,
      'username_taken',
      'username already taken',
    ],
    [new NotFoundException('Cannot GET /secret/path'), 404, 'not_found', 'not found'],
    [new BadRequestException(), 400, 'invalid_request', 'invalid request'],
    [new ThrottlerException(), 429, 'rate_limited', 'too many requests'],

    [new Error('db password=hunter2'), 500, 'internal_error', 'internal server error'],
    [
      new ApiError(503, 'unavailable', 'storage unavailable', new Error('ECONNREFUSED 10.0.0.3')),
      503,
      'unavailable',
      'storage unavailable',
    ],
    ['a thrown string', 500, 'internal_error', 'internal server error'],
  ])('renders %p as %i %s', (err, status, code, message) => {
    const { status: got, body } = render(err);
    expect(got).toBe(status);
    expect(body).toEqual({
      data: null,
      error: { code, message },
      meta: { request_id: 'req-1' },
    });
    expect(JSON.stringify(body)).not.toMatch(/hunter2|ECONNREFUSED|secret/);
  });

  it.each([
    [parseError(), 400, 'invalid_json', 'invalid json'],
    [
      createError(413, 'request entity too large', {
        type: 'entity.too.large',
      }),
      413,
      'payload_too_large',
      'payload too large',
    ],
    [
      createError(415, 'unsupported charset', { type: 'charset.unsupported' }),
      415,
      'unsupported_media_type',
      'unsupported media type',
    ],
  ])('translates body parser error %p', (err, status, code, message) => {
    let forwarded: unknown;
    bodyParserErrors(err, {} as never, {} as never, (e?: unknown) => {
      forwarded = e;
    });
    expect(render(forwarded)).toEqual({
      status,
      body: {
        data: null,
        error: { code, message },
        meta: { request_id: 'req-1' },
      },
    });
  });

  it('forwards other errors untouched', () => {
    const errors = [
      new Error('boom'),
      createError(500, 'stream error', { type: 'stream.encoding.set' }),
    ];
    for (const err of errors) {
      let forwarded: unknown;
      bodyParserErrors(err, {} as never, {} as never, (e?: unknown) => {
        forwarded = e;
      });
      expect(forwarded).toBe(err);
    }
  });

  it('does not write twice when the response already started', () => {
    const captured: Captured = {};
    new HttpExceptionFilter().catch(new Error('late'), host(captured, true));
    expect(captured).toEqual({});
  });
});
