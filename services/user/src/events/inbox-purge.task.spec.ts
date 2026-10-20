import { InboxPurgeTask } from './inbox-purge.task';
import { INBOX_RETENTION_MS } from './outbox';

describe('InboxPurgeTask', () => {
  it('purges rows older than the retention, batch after batch', async () => {
    const calls: Date[] = [];
    const deleted = [5000, 12];
    const prisma = {
      $executeRaw: (_sql: TemplateStringsArray, before: Date) => {
        calls.push(before);
        return Promise.resolve(deleted.shift() ?? 0);
      },
    };
    const now = new Date('2026-10-20T12:00:00Z');
    const task = new InboxPurgeTask(prisma as never);
    await expect(task.run(now)).resolves.toBe(5012);
    expect(calls).toHaveLength(2);
    expect(calls[0]?.getTime()).toBe(now.getTime() - INBOX_RETENTION_MS);
  });

  it('logs a failure and lets the next pass retry', async () => {
    const prisma = { $executeRaw: () => Promise.reject(new Error('db down')) };
    await expect(new InboxPurgeTask(prisma as never).run()).resolves.toBe(0);
  });

  it('runs on a timer that stops with the application', async () => {
    jest.useFakeTimers();
    try {
      let calls = 0;
      const prisma = {
        $executeRaw: () => {
          calls++;
          return Promise.resolve(0);
        },
      };
      const task = new InboxPurgeTask(prisma as never);
      task.onApplicationBootstrap();
      await jest.advanceTimersByTimeAsync(3_600_000);
      expect(calls).toBe(1);
      await task.onApplicationShutdown();
      await jest.advanceTimersByTimeAsync(7_200_000);
      expect(calls).toBe(1);
    } finally {
      jest.useRealTimers();
    }
  });
});
