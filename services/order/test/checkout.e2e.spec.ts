import { NestExpressApplication } from '@nestjs/platform-express';
import { Client } from 'pg';
import request from 'supertest';
import type { App } from 'supertest/types';

import { CatalogConsumer } from '../src/catalog/catalog.consumer';
import { uuidv7 } from '../src/common/uuid';
import { SagaService } from '../src/orders/saga.service';
import { AccountServer } from './support/account';
import { createApp } from './support/app';
import { Listing, newListing, productUpdated, shopUpdated } from './support/catalog';
import { JwksServer, SigningKey, signToken, signingKey } from './support/jwt';
import { TestDatabase, startDatabase } from './support/postgres';

const GPS_DELIVERY = {
  full_name: 'Moussa Diop',
  phone: '+221770000000',
  city: 'Dakar',
  location: { latitude: 14.6937, longitude: -17.4441, accuracy_m: 12 },
};

describe('checkout with recap and address book', () => {
  let db: TestDatabase;
  let pg: Client;
  let jwks: JwksServer;
  let account: AccountServer;
  let key: SigningKey;
  let app: NestExpressApplication;
  let server: App;
  let catalog: CatalogConsumer;

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    key = signingKey();
    jwks = new JwksServer();
    jwks.keys = [key.publicJwk];
    account = new AccountServer();
    const accountUrl = await account.start();
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: await jwks.start(),
      AUTH_URL: accountUrl,
      USER_URL: accountUrl,
      ACCOUNT_LOOKUP_TIMEOUT_MS: '300',
      PAYMENT_METHODS: 'cash_on_delivery',
      SCHEDULER_ENABLED: 'false',
    });
    server = app.getHttpServer();
    catalog = app.get(CatalogConsumer);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await jwks.stop();
    await account.stop();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  beforeEach(() => {
    account.authStatus = 200;
    account.userStatus = 200;
    account.delayMs = 0;
  });

  async function bearer(userId: string = uuidv7()): Promise<string> {
    return `Bearer ${await signToken(key, { sub: userId })}`;
  }

  async function listed(prices: number[] = [15000, 9000]): Promise<Listing> {
    const listing = newListing(prices.length);
    await catalog.handle(shopUpdated(listing));
    await catalog.handle(productUpdated(listing, prices));
    return listing;
  }

  async function addToCart(auth: string, variantId: string, quantity: number): Promise<void> {
    const res = await request(server)
      .put(`/api/v1/cart/items/${variantId}`)
      .set('Authorization', auth)
      .send({ quantity });
    expect(res.status).toBe(200);
  }

  function preview(auth: string, body: Record<string, unknown>): request.Test {
    return request(server).post('/api/v1/checkout/preview').set('Authorization', auth).send(body);
  }

  function confirm(auth: string, previewId: string, confirmed: unknown = true): request.Test {
    return request(server)
      .post('/api/v1/checkout/confirm')
      .set('Authorization', auth)
      .send({ preview_id: previewId, confirmed });
  }

  it('prefills the contact from the account, without blocking when it is down', async () => {
    const auth = await bearer();
    const res = await request(server).get('/api/v1/checkout/prefill').set('Authorization', auth);
    expect(res.body.data).toEqual({
      source: 'account',
      address_id: null,
      delivery: {
        full_name: 'Awa Koné',
        phone: '+2250700000001',
        city: null,
        address: null,
        landmark: null,
        location: null,
      },
      account_lookup: 'ok',
    });
    expect(account.authorizations.at(-1)).toBe(auth);

    account.authStatus = 503;
    const partial = await request(server)
      .get('/api/v1/checkout/prefill')
      .set('Authorization', auth);
    expect(partial.body.data).toMatchObject({
      source: 'account',
      account_lookup: 'partial',
      delivery: { full_name: 'Awa Koné', phone: null },
    });

    account.delayMs = 1_000;
    const slow = await request(server).get('/api/v1/checkout/prefill').set('Authorization', auth);
    expect(slow.status).toBe(200);
    expect(slow.body.data).toMatchObject({ source: 'empty', account_lookup: 'unavailable' });
  });

  it('shows a full recap and orders nothing before the explicit confirmation', async () => {
    const a = await listed([15000, 9000]);
    const b = await listed([2500]);
    const buyerId = uuidv7();
    const auth = await bearer(buyerId);
    await addToCart(auth, a.variants[0], 2);
    await addToCart(auth, a.variants[1], 1);
    await addToCart(auth, b.variants[0], 4);

    const recap = await preview(auth, { delivery: GPS_DELIVERY });
    expect(recap.status).toBe(201);
    expect(recap.body.data).toMatchObject({
      preview_id: expect.any(String),
      expires_at: expect.any(String),
      totals: [{ currency: 'XOF', amount: 49000 }],
      item_count: 7,
      delivery: { ...GPS_DELIVERY, address: null, landmark: null },
      payment_method: 'cash_on_delivery',
      payment_method_label: 'Paiement à la livraison',
      delivery_fee_notice: 'Frais de livraison à régler au livreur, non inclus dans le total.',
    });
    expect(recap.body.data.shops[0]).toMatchObject({
      shop: { shop_id: a.shopId, name: 'Pagnes Awa' },
      currency: 'XOF',
      subtotal: 39000,
      items: [
        {
          variant_id: a.variants[0],
          title: 'Pagne wax',
          variant_title: 'V0',
          quantity: 2,
          unit_price: 15000,
          line_total: 30000,
        },
        { variant_id: a.variants[1], quantity: 1, line_total: 9000 },
      ],
    });
    const before = await pg.query('SELECT count(*)::int AS n FROM orders WHERE buyer_id = $1', [
      buyerId,
    ]);
    expect(before.rows).toEqual([{ n: 0 }]);

    const refused = await confirm(auth, recap.body.data.preview_id as string, false);
    expect([refused.status, refused.body.error.code]).toEqual([422, 'confirmation_required']);
    expect((await confirm(auth, recap.body.data.preview_id as string, 'yes')).status).toBe(400);

    const placed = await confirm(auth, recap.body.data.preview_id as string);
    expect(placed.status).toBe(201);
    expect(placed.body.data.orders).toHaveLength(2);
    expect(placed.body.data.orders[0]).toMatchObject({
      status: 'pending',
      payment_method: 'cash_on_delivery',
      payment_method_label: 'Paiement à la livraison',
      delivery: {
        full_name: 'Moussa Diop',
        address: null,
        location: { latitude: 14.6937, longitude: -17.4441, accuracy_m: 12 },
      },
    });
    expect(placed.body.data.address_saved).toBe(false);
  });

  it('refuses a recap that no longer matches the cart, an expired one or another buyer s', async () => {
    const listing = await listed();
    const auth = await bearer();
    await addToCart(auth, listing.variants[0], 1);
    const first = await preview(auth, { delivery: GPS_DELIVERY });
    await addToCart(auth, listing.variants[0], 2);
    const changed = await confirm(auth, first.body.data.preview_id as string);
    expect([changed.status, changed.body.error.code]).toEqual([409, 'preview_outdated']);

    const second = await preview(auth, { delivery: GPS_DELIVERY });
    await catalog.handle(productUpdated(listing, [16000, 9000]));
    const repriced = await confirm(auth, second.body.data.preview_id as string);
    expect(repriced.body.error.code).toBe('preview_outdated');

    const third = await preview(auth, { delivery: GPS_DELIVERY });
    const stranger = await confirm(await bearer(), third.body.data.preview_id as string);
    expect([stranger.status, stranger.body.error.code]).toEqual([404, 'preview_not_found']);
    await pg.query(
      `UPDATE checkout_previews SET expires_at = now() - interval '1 minute' WHERE id = $1`,
      [third.body.data.preview_id],
    );
    const expired = await confirm(auth, third.body.data.preview_id as string);
    expect([expired.status, expired.body.error.code]).toEqual([409, 'preview_expired']);
  });

  it('requires a position or an address, the rest is optional', async () => {
    const listing = await listed();
    const auth = await bearer();
    await addToCart(auth, listing.variants[0], 1);
    const cases: [Record<string, unknown>, number, string | undefined][] = [
      [{ ...GPS_DELIVERY, location: null }, 422, 'delivery_location_required'],
      [{ ...GPS_DELIVERY, location: { latitude: 91, longitude: 0 } }, 400, 'invalid_request'],
      [{ ...GPS_DELIVERY, city: '' }, 422, 'delivery_invalid'],
      [{ ...GPS_DELIVERY, phone: '770000000' }, 400, 'invalid_request'],
      [
        { full_name: 'Awa', phone: '+2250700000000', city: 'Abidjan', address: 'Yopougon' },
        201,
        undefined,
      ],
    ];
    for (const [delivery, status, code] of cases) {
      const res = await preview(auth, { delivery });
      expect([res.status, res.body.error?.code]).toEqual([status, code]);
    }
  });

  it('saves the confirmed delivery and offers it next time', async () => {
    const listing = await listed();
    const buyerId = uuidv7();
    const auth = await bearer(buyerId);
    await addToCart(auth, listing.variants[0], 1);
    const recap = await preview(auth, {
      delivery: { ...GPS_DELIVERY, landmark: 'Derrière la mosquée' },
      save_address: true,
      address_label: 'Maison',
    });
    const placed = await confirm(auth, recap.body.data.preview_id as string);
    expect(placed.body.data.address_saved).toBe(true);

    const prefill = await request(server)
      .get('/api/v1/checkout/prefill')
      .set('Authorization', auth);
    expect(prefill.body.data).toMatchObject({
      source: 'address_book',
      account_lookup: 'skipped',
      delivery: { full_name: 'Moussa Diop', landmark: 'Derrière la mosquée', city: 'Dakar' },
    });
    const book = await request(server).get('/api/v1/addresses').set('Authorization', auth);
    expect(book.body.data).toEqual([
      expect.objectContaining({
        label: 'Maison',
        is_default: true,
        address_id: prefill.body.data.address_id,
      }),
    ]);
  });

  it('manages the address book', async () => {
    const auth = await bearer();
    const create = (body: Record<string, unknown>): request.Test =>
      request(server).post('/api/v1/addresses').set('Authorization', auth).send(body);
    const home = await create({ ...GPS_DELIVERY, label: 'Maison' });
    expect(home.status).toBe(201);
    expect(home.body.data).toMatchObject({ is_default: true, label: 'Maison' });
    const office = await create({
      full_name: 'Moussa Diop',
      phone: '+221770000000',
      city: 'Dakar',
      address: 'Plateau, immeuble Kebe',
      label: 'Bureau',
    });
    expect(office.body.data.is_default).toBe(false);

    const made = await request(server)
      .post(`/api/v1/addresses/${String(office.body.data.address_id)}/default`)
      .set('Authorization', auth);
    expect(made.body.data.is_default).toBe(true);
    const updated = await request(server)
      .put(`/api/v1/addresses/${String(home.body.data.address_id)}`)
      .set('Authorization', auth)
      .send({ ...GPS_DELIVERY, landmark: 'Maison bleue', label: 'Maison' });
    expect(updated.body.data).toMatchObject({ landmark: 'Maison bleue', is_default: false });

    const removed = await request(server)
      .delete(`/api/v1/addresses/${String(office.body.data.address_id)}`)
      .set('Authorization', auth);
    expect(removed.status).toBe(204);
    const book = await request(server).get('/api/v1/addresses').set('Authorization', auth);
    expect(book.body.data).toEqual([
      expect.objectContaining({ label: 'Maison', is_default: true }),
    ]);

    const stranger = await bearer();
    expect(
      (
        await request(server)
          .delete(`/api/v1/addresses/${String(home.body.data.address_id)}`)
          .set('Authorization', stranger)
      ).status,
    ).toBe(404);
    for (let i = 0; i < 9; i++) {
      expect((await create(GPS_DELIVERY)).status).toBe(201);
    }
    const full = await create(GPS_DELIVERY);
    expect([full.status, full.body.error.code]).toEqual([422, 'address_book_full']);
  });

  it('erases the location with the contact and purges old previews', async () => {
    const listing = await listed();
    const auth = await bearer();
    await addToCart(auth, listing.variants[0], 1);
    const recap = await preview(auth, { delivery: GPS_DELIVERY });
    const placed = await confirm(auth, recap.body.data.preview_id as string);
    const orderId = placed.body.data.orders[0].order_id as string;
    await pg.query(
      `UPDATE orders SET status = 'cancelled', cancelled_at = now() - interval '100 days' WHERE id = $1`,
      [orderId],
    );
    const saga = app.get(SagaService);
    expect(await saga.eraseContacts()).toBeGreaterThanOrEqual(1);
    const { rows } = await pg.query(
      'SELECT contact_name, landmark, latitude, longitude FROM orders WHERE id = $1',
      [orderId],
    );
    expect(rows).toEqual([{ contact_name: null, landmark: null, latitude: null, longitude: null }]);
    expect(await saga.purgePreviews(new Date(Date.now() + 2 * 86_400_000))).toBeGreaterThanOrEqual(
      1,
    );
  });
});
