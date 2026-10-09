import type { Prisma } from '../generated/prisma/client';
import { Envelope, validateEnvelope } from './envelope';

type OutboxWriter = Pick<Prisma.TransactionClient, 'outboxEvent'>;

/**
 * Stores env for publication by the outbox relay. Call it with the transaction
 * that changes the state the event describes, so both commit or neither does.
 */
export async function enqueue(tx: OutboxWriter, env: Envelope<object>): Promise<void> {
  validateEnvelope(env);
  await tx.outboxEvent.create({
    data: {
      id: env.id,
      topic: env.type,
      eventKey: env.subject,
      payload: env as unknown as Prisma.InputJsonObject,
      createdAt: new Date(env.occurred_at),
    },
  });
}

type InboxWriter = Pick<Prisma.TransactionClient, 'processedEvent'>;

/**
 * Records eventId for consumer inside the transaction applying its effect.
 * Returns false when the event was already applied; skip the effect then.
 */
export async function claim(tx: InboxWriter, consumer: string, eventId: string): Promise<boolean> {
  const { count } = await tx.processedEvent.createMany({
    data: [{ consumer, eventId }],
    skipDuplicates: true,
  });
  return count === 1;
}
