import { Client } from 'pg';

/** Payloads of the outbox events of topic for subject, oldest first. */
export async function outboxEvents(
  pg: Client,
  topic: string,
  subject: string,
): Promise<Record<string, unknown>[]> {
  const { rows } = await pg.query<{ payload: { data: Record<string, unknown> } }>(
    `SELECT payload FROM outbox_events WHERE topic = $1 AND event_key = $2 ORDER BY created_at, id`,
    [topic, subject],
  );
  return rows.map((row) => row.payload.data);
}
