import {
  CanActivate,
  ExecutionContext,
  Injectable,
  Logger,
  SetMetadata,
  createParamDecorator,
} from '@nestjs/common';
import { Reflector } from '@nestjs/core';

import { ApiError } from '../common/api-error';
import { AuthUser, PoroRequest } from '../common/request';
import { KeysUnavailableError, TokenVerifier } from './token-verifier';

const IS_PUBLIC = 'poro:isPublic';

/** Marks a route that does not require an access token. */
export const Public = (): MethodDecorator & ClassDecorator => SetMetadata(IS_PUBLIC, true);

/** Injects the verified token claims. Only valid on authenticated routes. */
export const CurrentUser = createParamDecorator((_: unknown, ctx: ExecutionContext): AuthUser => {
  const user = ctx.switchToHttp().getRequest<PoroRequest>().user;
  if (!user) {
    throw new ApiError(401, 'unauthorized', 'unauthorized');
  }
  return user;
});

/** Requires a valid auth access token on every route not marked @Public(). */
@Injectable()
export class AuthGuard implements CanActivate {
  private readonly logger = new Logger(AuthGuard.name);

  constructor(
    private readonly verifier: TokenVerifier,
    private readonly reflector: Reflector,
  ) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    const isPublic = this.reflector.getAllAndOverride<boolean | undefined>(IS_PUBLIC, [
      context.getHandler(),
      context.getClass(),
    ]);
    if (isPublic === true) {
      return true;
    }
    const req = context.switchToHttp().getRequest<PoroRequest>();
    const token = bearerToken(req.headers.authorization);
    if (token === undefined) {
      throw new ApiError(401, 'unauthorized', 'unauthorized');
    }
    try {
      req.user = await this.verifier.verify(token);
      return true;
    } catch (err) {
      if (err instanceof KeysUnavailableError) {
        throw new ApiError(503, 'unavailable', 'authentication temporarily unavailable', err);
      }
      this.logger.debug({
        msg: 'token rejected',
        request_id: req.id,
        reason: (err as Error).message,
      });
      throw new ApiError(401, 'unauthorized', 'unauthorized');
    }
  }
}

function bearerToken(header: string | undefined): string | undefined {
  if (header === undefined) {
    return undefined;
  }
  const parts = header.trim().split(/\s+/);
  if (parts.length !== 2 || parts[0].toLowerCase() !== 'bearer' || parts[1] === '') {
    return undefined;
  }
  return parts[1];
}
