import { NestExpressApplication } from '@nestjs/platform-express';
import { Client } from 'pg';

import { uuidv7 } from '../src/common/uuid';
import { Envelope, newEnvelope } from '../src/events/envelope';
import { PermanentError } from '../src/events/event-consumer';
import { ProductsService } from '../src/products/products.service';
import { ShopsService } from '../src/shops/shops.service';
import { StockConsumer } from '../src/stock/stock.consumer';
import { createApp } from './support/app';
import { outboxEvents } from './support/outbox';
import { TestDatabase, startDatabase } from './support/postgres';

interface Seller {
  ownerId: string;
  shopId: string;
  productId: string;
  variants: string[];
}

describe('stock saga', () => {
  let db: TestDatabase;
  let pg: Client;
  let app: NestExpressApplication;
  let consumer: StockConsumer;
  let handles = 0;

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    app = await createApp({ DATABASE_URL: db.url, JWKS_URL: 'http://127.0.0.1:1/jwks.json' });
    consumer = app.get(StockConsumer);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  async function seller(
    stocks: number[] = [3, 1],
    prices: number[] = [15000, 9000],
  ): Promise<Seller> {
    const ownerId = uuidv7();
    handles += 1;
    const shop = await app
      .get(ShopsService)
      .create(ownerId, { name: 'Pagnes', handle: `stock_shop${handles}`, country_code: 'SN' });
    const product = await app.get(ProductsService).create(ownerId, {
      title: 'Pagne wax',
      variants: stocks.map((stock, i) => ({ title: `V${i}`, price: prices[i], stock })),
    });
    await pg.query(`UPDATE products SET status = 'active' WHERE id = $1`, [product.product_id]);
    return {
      ownerId,
      shopId: shop.shop_id,
      productId: product.product_id,
      variants: product.variants.map((v) => v.variant_id),
    };
  }

  function created(
    s: Seller,
    items: { variant: number; quantity: number; product?: string }[],
    overrides: Record<string, unknown> = {},
  ): Envelope {
    const orderId = uuidv7();
    return newEnvelope('poro.order.order.created', 1, orderId, {
      order_id: orderId,
      buyer_id: uuidv7(),
      shop_id: s.shopId,
      seller_id: s.ownerId,
      currency: 'XOF',
      payment_method: 'wave',
      items: items.map((item) => ({
        product_id: item.product ?? s.productId,
        variant_id: s.variants[item.variant] ?? uuidv7(),
        quantity: item.quantity,
        unit_price: 1,
      })),
      created_at: new Date().toISOString(),
      ...overrides,
    });
  }

  function followUp(type: 'cancelled' | 'completed', order: Envelope, s: Seller): Envelope {
    const data = order.data as { order_id: string; buyer_id: string };
    const now = new Date().toISOString();
    return newEnvelope(`poro.order.order.${type}`, 1, data.order_id, {
      order_id: data.order_id,
      buyer_id: data.buyer_id,
      shop_id: s.shopId,
      seller_id: s.ownerId,
      ...(type === 'cancelled'
        ? { reason: 'buyer_cancelled', paid: false, cancelled_at: now }
        : { currency: 'XOF', total: 1, payment_method: 'wave', completed_at: now }),
    });
  }

  async function stock(variantId: string): Promise<{ on_hand: number; reserved: number }> {
    const { rows } = await pg.query<{ on_hand: number; reserved: number }>(
      'SELECT stock_on_hand AS on_hand, stock_reserved AS reserved FROM variants WHERE id = $1',
      [variantId],
    );
    return rows[0];
  }

  function orderId(env: Envelope): string {
    return (env.data as { order_id: string }).order_id;
  }

  it('holds the stock and answers with authoritative prices', async () => {
    const s = await seller();
    const order = created(s, [
      { variant: 0, quantity: 1 },
      { variant: 1, quantity: 1 },
      { variant: 0, quantity: 1 },
    ]);
    await consumer.handle(order);
    await consumer.handle(order);
    await consumer.handle({ ...order, id: uuidv7() });

    expect(await stock(s.variants[0])).toEqual({ on_hand: 3, reserved: 2 });
    expect(await stock(s.variants[1])).toEqual({ on_hand: 1, reserved: 1 });
    const answers = await outboxEvents(pg, 'poro.shop.stock.reserved', orderId(order));
    expect(answers).toEqual([
      {
        order_id: orderId(order),
        shop_id: s.shopId,
        currency: 'XOF',
        items: [
          {
            product_id: s.productId,
            variant_id: s.variants[0],
            quantity: 2,
            unit_price: 15000,
            title: 'Pagne wax — V0',
          },
          {
            product_id: s.productId,
            variant_id: s.variants[1],
            quantity: 1,
            unit_price: 9000,
            title: 'Pagne wax — V1',
          },
        ],
        subtotal: 39000,
        reserved_at: expect.any(String),
      },
    ]);
    const snapshots = await outboxEvents(pg, 'poro.shop.product.updated', s.productId);
    expect((snapshots.at(-1)?.variants as { in_stock: boolean }[]).map((v) => v.in_stock)).toEqual([
      true,
      false,
    ]);
  });

  it.each([
    [
      'out_of_stock',
      (s: Seller) => created(s, [{ variant: 1, quantity: 2 }]),
      (s: Seller) => [s.variants[1]],
    ],
    [
      'product_unavailable',
      (s: Seller) => created(s, [{ variant: 9, quantity: 1 }]),
      (): unknown[] => [expect.any(String) as unknown],
    ],
    [
      'product_unavailable',
      (s: Seller) => created(s, [{ variant: 0, quantity: 1, product: uuidv7() }]),
      (s: Seller) => [s.variants[0]],
    ],
    [
      'shop_unavailable',
      (s: Seller) => created(s, [{ variant: 0, quantity: 1 }], { seller_id: uuidv7() }),
      () => [],
    ],
    [
      'shop_unavailable',
      (s: Seller) => created(s, [{ variant: 0, quantity: 1 }], { shop_id: uuidv7() }),
      () => [],
    ],
    [
      'currency_mismatch',
      (s: Seller) => created(s, [{ variant: 0, quantity: 1 }], { currency: 'NGN' }),
      () => [],
    ],
  ])('rejects with %s and holds nothing', async (reason, build, ids) => {
    const s = await seller();
    const order = build(s);
    await consumer.handle(order);
    expect(await outboxEvents(pg, 'poro.shop.stock.rejected', orderId(order))).toEqual([
      expect.objectContaining({ reason, variant_ids: ids(s) }),
    ]);
    expect(await stock(s.variants[0])).toEqual({ on_hand: 3, reserved: 0 });
    expect(await outboxEvents(pg, 'poro.shop.stock.reserved', orderId(order))).toEqual([]);
  });

  it('rejects orders of a draft product or a suspended shop', async () => {
    const draft = await seller();
    await pg.query(`UPDATE products SET status = 'draft' WHERE id = $1`, [draft.productId]);
    const suspended = await seller();
    await pg.query(`UPDATE shops SET status = 'suspended' WHERE id = $1`, [suspended.shopId]);
    const a = created(draft, [{ variant: 0, quantity: 1 }]);
    const b = created(suspended, [{ variant: 0, quantity: 1 }]);
    await consumer.handle(a);
    await consumer.handle(b);
    expect((await outboxEvents(pg, 'poro.shop.stock.rejected', orderId(a)))[0].reason).toBe(
      'product_unavailable',
    );
    expect((await outboxEvents(pg, 'poro.shop.stock.rejected', orderId(b)))[0].reason).toBe(
      'shop_unavailable',
    );
  });

  it('releases held stock on cancellation and announces it is back', async () => {
    const s = await seller([1, 1]);
    const order = created(s, [{ variant: 0, quantity: 1 }]);
    await consumer.handle(order);
    expect(await stock(s.variants[0])).toEqual({ on_hand: 1, reserved: 1 });
    const cancel = followUp('cancelled', order, s);
    await consumer.handle(cancel);
    await consumer.handle(cancel);
    expect(await stock(s.variants[0])).toEqual({ on_hand: 1, reserved: 0 });
    const inStock = (await outboxEvents(pg, 'poro.shop.product.updated', s.productId)).map(
      (snap) => (snap.variants as { in_stock: boolean }[])[0].in_stock,
    );
    expect(inStock.slice(-2)).toEqual([false, true]);
    const { rows } = await pg.query('SELECT status, reason FROM reservations WHERE order_id = $1', [
      orderId(order),
    ]);
    expect(rows).toEqual([{ status: 'released', reason: 'buyer_cancelled' }]);
  });

  it('remembers a cancellation that overtook its order', async () => {
    const s = await seller();
    const order = created(s, [{ variant: 0, quantity: 1 }]);
    await consumer.handle(followUp('cancelled', order, s));
    await consumer.handle(order);
    expect(await stock(s.variants[0])).toEqual({ on_hand: 3, reserved: 0 });
    expect(await outboxEvents(pg, 'poro.shop.stock.reserved', orderId(order))).toEqual([]);
    expect(await outboxEvents(pg, 'poro.shop.stock.rejected', orderId(order))).toEqual([]);
  });

  it('takes completed orders out of the stock once', async () => {
    const s = await seller();
    const order = created(s, [{ variant: 0, quantity: 2 }]);
    await consumer.handle(order);
    const done = followUp('completed', order, s);
    await consumer.handle(done);
    await consumer.handle({ ...done, id: uuidv7() });
    expect(await stock(s.variants[0])).toEqual({ on_hand: 1, reserved: 0 });

    const ghost = created(s, [{ variant: 0, quantity: 1 }]);
    await expect(consumer.handle(followUp('completed', ghost, s))).rejects.toBeInstanceOf(
      PermanentError,
    );
    const released = created(s, [{ variant: 0, quantity: 1 }]);
    await consumer.handle(released);
    await consumer.handle(followUp('cancelled', released, s));
    await expect(consumer.handle(followUp('completed', released, s))).rejects.toBeInstanceOf(
      PermanentError,
    );
  });

  it('never sells the last unit twice', async () => {
    const s = await seller([1, 0]);
    const orders = Array.from({ length: 6 }, () => created(s, [{ variant: 0, quantity: 1 }]));
    await Promise.all(orders.map((order) => consumer.handle(order)));
    expect(await stock(s.variants[0])).toEqual({ on_hand: 1, reserved: 1 });
    let reserved = 0;
    let rejected = 0;
    for (const order of orders) {
      reserved += (await outboxEvents(pg, 'poro.shop.stock.reserved', orderId(order))).length;
      rejected += (await outboxEvents(pg, 'poro.shop.stock.rejected', orderId(order))).length;
    }
    expect([reserved, rejected]).toEqual([1, 5]);
  });

  it.each([
    ['an unknown type', { type: 'poro.order.order.placed' }],
    ['an unknown version', { version: 2 }],
    ['a bad order id', { data: { order_id: 'x', shop_id: uuidv7() } }],
    [
      'no lines',
      {
        data: {
          order_id: uuidv7(),
          shop_id: uuidv7(),
          buyer_id: uuidv7(),
          seller_id: uuidv7(),
          currency: 'XOF',
          payment_method: 'wave',
          created_at: 'now',
          items: [],
        },
      },
    ],
    [
      'a quantity of 100',
      {
        data: {
          order_id: uuidv7(),
          shop_id: uuidv7(),
          buyer_id: uuidv7(),
          seller_id: uuidv7(),
          currency: 'XOF',
          payment_method: 'wave',
          created_at: 'now',
          items: [{ product_id: uuidv7(), variant_id: uuidv7(), quantity: 100, unit_price: 1 }],
        },
      },
    ],
    [
      'a line that is not an object',
      {
        data: {
          order_id: uuidv7(),
          shop_id: uuidv7(),
          buyer_id: uuidv7(),
          seller_id: uuidv7(),
          currency: 'XOF',
          payment_method: 'wave',
          created_at: 'now',
          items: [7],
        },
      },
    ],
    [
      'a cancellation without reason',
      { type: 'poro.order.order.cancelled', data: { order_id: uuidv7(), shop_id: uuidv7() } },
    ],
  ])('dead-letters %s', async (_name, patch) => {
    const env = {
      ...newEnvelope('poro.order.order.created', 1, uuidv7(), { order_id: uuidv7() }),
      ...patch,
    } as Envelope;
    await expect(consumer.handle(env)).rejects.toBeInstanceOf(PermanentError);
  });
});
