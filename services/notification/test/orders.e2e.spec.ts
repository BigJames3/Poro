import { NestExpressApplication } from '@nestjs/platform-express';
import { Client } from 'pg';

import { uuidv7 } from '../src/common/uuid';
import { newEnvelope, type Envelope } from '../src/events/envelope';
import { PermanentError } from '../src/events/event-consumer';
import { NotificationDispatcher } from '../src/notifications/dispatcher';
import { createApp } from './support/app';
import { TestDatabase, startDatabase } from './support/postgres';
import { FakePushSender } from './support/push';

interface Row {
  id: string;
  user_id: string;
  type: string;
  title: string;
  body: string;
  data: Record<string, string>;
  actor_id: string | null;
  entity_type: string;
  entity_id: string;
  video_id: string | null;
  group_key: string;
}

interface Order {
  order_id: string;
  buyer_id: string;
  shop_id: string;
  seller_id: string;
}

const FCFA = (amount: string) => `${amount.replace(' ', '\u202f')}\u00a0FCFA`;

describe('order notifications', () => {
  let db: TestDatabase;
  let pg: Client;
  let app: NestExpressApplication;
  let dispatcher: NotificationDispatcher;
  const push = new FakePushSender();

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    app = await createApp({ DATABASE_URL: db.url }, push);
    dispatcher = app.get(NotificationDispatcher);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  beforeEach(() => {
    push.reset();
  });

  const event = (type: string, data: object, subject = uuidv7()): Envelope =>
    newEnvelope(type, 1, subject, data);

  const notifications = (userId: string): Promise<Row[]> =>
    pg
      .query<Row>('SELECT * FROM notifications WHERE user_id = $1 ORDER BY id', [userId])
      .then((r) => r.rows);

  const newOrder = (): Order => ({
    order_id: uuidv7(),
    buyer_id: uuidv7(),
    shop_id: uuidv7(),
    seller_id: uuidv7(),
  });

  function shopUpdated(order: Order, name: string, updatedAt = new Date()): Envelope {
    return event(
      'poro.shop.shop.updated',
      {
        shop_id: order.shop_id,
        owner_id: order.seller_id,
        name,
        handle: 'chez.fatou',
        description: null,
        logo_url: null,
        country_code: 'CI',
        currency: 'XOF',
        status: 'active',
        updated_at: updatedAt.toISOString(),
      },
      order.shop_id,
    );
  }

  async function knownParties(order: Order): Promise<void> {
    await dispatcher.handle(shopUpdated(order, 'Chez Fatou'));
    await dispatcher.handle(
      event('poro.user.profile.updated', {
        user_id: order.buyer_id,
        username: 'awa',
        display_name: 'Awa',
        avatar_url: null,
        is_creator: false,
        updated_at: new Date().toISOString(),
      }),
    );
  }

  async function device(userId: string, token: string) {
    await pg.query(
      `INSERT INTO devices (id, user_id, fcm_token, platform, updated_at)
       VALUES ($1, $2, $3, 'android', now())`,
      [uuidv7(), userId, token],
    );
  }

  const placed = (order: Order, extra: object = {}) =>
    event(
      'poro.order.order.placed',
      {
        ...order,
        currency: 'XOF',
        total: 12500,
        payment_method: 'cash_on_delivery',
        placed_at: new Date().toISOString(),
        expires_at: null,
        ...extra,
      },
      order.order_id,
    );

  const shipped = (order: Order, tracking: string | null) =>
    event(
      'poro.order.order.shipped',
      { ...order, tracking, shipped_at: new Date().toISOString() },
      order.order_id,
    );

  const cancelled = (order: Order, reason: string, paid = false) =>
    event(
      'poro.order.order.cancelled',
      { ...order, reason, paid, cancelled_at: new Date().toISOString() },
      order.order_id,
    );

  const completed = (order: Order) =>
    event(
      'poro.order.order.completed',
      {
        ...order,
        currency: 'XOF',
        total: 12500,
        payment_method: 'cash_on_delivery',
        completed_at: new Date().toISOString(),
      },
      order.order_id,
    );

  it('tells the seller of a new order and the buyer of its confirmation', async () => {
    const order = newOrder();
    await knownParties(order);
    await device(order.seller_id, 'token-seller-0000000000001');
    await device(order.buyer_id, 'token-buyer-00000000000001');

    await dispatcher.handle(placed(order));

    const [sellerRow] = await notifications(order.seller_id);
    expect(sellerRow).toMatchObject({
      type: 'order_new',
      title: 'Nouvelle commande',
      body: `Awa a passé une commande de ${FCFA('12 500')}, payée à la livraison.`,
      actor_id: order.buyer_id,
      entity_type: 'order',
      entity_id: order.order_id,
      video_id: null,
      group_key: `order:${order.order_id}:placed`,
      data: { order_id: order.order_id, role: 'seller' },
    });
    const [buyerRow] = await notifications(order.buyer_id);
    expect(buyerRow).toMatchObject({
      type: 'order_confirmed',
      title: 'Commande confirmée',
      body: `Ta commande chez Chez Fatou (${FCFA('12 500')}) est confirmée. Tu paieras à la livraison.`,
      actor_id: null,
      entity_type: 'order',
      data: { order_id: order.order_id, role: 'buyer' },
    });
    expect(push.sent).toEqual([
      {
        tokens: ['token-seller-0000000000001'],
        message: {
          title: 'Nouvelle commande',
          body: sellerRow.body,
          data: {
            order_id: order.order_id,
            role: 'seller',
            notification_id: sellerRow.id,
            type: 'order_new',
          },
        },
      },
      {
        tokens: ['token-buyer-00000000000001'],
        message: {
          title: 'Commande confirmée',
          body: buyerRow.body,
          data: {
            order_id: order.order_id,
            role: 'buyer',
            notification_id: buyerRow.id,
            type: 'order_confirmed',
          },
        },
      },
    ]);
  });

  it('falls back on placeholders before the shop and the buyer are known', async () => {
    const order = newOrder();
    await dispatcher.handle(placed(order, { currency: 'NGN', total: 1250050 }));

    expect((await notifications(order.seller_id))[0].body).toBe(
      'Quelqu’un a passé une commande de 12\u202f500,50\u00a0₦, payée à la livraison.',
    );
    expect((await notifications(order.buyer_id))[0].body).toBe(
      'Ta commande chez la boutique (12\u202f500,50\u00a0₦) est confirmée. Tu paieras à la livraison.',
    );
  });

  it('waits for the payment of an order not paid on delivery', async () => {
    const order = newOrder();
    await dispatcher.handle(
      placed(order, {
        payment_method: 'wave',
        expires_at: new Date(Date.now() + 1_800_000).toISOString(),
      }),
    );
    expect(await notifications(order.seller_id)).toEqual([]);
    expect(await notifications(order.buyer_id)).toEqual([]);
    expect(push.sent).toEqual([]);
  });

  it('notifies each step once, even when order republishes it', async () => {
    const order = newOrder();
    await device(order.seller_id, 'token-seller-0000000000002');
    const env = placed(order);

    await dispatcher.handle(env);
    await dispatcher.handle(env);
    await dispatcher.handle(placed(order));

    expect(await notifications(order.seller_id)).toHaveLength(1);
    expect(await notifications(order.buyer_id)).toHaveLength(1);
    expect(push.sent).toHaveLength(1);
  });

  it('tells the buyer the order shipped, with the tracking when given', async () => {
    const order = newOrder();
    await knownParties(order);

    await dispatcher.handle(shipped(order, 'Livreur Moussa, 07 00 00 00 00'));

    const rows = await notifications(order.buyer_id);
    expect(rows).toHaveLength(1);
    expect(rows[0]).toMatchObject({
      type: 'order_shipped',
      title: 'Commande expédiée',
      body: 'Chez Fatou a expédié ta commande. Suivi : « Livreur Moussa, 07 00 00 00 00 »',
      group_key: `order:${order.order_id}:shipped`,
      data: { order_id: order.order_id, role: 'buyer' },
    });
    expect(await notifications(order.seller_id)).toEqual([]);

    const other = newOrder();
    await dispatcher.handle(shipped(other, null));
    expect((await notifications(other.buyer_id))[0].body).toBe(
      'La boutique a expédié ta commande.',
    );
  });

  it('tells the seller when the buyer cancels, and only them', async () => {
    const order = newOrder();
    await knownParties(order);

    await dispatcher.handle(cancelled(order, 'buyer_cancelled', true));

    expect(await notifications(order.buyer_id)).toEqual([]);
    const [row] = await notifications(order.seller_id);
    expect(row).toMatchObject({
      type: 'order_cancelled',
      title: 'Commande annulée',
      body: 'Awa a annulé sa commande.',
      actor_id: order.buyer_id,
      data: { order_id: order.order_id, role: 'seller' },
    });
  });

  it.each([
    ['seller_cancelled', false, 'Commande annulée', 'Chez Fatou a annulé ta commande.'],
    [
      'seller_cancelled',
      true,
      'Commande annulée',
      'Chez Fatou a annulé ta commande. Ton remboursement est en cours.',
    ],
    [
      'stock_rejected',
      false,
      'Commande refusée',
      'Chez Fatou n’a pas pu valider ta commande : un article n’est plus disponible.',
    ],
    [
      'payment_timeout',
      false,
      'Commande annulée',
      'Ta commande chez Chez Fatou est annulée faute de paiement dans le délai.',
    ],
  ])('tells only the buyer of a %s order (paid: %p)', async (reason, paid, title, body) => {
    const order = newOrder();
    await knownParties(order);

    await dispatcher.handle(cancelled(order, reason, paid));

    expect(await notifications(order.seller_id)).toEqual([]);
    const [row] = await notifications(order.buyer_id);
    expect(row).toMatchObject({
      type: 'order_cancelled',
      title,
      body,
      group_key: `order:${order.order_id}:cancelled`,
      data: { order_id: order.order_id, role: 'buyer' },
    });
  });

  it('tells both parties of an order cancelled by an account erasure, naming neither', async () => {
    const order = newOrder();
    await knownParties(order);

    await dispatcher.handle(cancelled(order, 'account_deleted', true));

    const [buyer] = await notifications(order.buyer_id);
    expect(buyer).toMatchObject({
      type: 'order_cancelled',
      title: 'Commande annulée',
      body: 'Ta commande chez Chez Fatou est annulée suite à la suppression d’un compte. Ton remboursement est en cours.',
      actor_id: null,
      data: { order_id: order.order_id, role: 'buyer' },
    });
    const [seller] = await notifications(order.seller_id);
    expect(seller).toMatchObject({
      type: 'order_cancelled',
      title: 'Commande annulée',
      body: 'Une commande a été annulée suite à la suppression d’un compte.',
      actor_id: null,
      data: { order_id: order.order_id, role: 'seller' },
    });
  });

  it('tells the seller the order is completed', async () => {
    const order = newOrder();
    await knownParties(order);

    await dispatcher.handle(completed(order));

    expect(await notifications(order.buyer_id)).toEqual([]);
    const [row] = await notifications(order.seller_id);
    expect(row).toMatchObject({
      type: 'order_completed',
      title: 'Commande terminée',
      body: `La commande passée par Awa (${FCFA('12 500')}) est terminée.`,
      data: { order_id: order.order_id, role: 'seller' },
    });
  });

  it('keeps order notifications in-app when their push is muted', async () => {
    const order = newOrder();
    await device(order.seller_id, 'token-seller-0000000000003');
    await device(order.buyer_id, 'token-buyer-00000000000003');
    await pg.query(
      `INSERT INTO preferences (user_id, push_enabled, likes, comments, follows, video_ready, orders, updated_at)
       VALUES ($1, true, true, true, true, true, false, now()),
              ($2, false, true, true, true, true, true, now())`,
      [order.seller_id, order.buyer_id],
    );

    await dispatcher.handle(placed(order));

    expect(await notifications(order.seller_id)).toHaveLength(1);
    expect(await notifications(order.buyer_id)).toHaveLength(1);
    expect(push.sent).toEqual([]);
  });

  it('keeps the newest shop snapshot', async () => {
    const order = newOrder();
    const now = Date.now();
    await dispatcher.handle(shopUpdated(order, 'Nouveau nom', new Date(now)));
    await dispatcher.handle(shopUpdated(order, 'Ancien nom', new Date(now - 60_000)));

    await dispatcher.handle(shipped(order, null));

    expect((await notifications(order.buyer_id))[0].body).toBe(
      'Nouveau nom a expédié ta commande.',
    );
    const { rows } = await pg.query<{ name: string; handle: string }>(
      'SELECT name, handle FROM shop_projections WHERE shop_id = $1',
      [order.shop_id],
    );
    expect(rows).toEqual([{ name: 'Nouveau nom', handle: 'chez.fatou' }]);
  });

  it.each([
    ['poro.order.order.placed', { order_id: 'x' }, 'order_id must be a UUID'],
    [
      'poro.order.order.placed',
      { ...newOrder(), currency: 'EUR', total: 100 },
      'currency must be one of XOF, XAF, NGN',
    ],
    [
      'poro.order.order.completed',
      { ...newOrder(), currency: 'XOF', total: -1 },
      'total must be a non-negative integer',
    ],
    [
      'poro.order.order.cancelled',
      { ...newOrder(), reason: 'lost', paid: false },
      'reason must be stock_rejected, payment_timeout, buyer_cancelled, seller_cancelled or account_deleted',
    ],
    [
      'poro.order.order.shipped',
      { ...newOrder(), tracking: 'x'.repeat(201) },
      'tracking must be a string of at most 200 characters',
    ],
    [
      'poro.shop.shop.updated',
      { shop_id: uuidv7(), name: ' ', handle: 'abc', updated_at: new Date().toISOString() },
      'name must be a non-empty string',
    ],
  ])('rejects %s with %p permanently', async (type, data, message) => {
    const err = await dispatcher.handle(event(type, data)).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(PermanentError);
    expect((err as Error).message).toContain(message);
  });
});
