import type { Prisma } from '../generated/prisma/client';

type InboxWriter = Pick<Prisma.TransactionClient, 'inboxEvent'>;

/**
 * Records eventId for consumer inside the transaction applying its effect.
 * Returns false when the event was already applied; skip the effect then.
 */
export async function claim(tx: InboxWriter, consumer: string, eventId: string): Promise<boolean> {
  const { count } = await tx.inboxEvent.createMany({
    data: [{ consumer, eventId }],
    skipDuplicates: true,
  });
  return count === 1;
}
