import { Inject, Injectable, Logger } from '@nestjs/common';

import { appConfig, type AppConfigType } from '../config/app.config';
import { PHONE_PATTERN } from './delivery';

/** What the buyer's account already knows; null when unknown or unreachable. */
export interface AccountContact {
  fullName: string | null;
  phone: string | null;
  /** ok: both answered; partial: one did; unavailable: neither. */
  lookup: 'ok' | 'partial' | 'unavailable';
}

/**
 * Reads the buyer's own account from auth (phone) and user (display name)
 * with the buyer's token, to prefill the delivery form. Best effort: a slow
 * or failing service only leaves its field empty, it never blocks checkout.
 */
@Injectable()
export class AccountClient {
  private readonly logger = new Logger(AccountClient.name);

  constructor(@Inject(appConfig.KEY) private readonly config: AppConfigType) {}

  async contact(authorization: string): Promise<AccountContact> {
    const [auth, user] = await Promise.all([
      this.read(`${this.config.authUrl}/api/v1/auth/me`, authorization),
      this.read(`${this.config.userUrl}/api/v1/users/me`, authorization),
    ]);
    const phone =
      typeof auth?.phone === 'string' && PHONE_PATTERN.test(auth.phone) ? auth.phone : null;
    const name =
      typeof user?.display_name === 'string' && user.display_name.trim() !== ''
        ? user.display_name.trim().slice(0, 80)
        : null;
    const answered = [auth, user].filter((data) => data !== undefined).length;
    return {
      fullName: name,
      phone,
      lookup: answered === 2 ? 'ok' : answered === 1 ? 'partial' : 'unavailable',
    };
  }

  /** The data of a {data, error, meta} response, or undefined on any failure. */
  private async read(
    url: string,
    authorization: string,
  ): Promise<Record<string, unknown> | undefined> {
    try {
      const res = await fetch(url, {
        headers: { authorization, accept: 'application/json' },
        signal: AbortSignal.timeout(this.config.accountLookupTimeoutMs),
      });
      if (!res.ok) {
        this.logger.warn({ msg: 'account lookup refused', url, status: res.status });
        return undefined;
      }
      const body = (await res.json()) as { data?: unknown };
      return typeof body.data === 'object' && body.data !== null
        ? (body.data as Record<string, unknown>)
        : undefined;
    } catch (err) {
      this.logger.warn({ msg: 'account lookup failed', url, err });
      return undefined;
    }
  }
}
