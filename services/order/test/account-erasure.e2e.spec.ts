import { NestExpressApplication } from '@nestjs/platform-express';
import { Client } from 'pg';

import { AccountErasureConsumer } from '../src/account/account-erasure.consumer';
import { uuidv7 } from '../src/common/uuid';
import { Envelope, newEnvelope } from '../src/events/envelope';
import { PermanentError } from '../src/events/event-consumer';
import { SagaService } from '../src/orders/saga.service';
import { PrismaService } from '../src/prisma/prisma.service';
import { createApp } from './support/app';
import { outboxEvents } from './support/outbox';
import { TestDatabase, startDatabase } from './support/postgres';

const CONTACT = {
  contactName: 'Awa Koné',
  contactPhone: '+2250700000000',
  city: 'Abidjan',
  address: 'Cocody, rue des Jardins',
  landmark: 'Portail vert',
  latitude: 5.35,
  longitude: -4.0,
  locationAccuracyM: 12,
};
const CONTACT_COLUMNS = `contact_name, contact_phone, city, address, landmark, latitude,
  longitude, location_accuracy_m, contact_erased_at IS NOT NULL AS contact_erased`;
const ERASED_CONTACT = {
  contact_name: null,
  contact_phone: null,
  city: null,
  address: null,
  landmark: null,
  latitude: null,
  longitude: null,
  location_accuracy_m: null,
  contact_erased: true,
};
const KEPT_CONTACT = {
  contact_name: 'Awa Koné',
  contact_phone: '+2250700000000',
  city: 'Abidjan',
  address: 'Cocody, rue des Jardins',
  landmark: 'Portail vert',
  latitude: 5.35,
  longitude: -4.0,
  location_accuracy_m: 12,
  contact_erased: false,
};

