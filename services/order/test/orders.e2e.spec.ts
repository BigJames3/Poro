import { NestExpressApplication } from '@nestjs/platform-express';
import { Client } from 'pg';
import request from 'supertest';
import type { App } from 'supertest/types';

import { CatalogConsumer } from '../src/catalog/catalog.consumer';
import { uuidv7 } from '../src/common/uuid';
import { Envelope, newEnvelope } from '../src/events/envelope';
import { PermanentError } from '../src/events/event-consumer';
import { SagaConsumer } from '../src/orders/saga.consumer';
import { SagaService } from '../src/orders/saga.service';
import { Scheduler } from '../src/orders/scheduler';
import { Listing, newListing, productUpdated, shopUpdated } from './support/catalog';
import { createApp } from './support/app';
import { JwksServer, SigningKey, signToken, signingKey } from './support/jwt';
import { outboxEvents } from './support/outbox';
import { TestDatabase, startDatabase } from './support/postgres';

const DELIVERY = {
  full_name: 'Awa Koné',
  phone: '+2250700000000',
  city: 'Abidjan',
  address: 'Cocody, rue des Jardins',
  landmark: 'Portail vert après la pharmacie',
};
const DELIVERY_VIEW = { ...DELIVERY, location: null };

interface OrderBody {
  order_id: string;
  status: string;
  total: number | null;
  paid: boolean;
  items: { variant_id: string; unit_price: number; unit_price_seen: number; title: string }[];
  delivery: Record<string, unknown> | null;
  [key: string]: unknown;
}

