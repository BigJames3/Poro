import { HealthController } from './health.controller';
import { PrismaService } from '../prisma/prisma.service';

interface Captured {
  statusCode?: number;
  body?: unknown;
}

function res(captured: Captured): {
  status: (code: number) => { json: (body: unknown) => void };
  json: (body: unknown) => void;
} {
  return {
    json: (body: unknown) => {
      captured.statusCode = 200;
      captured.body = body;
    },
    status: (code: number) => ({
      json: (body: unknown) => {
        captured.statusCode = code;
        captured.body = body;
      },
    }),
  };
}

describe('HealthController', () => {
  it('live never checks dependencies', () => {
    const captured: Captured = {};
    const ping = jest.fn();
    const prisma = { ping } as unknown as PrismaService;
    new HealthController(prisma).live(res(captured) as never);
    expect(captured.body).toMatchObject({ status: 'ok', service: 'order' });
    expect(ping).not.toHaveBeenCalled();
  });

  it('ready is degraded when postgres is down', async () => {
    const captured: Captured = {};
    const prisma = { ping: () => Promise.reject(new Error('down')) } as unknown as PrismaService;
    const redis = { ping: () => Promise.resolve('PONG') };
    await new HealthController(prisma, redis as never).ready(res(captured) as never);
    expect(captured.statusCode).toBe(503);
    expect(captured.body).toMatchObject({
      status: 'degraded',
      checks: { postgres: 'down', redis: 'up' },
    });
  });
});
