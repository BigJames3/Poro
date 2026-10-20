import { PrismaPg } from '@prisma/adapter-pg';

import { claim, INBOX_RETENTION_MS, purgeProcessedEvents } from '../src/events/outbox';
import { PrismaClient } from '../src/generated/prisma/client';
import { uuidv7 } from '../src/common/uuid';
import { startDatabase, TestDatabase } from './support/postgres';

describe('processed events purge (PORO-019)', () => {
  let db: TestDatabase;
  let prisma: PrismaClient;

  beforeAll(async () => {
    db = await startDatabase();
    prisma = new PrismaClient({ adapter: new PrismaPg({ connectionString: db.url }) });
  });

  afterAll(async () => {
    await prisma.$disconnect();
    await db.container.stop();
  });

  it('forgets inbox rows older than the retention and keeps the others', async () => {
    const old = uuidv7();
    const recent = uuidv7();
    expect(await claim(prisma, 'poro-test', old)).toBe(true);
    expect(await claim(prisma, 'poro-test', recent)).toBe(true);
    await prisma.$executeRaw`
      UPDATE processed_events SET processed_at = now() - interval '31 days' WHERE event_id = ${old}::uuid`;

    const deleted = await purgeProcessedEvents(prisma, new Date(Date.now() - INBOX_RETENTION_MS));
    expect(deleted).toBe(1);
    expect(await claim(prisma, 'poro-test', recent)).toBe(false);
    expect(await claim(prisma, 'poro-test', old)).toBe(true);
  });
});