describe('order service', () => {
  let db: TestDatabase;
  let pg: Client;
  let jwks: JwksServer;
  let key: SigningKey;
  let app: NestExpressApplication;
  let server: App;
  let catalog: CatalogConsumer;
  let saga: SagaConsumer;

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    key = signingKey();
    jwks = new JwksServer();
    jwks.keys = [key.publicJwk];
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: await jwks.start(),
      PAYMENT_METHODS: 'cash_on_delivery,wave',
      SCHEDULER_ENABLED: 'false',
    });
    server = app.getHttpServer();
    catalog = app.get(CatalogConsumer);
    saga = app.get(SagaConsumer);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await jwks.stop();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  async function bearer(userId: string): Promise<string> {
    return `Bearer ${await signToken(key, { sub: userId })}`;
  }

  async function listed(prices: number[] = [15000, 9000]): Promise<Listing> {
    const listing = newListing(prices.length);
    await catalog.handle(shopUpdated(listing));
    await catalog.handle(productUpdated(listing, prices));
    return listing;
  }

  async function addToCart(
    auth: string,
    variantId: string,
    quantity: number,
  ): Promise<request.Response> {
    return request(server)
      .put(`/api/v1/cart/items/${variantId}`)
      .set('Authorization', auth)
      .send({ quantity });
  }

  async function preview(
    auth: string,
    method = 'cash_on_delivery',
    delivery: Record<string, unknown> = DELIVERY,
  ): Promise<request.Response> {
    return request(server)
      .post('/api/v1/checkout/preview')
      .set('Authorization', auth)
      .send({ payment_method: method, delivery });
  }

  async function confirm(auth: string, previewId: string): Promise<request.Response> {
    return request(server)
      .post('/api/v1/checkout/confirm')
      .set('Authorization', auth)
      .send({ preview_id: previewId, confirmed: true });
  }

  /** Reviews the recap then confirms it; returns the failing response if any. */
  async function checkout(auth: string, method = 'cash_on_delivery'): Promise<request.Response> {
    const recap = await preview(auth, method);
    if (recap.status !== 201) {
      return recap;
    }
    return confirm(auth, recap.body.data.preview_id as string);
  }

  /** A buyer with one pending order of listing; returns the order. */
  async function pendingOrder(
    listing: Listing,
    method = 'cash_on_delivery',
  ): Promise<{ buyerId: string; auth: string; order: OrderBody }> {
    const buyerId = uuidv7();
    const auth = await bearer(buyerId);
    expect((await addToCart(auth, listing.variants[0], 2)).status).toBe(200);
    const res = await checkout(auth, method);
    expect(res.status).toBe(201);
    return { buyerId, auth, order: (res.body.data.orders as OrderBody[])[0] };
  }

  function reserved(listing: Listing, orderId: string, unitPrice: number, quantity = 2): Envelope {
    return newEnvelope('poro.shop.stock.reserved', 1, orderId, {
      order_id: orderId,
      shop_id: listing.shopId,
      currency: 'XOF',
      items: [
        {
          product_id: listing.productId,
          variant_id: listing.variants[0],
          quantity,
          unit_price: unitPrice,
          title: 'Pagne wax — Bleu',
        },
      ],
      subtotal: unitPrice * quantity,
      reserved_at: new Date().toISOString(),
    });
  }

  function paymentEvent(
    type: 'succeeded' | 'failed',
    order: { order_id: string },
    buyerId: string,
    amount: number,
  ): Envelope {
    return newEnvelope(`poro.payment.payment.${type}`, 1, uuidv7(), {
      payment_id: uuidv7(),
      order_id: order.order_id,
      buyer_id: buyerId,
      provider: 'wave',
      ...(type === 'succeeded'
        ? { amount, currency: 'XOF', succeeded_at: new Date().toISOString() }
        : { reason: 'declined', failed_at: new Date().toISOString() }),
    });
  }

  async function getOrder(auth: string, orderId: string): Promise<OrderBody> {
    const res = await request(server).get(`/api/v1/orders/${orderId}`).set('Authorization', auth);
    expect(res.status).toBe(200);
    return res.body.data as OrderBody;
  }

  it('requires a token', async () => {
    expect((await request(server).get('/api/v1/cart')).status).toBe(401);
  });

  it('keeps a cart grouped by shop with catalogue prices', async () => {
    const a = await listed([15000, 9000]);
    const b = await listed([2000]);
    const auth = await bearer(uuidv7());
    await addToCart(auth, a.variants[0], 2);
    await addToCart(auth, b.variants[0], 1);
    const res = await addToCart(auth, a.variants[1], 3);
    expect(res.status).toBe(200);
    expect(res.body.data.item_count).toBe(6);
    expect(res.body.data.shops).toHaveLength(2);
    expect(res.body.data.shops[0]).toMatchObject({
      shop: { shop_id: a.shopId, currency: 'XOF' },
      subtotal: 57000,
    });
    expect(res.body.data.shops[0].items[0]).toMatchObject({
      title: 'Pagne wax',
      variant_title: 'V0',
      unit_price: 15000,
      line_total: 30000,
      image_url: 'https://cdn.poro.test/p.webp',
      available: true,
    });

    await catalog.handle(productUpdated(a, [15000, 9000], { status: 'archived' }));
    const after = await request(server).get('/api/v1/cart').set('Authorization', auth);
    expect(after.body.data.shops).toHaveLength(1);
    expect(after.body.data.unavailable).toHaveLength(2);

    await request(server).delete(`/api/v1/cart/items/${b.variants[0]}`).set('Authorization', auth);
    const cleared = await request(server).delete('/api/v1/cart').set('Authorization', auth);
    expect(cleared.body.data).toEqual({ shops: [], unavailable: [], item_count: 0 });
  });

  it('refuses items that are not on sale, own items and bad quantities', async () => {
    const listing = await listed();
    const seller = await bearer(listing.sellerId);
    const own = await addToCart(seller, listing.variants[0], 1);
    expect([own.status, own.body.error.code]).toEqual([422, 'own_shop']);
    const buyer = await bearer(uuidv7());
    const unknown = await addToCart(buyer, uuidv7(), 1);
    expect([unknown.status, unknown.body.error.code]).toEqual([422, 'variant_unavailable']);
    expect((await addToCart(buyer, listing.variants[0], 0)).status).toBe(400);
    expect((await addToCart(buyer, listing.variants[0], 100)).status).toBe(400);
    await catalog.handle(shopUpdated(listing, { status: 'suspended' }));
    const suspended = await addToCart(buyer, listing.variants[0], 1);
    expect(suspended.body.error.code).toBe('variant_unavailable');
  });

  it('caps the cart at 100 lines', async () => {
    const listing = await listed();
    const buyerId = uuidv7();
    await pg.query(
      `INSERT INTO cart_items (buyer_id, variant_id, quantity, updated_at)
       SELECT $1, gen_random_uuid(), 1, now() FROM generate_series(1, 100)`,
      [buyerId],
    );
    const res = await addToCart(await bearer(buyerId), listing.variants[0], 1);
    expect([res.status, res.body.error.code]).toEqual([422, 'cart_full']);
  });

  it('splits a checkout per shop, empties the cart and asks for stock', async () => {
    const a = await listed([15000, 9000]);
    const b = await listed([2000]);
    const buyerId = uuidv7();
    const auth = await bearer(buyerId);
    await addToCart(auth, a.variants[0], 2);
    await addToCart(auth, b.variants[0], 1);
    const recap = await preview(auth, 'wave');
    expect(recap.status).toBe(201);
    const res = await confirm(auth, recap.body.data.preview_id as string);
    expect(res.status).toBe(201);
    const orders = res.body.data.orders as OrderBody[];
    expect(orders.map((o) => o.shop)).toEqual(
      expect.arrayContaining([
        { shop_id: a.shopId, name: 'Pagnes Awa' },
        { shop_id: b.shopId, name: 'Pagnes Awa' },
      ]),
    );
    const first = orders.find((o) => (o.shop as { shop_id: string }).shop_id === a.shopId);
    expect(first).toMatchObject({
      status: 'pending',
      buyer_id: buyerId,
      seller_id: a.sellerId,
      currency: 'XOF',
      payment_method: 'wave',
      subtotal_seen: 30000,
      total: null,
      delivery: DELIVERY_VIEW,
    });
    expect(await outboxEvents(pg, 'poro.order.order.created', first?.order_id ?? '')).toEqual([
      expect.objectContaining({
        buyer_id: buyerId,
        seller_id: a.sellerId,
        payment_method: 'wave',
        items: [
          { product_id: a.productId, variant_id: a.variants[0], quantity: 2, unit_price: 15000 },
        ],
      }),
    ]);
    const cart = await request(server).get('/api/v1/cart').set('Authorization', auth);
    expect(cart.body.data.item_count).toBe(0);

    const replay = await confirm(auth, recap.body.data.preview_id as string);
    expect(replay.status).toBe(201);
    expect(replay.body.data.checkout_id).toBe(res.body.data.checkout_id);
    expect(replay.body.data.orders).toHaveLength(2);
  });

  it('validates the checkout', async () => {
    const listing = await listed();
    const auth = await bearer(uuidv7());
    const empty = await checkout(auth);
    expect([empty.status, empty.body.error.code]).toEqual([422, 'cart_empty']);
    await addToCart(auth, listing.variants[0], 1);
    const simulated = await checkout(auth, 'simulated');
    expect([simulated.status, simulated.body.error.code]).toEqual([
      422,
      'payment_method_unavailable',
    ]);
    const badPhone = await preview(auth, 'wave', { ...DELIVERY, phone: '0700000000' });
    expect(badPhone.status).toBe(400);
    const blankName = await preview(auth, 'wave', { ...DELIVERY, full_name: '   ' });
    expect(blankName.body.error.code).toBe('delivery_invalid');
    const nowhere = await preview(auth, 'wave', { ...DELIVERY, address: '   ' });
    expect([nowhere.status, nowhere.body.error.code]).toEqual([422, 'delivery_location_required']);
    await catalog.handle(productUpdated(listing, [15000, 9000], { status: 'draft' }));
    const gone = await checkout(auth);
    expect([gone.status, gone.body.error.code]).toEqual([422, 'cart_unavailable']);
  });

  it('confirms a cash order at the reserved price, then ships and completes it', async () => {
    const listing = await listed();
    const { auth, order } = await pendingOrder(listing);
    await saga.handle(reserved(listing, order.order_id, 15500));
    const placed = await getOrder(auth, order.order_id);
    expect(placed).toMatchObject({ status: 'confirmed', total: 31000, expires_at: null });
    expect(placed.items[0]).toMatchObject({
      unit_price: 15500,
      unit_price_seen: 15000,
      title: 'Pagne wax — Bleu',
    });
    expect(await outboxEvents(pg, 'poro.order.order.placed', order.order_id)).toEqual([
      expect.objectContaining({
        total: 31000,
        payment_method: 'cash_on_delivery',
        expires_at: null,
      }),
    ]);

    const seller = await bearer(listing.sellerId);
    const sellerList = await request(server)
      .get('/api/v1/seller/orders?status=confirmed')
      .set('Authorization', seller);
    expect((sellerList.body.data.items as OrderBody[]).map((o) => o.order_id)).toEqual([
      order.order_id,
    ]);
    const sellerView = await request(server)
      .get(`/api/v1/seller/orders/${order.order_id}`)
      .set('Authorization', seller);
    expect(sellerView.body.data.delivery).toEqual(DELIVERY_VIEW);

    expect(
      (
        await request(server)
          .post(`/api/v1/orders/${order.order_id}/confirm-receipt`)
          .set('Authorization', auth)
      ).status,
    ).toBe(409);
    const shipped = await request(server)
      .post(`/api/v1/seller/orders/${order.order_id}/ship`)
      .set('Authorization', seller)
      .send({ tracking: ' Colis remis à Yango ' });
    expect(shipped.body.data).toMatchObject({ status: 'shipped', tracking: 'Colis remis à Yango' });
    expect(
      (
        await request(server)
          .post(`/api/v1/orders/${order.order_id}/cancel`)
          .set('Authorization', auth)
      ).status,
    ).toBe(409);
    const done = await request(server)
      .post(`/api/v1/orders/${order.order_id}/confirm-receipt`)
      .set('Authorization', auth);
    expect(done.body.data.status).toBe('completed');
    expect(await outboxEvents(pg, 'poro.order.order.completed', order.order_id)).toEqual([
      expect.objectContaining({ total: 31000, payment_method: 'cash_on_delivery' }),
    ]);
    const again = await request(server)
      .post(`/api/v1/orders/${order.order_id}/confirm-receipt`)
      .set('Authorization', auth);
    expect(again.body.data.status).toBe('completed');
  });

  it('hides orders from other accounts', async () => {
    const listing = await listed();
    const { order } = await pendingOrder(listing);
    const stranger = await bearer(uuidv7());
    expect(
      (await request(server).get(`/api/v1/orders/${order.order_id}`).set('Authorization', stranger))
        .status,
    ).toBe(404);
    expect(
      (
        await request(server)
          .post(`/api/v1/seller/orders/${order.order_id}/ship`)
          .set('Authorization', stranger)
          .send({})
      ).status,
    ).toBe(404);
    expect(
      (await request(server).get('/api/v1/orders/not-a-uuid').set('Authorization', stranger))
        .status,
    ).toBe(400);
  });

  it('waits for a Wave payment, records failures and becomes paid', async () => {
    const listing = await listed();
    const { buyerId, auth, order } = await pendingOrder(listing, 'wave');
    await saga.handle(reserved(listing, order.order_id, 15000));
    const awaiting = await getOrder(auth, order.order_id);
    expect(awaiting.status).toBe('awaiting_payment');
    expect(Date.parse(String(awaiting.expires_at)) - Date.now()).toBeGreaterThan(29 * 60_000);

    await saga.handle(paymentEvent('failed', order, buyerId, 0));
    expect((await getOrder(auth, order.order_id)).last_payment_error).toBe('declined');
    await expect(saga.handle(paymentEvent('succeeded', order, buyerId, 1))).rejects.toBeInstanceOf(
      PermanentError,
    );
    const success = paymentEvent('succeeded', order, buyerId, 30000);
    await saga.handle(success);
    await saga.handle(success);
    expect(await getOrder(auth, order.order_id)).toMatchObject({
      status: 'paid',
      paid: true,
      last_payment_error: null,
    });

    const seller = await bearer(listing.sellerId);
    const cancelled = await request(server)
      .post(`/api/v1/seller/orders/${order.order_id}/cancel`)
      .set('Authorization', seller);
    expect(cancelled.body.data).toMatchObject({
      status: 'cancelled',
      cancel_reason: 'seller_cancelled',
    });
    expect(await outboxEvents(pg, 'poro.order.order.cancelled', order.order_id)).toEqual([
      expect.objectContaining({ reason: 'seller_cancelled', paid: true }),
    ]);
    await saga.handle(
      newEnvelope('poro.payment.refund.succeeded', 1, uuidv7(), {
        refund_id: uuidv7(),
        payment_id: uuidv7(),
        order_id: order.order_id,
        buyer_id: buyerId,
        amount: 30000,
        currency: 'XOF',
        provider: 'wave',
        refunded_at: new Date().toISOString(),
      }),
    );
    expect((await getOrder(auth, order.order_id)).refunded_at).not.toBeNull();
  });

  it('cancels on stock rejection and ignores a late reservation', async () => {
    const listing = await listed();
    const { auth, order } = await pendingOrder(listing);
    await saga.handle(
      newEnvelope('poro.shop.stock.rejected', 1, order.order_id, {
        order_id: order.order_id,
        shop_id: listing.shopId,
        reason: 'out_of_stock',
        variant_ids: [listing.variants[0]],
        rejected_at: new Date().toISOString(),
      }),
    );
    expect(await getOrder(auth, order.order_id)).toMatchObject({
      status: 'cancelled',
      cancel_reason: 'stock_rejected',
    });
    expect(await outboxEvents(pg, 'poro.order.order.cancelled', order.order_id)).toEqual([
      expect.objectContaining({ reason: 'stock_rejected', paid: false }),
    ]);

    const buyerCancel = await pendingOrder(listing);
    const cancel = await request(server)
      .post(`/api/v1/orders/${buyerCancel.order.order_id}/cancel`)
      .set('Authorization', buyerCancel.auth);
    expect(cancel.body.data.cancel_reason).toBe('buyer_cancelled');
    await saga.handle(reserved(listing, buyerCancel.order.order_id, 15000));
    expect((await getOrder(buyerCancel.auth, buyerCancel.order.order_id)).status).toBe('cancelled');
    expect(await outboxEvents(pg, 'poro.order.order.placed', buyerCancel.order.order_id)).toEqual(
      [],
    );
  });

  it('expires unpaid orders and asks for a refund when money arrives late', async () => {
    const listing = await listed();
    const { buyerId, auth, order } = await pendingOrder(listing, 'wave');
    await saga.handle(reserved(listing, order.order_id, 15000));
    const sagaService = app.get(SagaService);
    expect(await sagaService.expireUnpaid(new Date(Date.now() + 10 * 60_000))).toBe(0);
    await app.get(Scheduler).tick(new Date(Date.now() + 31 * 60_000));
    expect(await getOrder(auth, order.order_id)).toMatchObject({
      status: 'cancelled',
      cancel_reason: 'payment_timeout',
      paid: false,
    });

    await saga.handle(paymentEvent('succeeded', order, buyerId, 30000));
    expect((await getOrder(auth, order.order_id)).paid).toBe(true);
    expect(
      (await outboxEvents(pg, 'poro.order.order.cancelled', order.order_id))
        .map((e) => e.paid)
        .sort(),
    ).toEqual([false, true]);
  });

  it('completes shipped orders after 7 days and erases contacts after 90', async () => {
    const listing = await listed();
    const { auth, order } = await pendingOrder(listing);
    await saga.handle(reserved(listing, order.order_id, 15000));
    const seller = await bearer(listing.sellerId);
    await request(server)
      .post(`/api/v1/seller/orders/${order.order_id}/ship`)
      .set('Authorization', seller)
      .send({});
    const sagaService = app.get(SagaService);
    expect(await sagaService.autoComplete(new Date(Date.now() + 6 * 86_400_000))).toBe(0);
    expect(
      await sagaService.autoComplete(new Date(Date.now() + 8 * 86_400_000)),
    ).toBeGreaterThanOrEqual(1);
    const completed = await getOrder(auth, order.order_id);
    expect(completed.status).toBe('completed');

    expect(await sagaService.eraseContacts(new Date(Date.now() + 80 * 86_400_000))).toBe(0);
    expect(
      await sagaService.eraseContacts(new Date(Date.now() + 100 * 86_400_000)),
    ).toBeGreaterThanOrEqual(1);
    expect((await getOrder(auth, order.order_id)).delivery).toBeNull();
  });

  it('pages through orders', async () => {
    const listing = await listed();
    const buyerId = uuidv7();
    const auth = await bearer(buyerId);
    for (let i = 0; i < 3; i++) {
      await addToCart(auth, listing.variants[0], 1);
      expect((await checkout(auth)).status).toBe(201);
    }
    const first = await request(server).get('/api/v1/orders?limit=2').set('Authorization', auth);
    expect(first.body.data.items).toHaveLength(2);
    const second = await request(server)
      .get(`/api/v1/orders?limit=2&cursor=${String(first.body.data.next_cursor)}`)
      .set('Authorization', auth);
    expect(second.body.data).toMatchObject({ next_cursor: null });
    expect(second.body.data.items).toHaveLength(1);
  });

  it('dead-letters events it cannot apply', async () => {
    await expect(saga.handle(reserved(newListing(), uuidv7(), 1))).rejects.toBeInstanceOf(
      PermanentError,
    );
    await expect(
      saga.handle(newEnvelope('poro.order.order.placed', 1, uuidv7(), { order_id: uuidv7() })),
    ).rejects.toBeInstanceOf(PermanentError);
    await expect(
      saga.handle({ ...reserved(newListing(), uuidv7(), 1), version: 2 }),
    ).rejects.toBeInstanceOf(PermanentError);
    const listing = await listed();
    const { order } = await pendingOrder(listing);
    const wrongCurrency = reserved(listing, order.order_id, 1);
    (wrongCurrency.data as Record<string, unknown>).currency = 'NGN';
    await expect(saga.handle(wrongCurrency)).rejects.toBeInstanceOf(PermanentError);
  });

  it('keeps the newest catalogue snapshot and remembers deletions', async () => {
    const listing = newListing(1);
    await catalog.handle(shopUpdated(listing));
    const newer = productUpdated(listing, [5000]);
    const older = productUpdated(listing, [9999], { updated_at: '2020-01-01T00:00:00Z' });
    await catalog.handle(newer);
    await catalog.handle(older);
    await catalog.handle(newer);
    const { rows } = await pg.query('SELECT price::int FROM catalog_variants WHERE id = $1', [
      listing.variants[0],
    ]);
    expect(rows).toEqual([{ price: 5000 }]);

    await catalog.handle(
      newEnvelope('poro.shop.product.deleted', 1, listing.productId, {
        product_id: listing.productId,
        shop_id: listing.shopId,
        owner_id: listing.sellerId,
        deleted_at: new Date(Date.now() + 1000).toISOString(),
      }),
    );
    await catalog.handle(productUpdated(listing, [7000]));
    const auth = await bearer(uuidv7());
    expect((await addToCart(auth, listing.variants[0], 1)).body.error.code).toBe(
      'variant_unavailable',
    );
    await expect(
      catalog.handle(newEnvelope('poro.shop.product.updated', 1, uuidv7(), { product_id: 'x' })),
    ).rejects.toBeInstanceOf(PermanentError);
    await expect(
      catalog.handle(newEnvelope('poro.shop.shop.created', 1, uuidv7(), {})),
    ).rejects.toBeInstanceOf(PermanentError);
  });

  it('serves health probes', async () => {
    const ready = await request(server).get('/health/ready');
    expect(ready.body).toMatchObject({ status: 'ok', service: 'order' });
  });
});
