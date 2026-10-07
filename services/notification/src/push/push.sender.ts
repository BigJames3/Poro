import { cert, deleteApp, getApps, initializeApp } from 'firebase-admin/app';
import { getMessaging, type BatchResponse, type BaseMessage } from 'firebase-admin/messaging';

import type { FcmCredentials } from '../config/app.config';

export const PUSH_SENDER = Symbol('PUSH_SENDER');

export interface PushMessage {
  title: string;
  body: string;
  /** FCM data values must be strings. */
  data: Record<string, string>;
}

export interface PushResult {
  sent: number;
  failed: number;
  /** Tokens FCM will never accept again: the app was uninstalled or the token rotated. */
  invalidTokens: string[];
}

export interface PushSender {
  readonly enabled: boolean;
  send(tokens: string[], message: PushMessage): Promise<PushResult>;
  close(): Promise<void>;
}

/**
 * A multicast to FCM registration tokens, the identifiers the Android SDK
 * hands out today. firebase-admin 14 marks them deprecated in favour of
 * installation IDs; moving to FIDs needs a client change first.
 */
export interface TokenMulticast extends BaseMessage {
  tokens: string[];
}

/** The part of firebase-admin Messaging this service uses. */
export interface MulticastClient {
  sendEachForMulticast(message: TokenMulticast): Promise<BatchResponse>;
}

// Errors that condemn the token itself. Others (quota, unavailable, a payload
// error) say nothing about the token, so it is kept.
const INVALID_TOKEN_CODES = ['registration-token-not-registered', 'invalid-registration-token'];

function isInvalidToken(code: string | undefined): boolean {
  return INVALID_TOKEN_CODES.some((invalid) => code?.endsWith(invalid) === true);
}

export class FcmPushSender implements PushSender {
  readonly enabled = true;

  constructor(
    private readonly client: MulticastClient,
    private readonly onClose: () => Promise<void> = () => Promise.resolve(),
  ) {}

  async send(tokens: string[], message: PushMessage): Promise<PushResult> {
    if (tokens.length === 0) {
      return { sent: 0, failed: 0, invalidTokens: [] };
    }
    const response = await this.client.sendEachForMulticast({
      tokens,
      notification: { title: message.title, body: message.body },
      data: message.data,
      android: { priority: 'high' },
      apns: { payload: { aps: { sound: 'default' } } },
    });
    const invalidTokens = response.responses.flatMap((res, i) =>
      !res.success && isInvalidToken(res.error?.code) ? [tokens[i]] : [],
    );
    return {
      sent: response.successCount,
      failed: response.failureCount,
      invalidTokens,
    };
  }

  close(): Promise<void> {
    return this.onClose();
  }
}

/** Used when FCM_CREDENTIALS_B64 is unset: only in-app notifications are produced. */
export class DisabledPushSender implements PushSender {
  readonly enabled = false;

  send(): Promise<PushResult> {
    return Promise.resolve({ sent: 0, failed: 0, invalidTokens: [] });
  }

  close(): Promise<void> {
    return Promise.resolve();
  }
}

const APP_NAME = 'poro-notification';

export function pushSenderFor(credentials: FcmCredentials | undefined): PushSender {
  if (credentials === undefined) {
    return new DisabledPushSender();
  }
  const existing = getApps().find((app) => app.name === APP_NAME);
  const app =
    existing ??
    initializeApp(
      {
        credential: cert({
          projectId: credentials.projectId,
          clientEmail: credentials.clientEmail,
          privateKey: credentials.privateKey,
        }),
        projectId: credentials.projectId,
      },
      APP_NAME,
    );
  return new FcmPushSender(getMessaging(app), () => deleteApp(app));
}
