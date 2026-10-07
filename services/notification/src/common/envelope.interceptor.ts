import { CallHandler, ExecutionContext, Injectable, NestInterceptor } from '@nestjs/common';
import { Observable, map } from 'rxjs';

import { PoroRequest, requestIdOf } from './request';

export interface SuccessEnvelope<T> {
  data: T;
  error: null;
  meta: { request_id: string };
}

/** Wraps every controller result in the {data, error, meta} envelope. */
@Injectable()
export class EnvelopeInterceptor implements NestInterceptor {
  intercept<T>(context: ExecutionContext, next: CallHandler<T>): Observable<SuccessEnvelope<T>> {
    const req = context.switchToHttp().getRequest<PoroRequest>();
    return next.handle().pipe(
      map((data) => ({
        data,
        error: null,
        meta: { request_id: requestIdOf(req) },
      })),
    );
  }
}