describe('account erasure', () => {
  let db: TestDatabase;
  let pg: Client;
  let app: NestExpressApplication;
  let prisma: PrismaService;
  let consumer: AccountErasureConsumer;

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: 'http://127.0.0.1:1/jwks.json',
      SCHEDULER_ENABLED: 'false',
    });
    prisma = app.get(PrismaService);
    consumer = app.get(AccountErasureConsumer);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  /** An order of buyerId from the shop of sellerId, with its delivery contact. */
  async function order(
    buyerId: string,
    sellerId: string,
    status: string,
    paid = false,
  ): Promise<string> {
    const checkout = await prisma.checkout.create({
      data: { id: uuidv7(), buyerId, idempotencyKey: uuidv7(), requestHash: 'a'.repeat(64) },
    });
    const now = new Date();
    const created = await prisma.order.create({
      data: {
        id: uuidv7(),
        checkoutId: checkout.id,
        buyerId,
        shopId: uuidv7(),
        shopName: 'Pagnes Awa',
        sellerId,
        currency: 'XOF',
        paymentMethod: paid ? 'wave' : 'cash_on_delivery',
        status,
        subtotalSeen: 30000n,
        total: 30000n,
        paid,
        paidAt: paid ? now : null,
        shippedAt: ['shipped', 'completed'].includes(status) ? now : null,
        completedAt: status === 'completed' ? now : null,
        cancelledAt: status === 'cancelled' ? now : null,
        cancelReason: status === 'cancelled' ? 'buyer_cancelled' : null,
        ...CONTACT,
        lines: {
          create: [
            {
              position: 0,
              productId: uuidv7(),
              variantId: uuidv7(),
              title: 'Pagne wax',
              quantity: 2,
              unitPriceSeen: 15000n,
              unitPrice: 15000n,
            },
          ],
        },
      },
    });
    return created.id;
  }

  /** Address book, cart and an unconfirmed preview of userId. */
  async function personal(userId: string): Promise<void> {
    await prisma.address.create({
      data: {
        id: uuidv7(),
        buyerId: userId,
        fullName: 'Awa Koné',
        phone: '+2250700000000',
        city: 'Abidjan',
        address: 'Cocody, rue des Jardins',
      },
    });
    await prisma.cartItem.create({ data: { buyerId: userId, variantId: uuidv7(), quantity: 1 } });
    await prisma.checkoutPreview.create({
      data: {
        id: uuidv7(),
        buyerId: userId,
        cartHash: 'b'.repeat(64),
        request: { delivery: { full_name: 'Awa Koné', phone: '+2250700000000' } },
        expiresAt: new Date(Date.now() + 600_000),
      },
    });
  }

  async function personalRows(userId: string): Promise<Record<string, number>> {
    const { rows } = await pg.query<Record<string, number>>(
      `SELECT (SELECT count(*)::int FROM addresses WHERE buyer_id = $1) AS addresses,
              (SELECT count(*)::int FROM cart_items WHERE buyer_id = $1) AS cart_items,
              (SELECT count(*)::int FROM checkout_previews WHERE buyer_id = $1) AS previews,
              (SELECT count(*)::int FROM checkouts WHERE buyer_id = $1) AS checkouts`,
      [userId],
    );
    return rows[0];
  }

  async function orderRow(orderId: string): Promise<Record<string, unknown>> {
    const { rows } = await pg.query(
      `SELECT status, cancel_reason, paid, total::int, ${CONTACT_COLUMNS},
              (SELECT count(*)::int FROM order_lines WHERE order_id = o.id) AS lines
         FROM orders o WHERE id = $1`,
      [orderId],
    );
    return rows[0] as Record<string, unknown>;
  }

  async function outboxCount(): Promise<number> {
    const { rows } = await pg.query<{ n: number }>('SELECT count(*)::int AS n FROM outbox_events');
    return rows[0].n;
  }

  function deleted(userId: string): Envelope {
    return newEnvelope('poro.auth.user.deleted', 1, userId, {
      user_id: userId,
      deleted_at: new Date().toISOString(),
    });
  }

  it('cancels unshipped orders, erases contacts and deletes the address book', async () => {
    const user = uuidv7();
    const seller = uuidv7();
    const otherBuyer = uuidv7();
    await personal(user);
    await personal(otherBuyer);
    const pending = await order(user, seller, 'pending');
    const awaiting = await order(user, seller, 'awaiting_payment');
    const paid = await order(user, seller, 'paid', true);
    const shipped = await order(user, seller, 'shipped', true);
    const completed = await order(user, seller, 'completed', true);
    const cancelled = await order(user, seller, 'cancelled');
    // The account also sells: its buyers' orders not shipped yet end too.
    const sold = await order(otherBuyer, user, 'confirmed');
    const soldShipped = await order(otherBuyer, user, 'shipped');
    const unrelated = await order(otherBuyer, seller, 'paid', true);
    const before = await outboxCount();
    const env = deleted(user);

    await consumer.handle(env);

    expect(await personalRows(user)).toEqual({
      addresses: 0,
      cart_items: 0,
      previews: 0,
      checkouts: 6,
    });
    expect(await personalRows(otherBuyer)).toEqual({
      addresses: 1,
      cart_items: 1,
      previews: 1,
      checkouts: 3,
    });

    const kept = { total: 30000, lines: 1 };
    for (const id of [pending, awaiting]) {
      expect(await orderRow(id)).toEqual({
        status: 'cancelled',
        cancel_reason: 'account_deleted',
        paid: false,
        ...kept,
        ...ERASED_CONTACT,
      });
    }
    expect(await orderRow(paid)).toEqual({
      status: 'cancelled',
      cancel_reason: 'account_deleted',
      paid: true,
      ...kept,
      ...ERASED_CONTACT,
    });
    expect(await orderRow(shipped)).toEqual({
      status: 'shipped',
      cancel_reason: null,
      paid: true,
      ...kept,
      ...KEPT_CONTACT,
    });
    expect(await orderRow(completed)).toEqual({
      status: 'completed',
      cancel_reason: null,
      paid: true,
      ...kept,
      ...ERASED_CONTACT,
    });
    expect(await orderRow(cancelled)).toEqual({
      status: 'cancelled',
      cancel_reason: 'buyer_cancelled',
      paid: false,
      ...kept,
      ...ERASED_CONTACT,
    });
    // The contact there is the other buyer's: it follows the usual retention.
    expect(await orderRow(sold)).toEqual({
      status: 'cancelled',
      cancel_reason: 'account_deleted',
      paid: false,
      ...kept,
      ...KEPT_CONTACT,
    });
    expect(await orderRow(soldShipped)).toMatchObject({ status: 'shipped', ...KEPT_CONTACT });
    expect(await orderRow(unrelated)).toMatchObject({
      status: 'paid',
      cancel_reason: null,
      ...KEPT_CONTACT,
    });

    const cancellations = new Map([
      [pending, false],
      [awaiting, false],
      [paid, true],
      [sold, false],
    ]);
    for (const [id, wasPaid] of cancellations) {
      expect(await outboxEvents(pg, 'poro.order.order.cancelled', id)).toEqual([
        {
          order_id: id,
          buyer_id: id === sold ? otherBuyer : user,
          shop_id: expect.any(String),
          seller_id: id === sold ? user : seller,
          reason: 'account_deleted',
          paid: wasPaid,
          cancelled_at: expect.any(String),
        },
      ]);
    }
    expect(await outboxCount()).toBe(before + cancellations.size);
  });

  it('lets a shipped order complete, then the retention task erases its contact', async () => {
    const user = uuidv7();
    const shipped = await order(user, uuidv7(), 'shipped', true);
    await consumer.handle(deleted(user));
    expect(await orderRow(shipped)).toMatchObject({ status: 'shipped', ...KEPT_CONTACT });

    const saga = app.get(SagaService);
    const later = new Date(Date.now() + 365 * 86_400_000);
    await saga.autoComplete(later);
    await saga.eraseContacts(new Date(later.getTime() + 365 * 86_400_000));

    expect(await orderRow(shipped)).toMatchObject({ status: 'completed', ...ERASED_CONTACT });
  });

  it('applies an event once and a second erasure of the account changes nothing', async () => {
    const user = uuidv7();
    await personal(user);
    const pending = await order(user, uuidv7(), 'pending');
    const env = deleted(user);
    await consumer.handle(env);
    const published = await outboxCount();
    const row = await orderRow(pending);

    await consumer.handle(env);
    const again = deleted(user);
    await consumer.handle(again);

    expect(await outboxCount()).toBe(published);
    expect(await outboxEvents(pg, 'poro.order.order.cancelled', pending)).toHaveLength(1);
    expect(await orderRow(pending)).toEqual(row);
    const { rows } = await pg.query(
      `SELECT event_id FROM processed_events WHERE consumer = 'poro-order-account-erasure'
          AND event_id = ANY($1::uuid[])`,
      [[env.id, again.id]],
    );
    expect(rows).toHaveLength(2);
  });

  it.each([
    ['another type', { type: 'poro.auth.user.created' }],
    ['another version', { version: 2 }],
    [
      'a user_id that is not a UUID',
      { data: { user_id: 'x', deleted_at: '2026-10-15T00:00:00Z' } },
    ],
  ])('dead-letters %s before any change', async (_name, overrides) => {
    const user = uuidv7();
    await personal(user);
    const pending = await order(user, uuidv7(), 'pending');
    const env = { ...deleted(user), ...overrides } as Envelope;

    await expect(consumer.handle(env)).rejects.toBeInstanceOf(PermanentError);

    expect(await personalRows(user)).toEqual({
      addresses: 1,
      cart_items: 1,
      previews: 1,
      checkouts: 1,
    });
    expect(await orderRow(pending)).toMatchObject({ status: 'pending', ...KEPT_CONTACT });
    const { rows } = await pg.query('SELECT 1 FROM processed_events WHERE event_id = $1', [env.id]);
    expect(rows).toHaveLength(0);
  });
});
