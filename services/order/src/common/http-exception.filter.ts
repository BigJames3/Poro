import { ArgumentsHost, Catch, ExceptionFilter, HttpException, Logger } from '@nestjs/common';
import type { NextFunction, Request, Response } from 'express';

import { ApiError, codeForStatus, genericMessage } from './api-error';
import { PoroRequest, requestIdOf } from './request';

interface BodyParserError {
  type: string;
  status: number;
}

function isBodyParserError(err: unknown): err is BodyParserError {
  return (
    typeof err === 'object' &&
    err !== null &&
    typeof (err as BodyParserError).type === 'string' &&
    typeof (err as BodyParserError).status === 'number'
  );
}

/**
 * Mount right after the body parser. Nest turns any SyntaxError into a plain
 * 400 before filters run, so malformed JSON must become an ApiError first.
 */
export function bodyParserErrors(
  err: unknown,
  _req: Request,
  _res: Response,
  next: NextFunction,
): void {
  if (!isBodyParserError(err) || err.status < 400 || err.status >= 500) {
    next(err);
    return;
  }
  if (err.type === 'entity.parse.failed') {
    next(new ApiError(400, 'invalid_json', 'invalid json'));
    return;
  }
  const code = codeForStatus(err.status);
  next(new ApiError(err.status, code, genericMessage(code)));
}

/**
 * Renders every error in the {data, error, meta} envelope. Internal errors are
 * logged with detail and answered with a generic message.
 */
@Catch()
export class HttpExceptionFilter implements ExceptionFilter {
  private readonly logger = new Logger('HttpExceptionFilter');

  catch(err: unknown, host: ArgumentsHost): void {
    const ctx = host.switchToHttp();
    const req = ctx.getRequest<PoroRequest>();
    const res = ctx.getResponse<Response>();

    let status = 500;
    let code = 'internal_error';
    let message = genericMessage(code);
    if (err instanceof ApiError) {
      status = err.getStatus();
      code = err.code;
      message = err.message;
    } else if (err instanceof HttpException) {
      status = err.getStatus();
      code = codeForStatus(status);
      message = genericMessage(code);
    }

    if (status >= 500) {
      this.logger.error({
        msg: 'request failed',
        request_id: requestIdOf(req),
        method: req.method,
        path: req.path,
        err: err instanceof ApiError && err.cause !== undefined ? err.cause : err,
      });
    }
    if (res.headersSent) {
      return;
    }
    res.status(status).json({
      data: null,
      error: { code, message },
      meta: { request_id: requestIdOf(req) },
    });
  }
}
