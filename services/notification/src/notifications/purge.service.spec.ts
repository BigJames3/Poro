import type { AppConfigType } from '../config/app.config';
import type { PrismaService } from '../prisma/prisma.service';
import { PurgeService } from './purge.service';

describe('PurgeService schedule', () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });
  afterEach(() => {
    jest.useRealTimers();
  });

  function service(purgeEnabled: boolean, executeRaw: jest.Mock) {
    const prisma = { $executeRaw: executeRaw } as unknown as PrismaService;
    return new PurgeService(prisma, { purgeEnabled, retentionDays: 90 } as AppConfigType);
  }

  it('runs a minute after start, then daily, in batches', async () => {
    const executeRaw = jest
      .fn()
      .mockResolvedValueOnce(5_000)
      .mockResolvedValueOnce(3)
      .mockResolvedValue(0);
    const purge = service(true, executeRaw);
    purge.onApplicationBootstrap();
    expect(executeRaw).not.toHaveBeenCalled();
    await jest.advanceTimersByTimeAsync(60_000);
    expect(executeRaw).toHaveBeenCalledTimes(3);
    await jest.advanceTimersByTimeAsync(24 * 60 * 60 * 1000);
    expect(executeRaw).toHaveBeenCalledTimes(5);
    purge.onApplicationShutdown();
  });

  it('logs failures and keeps the schedule', async () => {
    const executeRaw = jest.fn().mockRejectedValue(new Error('db down'));
    const purge = service(true, executeRaw);
    purge.onApplicationBootstrap();
    await jest.advanceTimersByTimeAsync(60_000);
    expect(executeRaw).toHaveBeenCalledTimes(1);
    purge.onApplicationShutdown();
  });

  it('stays idle when disabled', async () => {
    const executeRaw = jest.fn();
    const purge = service(false, executeRaw);
    purge.onApplicationBootstrap();
    await jest.advanceTimersByTimeAsync(2 * 24 * 60 * 60 * 1000);
    expect(executeRaw).not.toHaveBeenCalled();
  });
});
