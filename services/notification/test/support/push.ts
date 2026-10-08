import type { PushMessage, PushResult, PushSender } from '../../src/push/push.sender';

export interface SentPush {
  tokens: string[];
  message: PushMessage;
}

/** Records pushes instead of calling FCM. */
export class FakePushSender implements PushSender {
  readonly enabled = true;
  sent: SentPush[] = [];
  invalid = new Set<string>();
  failWith: Error | undefined;

  send(tokens: string[], message: PushMessage): Promise<PushResult> {
    if (this.failWith !== undefined) {
      return Promise.reject(this.failWith);
    }
    this.sent.push({ tokens, message });
    const invalidTokens = tokens.filter((token) => this.invalid.has(token));
    return Promise.resolve({
      sent: tokens.length - invalidTokens.length,
      failed: invalidTokens.length,
      invalidTokens,
    });
  }

  close(): Promise<void> {
    return Promise.resolve();
  }

  reset(): void {
    this.sent = [];
    this.invalid.clear();
    this.failWith = undefined;
  }
}
