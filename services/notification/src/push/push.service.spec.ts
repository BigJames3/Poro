import { MetricsService } from '../metrics/metrics';
import type { PrismaService } from '../prisma/prisma.service';
import type { PushSender } from './push.sender';
import { PushService } from './push.service';

const message = { title: 't', body: 'b', data: {} };

function setup(sender: Partial<PushSender>, tokens: string[] = ['a', 'b']) {
  const deleteMany = jest.fn(() => Promise.resolve({ count: 1 }));
  const findMany = jest.fn(() => Promise.resolve(tokens.map((fcmToken) => ({ fcmToken }))));
  const prisma = { device: { findMany, deleteMany } } as unknown as PrismaService;
  const metrics = new MetricsService();
  const service = new PushService(
    prisma,
    { enabled: true, close: () => Promise.resolve(), ...sender } as PushSender,
    metrics,
  );
  return { service, findMany, deleteMany, metrics };
}

async function pushes(metrics: MetricsService): Promise<string> {
  return (await metrics.registry.metrics())
    .split('\n')
    .filter((line) => line.startsWith('notification_pushes_total{'))
    .join('\n');
}

describe('PushService', () => {
  it('does nothing when push is disabled', async () => {
    const { service, findMany } = setup({ enabled: false });
    expect(service.enabled).toBe(false);
    await service.deliver('u', message);
    expect(findMany).not.toHaveBeenCalled();
  });

  it('does not call FCM for an account without devices', async () => {
    const send = jest.fn();
    const { service } = setup({ send }, []);
    await service.deliver('u', message);
    expect(send).not.toHaveBeenCalled();
  });

  it('removes invalid tokens and counts outcomes', async () => {
    const { service, deleteMany, metrics } = setup({
      send: () => Promise.resolve({ sent: 1, failed: 2, invalidTokens: ['b'] }),
    });
    await service.deliver('u', message);
    expect(deleteMany).toHaveBeenCalledWith({ where: { userId: 'u', fcmToken: { in: ['b'] } } });
    const lines = await pushes(metrics);
    expect(lines).toContain('outcome="sent",service="notification"} 1');
    expect(lines).toContain('outcome="failed",service="notification"} 1');
    expect(lines).toContain('outcome="invalid_token",service="notification"} 1');
  });

  it('swallows failures', async () => {
    const { service, metrics } = setup({ send: () => Promise.reject(new Error('down')) });
    await expect(service.deliver('u', message)).resolves.toBeUndefined();
    expect(await pushes(metrics)).toContain('outcome="error",service="notification"} 1');
  });

  it('closes the sender on shutdown, ignoring errors', async () => {
    const { service } = setup({ close: () => Promise.reject(new Error('closed')) });
    await expect(service.onApplicationShutdown()).resolves.toBeUndefined();
  });
});
