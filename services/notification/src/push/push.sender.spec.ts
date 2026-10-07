import { generateKeyPairSync } from 'node:crypto';

import type { BatchResponse } from 'firebase-admin/messaging';

import {
  DisabledPushSender,
  FcmPushSender,
  pushSenderFor,
  type TokenMulticast,
} from './push.sender';

const message = { title: 'Nouvel abonné', body: 'Awa a commencé à te suivre', data: { a: '1' } };

function failure(code: string): BatchResponse['responses'][number] {
  return { success: false, error: { code, message: code } as never };
}

describe('FcmPushSender', () => {
  it('sends one multicast and reports tokens FCM rejects for good', async () => {
    const calls: TokenMulticast[] = [];
    const sender = new FcmPushSender({
      sendEachForMulticast: (msg) => {
        calls.push(msg);
        return Promise.resolve({
          successCount: 1,
          failureCount: 3,
          responses: [
            { success: true, messageId: 'm1' },
            failure('messaging/registration-token-not-registered'),
            failure('messaging/invalid-registration-token'),
            failure('messaging/internal-error'),
          ],
        });
      },
    });
    const result = await sender.send(['ok', 'gone', 'bad', 'retry'], message);
    expect(result).toEqual({ sent: 1, failed: 3, invalidTokens: ['gone', 'bad'] });
    expect(calls).toEqual([
      {
        tokens: ['ok', 'gone', 'bad', 'retry'],
        notification: { title: message.title, body: message.body },
        data: message.data,
        android: { priority: 'high' },
        apns: { payload: { aps: { sound: 'default' } } },
      },
    ]);
  });

  it('skips the call without tokens and closes through its hook', async () => {
    const sendEachForMulticast = jest.fn();
    const onClose = jest.fn(() => Promise.resolve());
    const sender = new FcmPushSender({ sendEachForMulticast }, onClose);
    expect(await sender.send([], message)).toEqual({ sent: 0, failed: 0, invalidTokens: [] });
    expect(sendEachForMulticast).not.toHaveBeenCalled();
    await sender.close();
    expect(onClose).toHaveBeenCalled();
    await expect(new FcmPushSender({ sendEachForMulticast }).close()).resolves.toBeUndefined();
  });
});

describe('pushSenderFor', () => {
  it('disables push without credentials', async () => {
    const sender = pushSenderFor(undefined);
    expect(sender).toBeInstanceOf(DisabledPushSender);
    expect(sender.enabled).toBe(false);
    expect(await sender.send(['t'], message)).toEqual({ sent: 0, failed: 0, invalidTokens: [] });
    await expect(sender.close()).resolves.toBeUndefined();
  });

  it('builds an FCM sender from a service account, without network', async () => {
    const { privateKey } = generateKeyPairSync('rsa', { modulusLength: 2048 });
    const credentials = {
      projectId: 'poro-test',
      clientEmail: 'push@poro-test.iam.gserviceaccount.com',
      privateKey: privateKey.export({ type: 'pkcs8', format: 'pem' }).toString(),
    };
    const sender = pushSenderFor(credentials);
    expect(sender).toBeInstanceOf(FcmPushSender);
    expect(sender.enabled).toBe(true);
    // A second build reuses the named app instead of failing on a duplicate.
    expect(pushSenderFor(credentials).enabled).toBe(true);
    await sender.close();
  });
});
