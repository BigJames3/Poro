import {
  DeleteObjectCommand,
  GetObjectCommand,
  PutObjectCommand,
  S3Client,
} from '@aws-sdk/client-s3';
import { NestExpressApplication } from '@nestjs/platform-express';
import { sdkStreamMixin } from '@smithy/util-stream';
import { mockClient } from 'aws-sdk-client-mock';
import { Readable } from 'node:stream';
import { Client } from 'pg';
import sharp from 'sharp';
import request from 'supertest';
import type { App } from 'supertest/types';

import { uuidv7 } from '../src/common/uuid';
import { createApp } from './support/app';
import { JwksServer, SigningKey, signToken, signingKey } from './support/jwt';
import { outboxEvents } from './support/outbox';
import { TestDatabase, startDatabase } from './support/postgres';

const s3 = mockClient(S3Client);
const CDN = 'https://cdn.poro.test/shop';

describe('shop service HTTP API', () => {
  let db: TestDatabase;
  let pg: Client;
  let jwks: JwksServer;
  let key: SigningKey;
  let app: NestExpressApplication;
  let server: App;
  let handles = 0;

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    key = signingKey();
    jwks = new JwksServer();
    jwks.keys = [key.publicJwk];
    const jwksUrl = await jwks.start();
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: jwksUrl,
      S3_ENDPOINT: 'http://seaweedfs:8333',
      S3_PUBLIC_ENDPOINT: 'http://localhost:9000',
      MEDIA_PUBLIC_BASE_URL: CDN,
    });
    server = app.getHttpServer();
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await jwks.stop();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  beforeEach(() => {
    s3.reset();
  });

  async function as(
    userId: string = uuidv7(),
    roles: string[] = ['PERSONAL'],
  ): Promise<{ userId: string; auth: string }> {
    return { userId, auth: `Bearer ${await signToken(key, { sub: userId, roles })}` };
  }

  function nextHandle(): string {
    handles += 1;
    return `boutique_awa${handles}`;
  }

  async function openShop(
    country = 'CI',
  ): Promise<{ userId: string; auth: string; shop: Record<string, unknown> }> {
    const seller = await as();
    const res = await request(server)
      .post('/api/v1/shops')
      .set('Authorization', seller.auth)
      .send({ name: "Pagnes d'Awa", handle: nextHandle(), country_code: country });
    expect(res.status).toBe(201);
    return { ...seller, shop: res.body.data as Record<string, unknown> };
  }

  async function createProduct(
    auth: string,
    variants: Record<string, unknown>[] = [{ title: 'Bleu', price: 15000, stock: 3 }],
  ): Promise<Record<string, unknown>> {
    const res = await request(server)
      .post('/api/v1/shops/me/products')
      .set('Authorization', auth)
      .send({ title: 'Pagne wax 6 yards', description: 'Tissé main', variants });
    expect(res.status).toBe(201);
    return res.body.data as Record<string, unknown>;
  }

  async function addPhoto(
    auth: string,
    userId: string,
    productId: string,
  ): Promise<Record<string, unknown>> {
    const upload = await request(server)
      .post(`/api/v1/shops/me/products/${productId}/images/upload-url`)
      .set('Authorization', auth)
      .send({ content_type: 'image/jpeg' });
    expect(upload.status).toBe(201);
    const uploadKey = upload.body.data.upload_key as string;
    expect(uploadKey).toMatch(new RegExp(`^uploads/${userId}/[0-9a-f-]{36}$`));
    const jpeg = await sharp({
      create: { width: 2000, height: 1500, channels: 3, background: '#e07a1f' },
    })
      .jpeg()
      .toBuffer();
    s3.on(GetObjectCommand, { Key: uploadKey }).resolves({
      ContentLength: jpeg.length,
      Body: sdkStreamMixin(Readable.from([jpeg])),
    });
    const res = await request(server)
      .put(`/api/v1/shops/me/products/${productId}/images`)
      .set('Authorization', auth)
      .send({ upload_key: uploadKey });
    expect(res.status).toBe(200);
    return res.body.data as Record<string, unknown>;
  }

  it('requires a token on seller routes', async () => {
    expect((await request(server).post('/api/v1/shops').send({})).status).toBe(401);
    expect((await request(server).get('/api/v1/shops/me')).status).toBe(401);
  });

  it('opens one shop per account and announces it', async () => {
    const { userId, auth, shop } = await openShop('CM');
    expect(shop).toMatchObject({
      owner_id: userId,
      name: "Pagnes d'Awa",
      country_code: 'CM',
      currency: 'XAF',
      status: 'active',
      logo_url: null,
    });
    const shopId = shop.shop_id as string;
    expect(await outboxEvents(pg, 'poro.shop.shop.created', shopId)).toEqual([
      expect.objectContaining({
        shop_id: shopId,
        owner_id: userId,
        currency: 'XAF',
        country_code: 'CM',
      }),
    ]);
    expect(await outboxEvents(pg, 'poro.shop.shop.updated', shopId)).toEqual([
      expect.objectContaining({ status: 'active', description: null }),
    ]);

    const again = await request(server)
      .post('/api/v1/shops')
      .set('Authorization', auth)
      .send({ name: 'Autre', handle: nextHandle(), country_code: 'CI' });
    expect(again.status).toBe(409);
    expect(again.body.error.code).toBe('shop_exists');

    const mine = await request(server).get('/api/v1/shops/me').set('Authorization', auth);
    expect(mine.body.data.shop_id).toBe(shopId);
  });

  it('validates the handle, the name and the country', async () => {
    const { auth: taken, shop } = await openShop();
    const other = await as();
    const cases: [Record<string, unknown>, number, string][] = [
      [{ name: 'X', handle: shop.handle, country_code: 'CI' }, 409, 'handle_taken'],
      [{ name: 'X', handle: 'Shop', country_code: 'CI' }, 422, 'handle_reserved'],
      [{ name: 'X', handle: 'a b', country_code: 'CI' }, 422, 'handle_invalid'],
      [{ name: '   ', handle: nextHandle(), country_code: 'CI' }, 422, 'shop_name_invalid'],
      [{ name: 'X', handle: nextHandle(), country_code: 'GH' }, 400, 'invalid_request'],
      [
        { name: 'X', handle: nextHandle(), country_code: 'CI', owner_id: uuidv7() },
        400,
        'invalid_request',
      ],
    ];
    for (const [body, status, code] of cases) {
      const res = await request(server)
        .post('/api/v1/shops')
        .set('Authorization', other.auth)
        .send(body);
      expect([res.status, res.body.error?.code]).toEqual([status, code]);
    }
    expect(
      (await request(server).get('/api/v1/shops/me').set('Authorization', other.auth)).status,
    ).toBe(404);
    expect(taken).toBeDefined();
  });

  it('updates the shop and publishes a snapshot only when a public field changes', async () => {
    const { auth, shop } = await openShop();
    const shopId = shop.shop_id as string;
    const res = await request(server)
      .patch('/api/v1/shops/me')
      .set('Authorization', auth)
      .send({ name: '  Pagnes   Awa ', description: 'Wax et bazin\nLivraison à Abidjan' });
    expect(res.status).toBe(200);
    expect(res.body.data).toMatchObject({
      name: 'Pagnes Awa',
      description: 'Wax et bazin\nLivraison à Abidjan',
    });
    await request(server)
      .patch('/api/v1/shops/me')
      .set('Authorization', auth)
      .send({ name: 'Pagnes Awa' });
    expect((await outboxEvents(pg, 'poro.shop.shop.updated', shopId)).map((e) => e.name)).toEqual([
      "Pagnes d'Awa",
      'Pagnes Awa',
    ]);

    const publicView = await request(server).get(
      `/api/v1/shops/${String(shop.handle).toUpperCase()}`,
    );
    expect(publicView.status).toBe(200);
    expect(publicView.body.data).toMatchObject({ shop_id: shopId, name: 'Pagnes Awa' });
    expect(publicView.body.data.owner_id).toBeUndefined();
    expect((await request(server).get('/api/v1/shops/nobody_here')).status).toBe(404);
  });

  it('closes, reopens, and lets moderators suspend a shop', async () => {
    const { auth, shop } = await openShop();
    const shopId = shop.shop_id as string;
    const close = await request(server).post('/api/v1/shops/me/close').set('Authorization', auth);
    expect(close.body.data.status).toBe('closed');
    expect((await request(server).get(`/api/v1/shops/${String(shop.handle)}`)).status).toBe(404);
    const reopen = await request(server).post('/api/v1/shops/me/reopen').set('Authorization', auth);
    expect(reopen.body.data.status).toBe('active');

    const user = await as();
    expect(
      (
        await request(server)
          .post(`/api/v1/admin/shops/${shopId}/suspend`)
          .set('Authorization', user.auth)
      ).status,
    ).toBe(403);
    const moderator = await as(uuidv7(), ['PERSONAL', 'MODERATOR']);
    const suspended = await request(server)
      .post(`/api/v1/admin/shops/${shopId}/suspend`)
      .set('Authorization', moderator.auth);
    expect(suspended.body.data.status).toBe('suspended');

    const closeSuspended = await request(server)
      .post('/api/v1/shops/me/close')
      .set('Authorization', auth);
    expect([closeSuspended.status, closeSuspended.body.error.code]).toEqual([
      409,
      'shop_status_conflict',
    ]);
    const write = await request(server)
      .post('/api/v1/shops/me/products')
      .set('Authorization', auth)
      .send({ title: 'X', variants: [{ title: 'U', price: 1 }] });
    expect([write.status, write.body.error.code]).toEqual([403, 'shop_suspended']);

    const reinstated = await request(server)
      .post(`/api/v1/admin/shops/${shopId}/reinstate`)
      .set('Authorization', moderator.auth);
    expect(reinstated.body.data.status).toBe('active');
    expect((await outboxEvents(pg, 'poro.shop.shop.updated', shopId)).map((e) => e.status)).toEqual(
      ['active', 'closed', 'active', 'suspended', 'active'],
    );
    expect(
      (
        await request(server)
          .post(`/api/v1/admin/shops/${uuidv7()}/suspend`)
          .set('Authorization', moderator.auth)
      ).status,
    ).toBe(404);
    expect(
      (
        await request(server)
          .post('/api/v1/admin/shops/not-a-uuid/suspend')
          .set('Authorization', moderator.auth)
      ).status,
    ).toBe(400);
  });

  it('replaces the logo with a re-encoded square', async () => {
    const { userId, auth, shop } = await openShop();
    const upload = await request(server)
      .post('/api/v1/shops/me/logo/upload-url')
      .set('Authorization', auth)
      .send({ content_type: 'image/png' });
    expect(upload.status).toBe(201);
    expect(upload.body.data.upload_url).toBe('http://localhost:9000/poro-shop');
    const uploadKey = upload.body.data.upload_key as string;
    const png = await sharp({
      create: { width: 900, height: 600, channels: 3, background: '#123' },
    })
      .png()
      .toBuffer();
    s3.on(GetObjectCommand, { Key: uploadKey }).resolves({
      ContentLength: png.length,
      Body: sdkStreamMixin(Readable.from([png])),
    });
    const set = await request(server)
      .put('/api/v1/shops/me/logo')
      .set('Authorization', auth)
      .send({ upload_key: uploadKey });
    expect(set.status).toBe(200);
    const put = s3.commandCalls(PutObjectCommand)[0].args[0].input;
    expect(put.Key).toMatch(new RegExp(`^shops/${String(shop.shop_id)}/[0-9a-f-]{36}\\.webp$`));
    expect(await sharp(put.Body as Buffer).metadata()).toMatchObject({ width: 512, height: 512 });
    expect(set.body.data.logo_url).toBe(`${CDN}/${String(put.Key)}`);

    const foreign = await request(server)
      .put('/api/v1/shops/me/logo')
      .set('Authorization', auth)
      .send({ upload_key: `uploads/${uuidv7()}/${uuidv7()}` });
    expect(foreign.body.error.code).toBe('image_upload_invalid');

    const removed = await request(server)
      .delete('/api/v1/shops/me/logo')
      .set('Authorization', auth);
    expect(removed.body.data.logo_url).toBeNull();
    expect(s3.commandCalls(DeleteObjectCommand).map((c) => c.args[0].input.Key)).toEqual([
      uploadKey,
      put.Key,
    ]);
    expect(userId).toBeDefined();
  });

  it('creates a draft product, publishes it once it has a photo, and lists it publicly', async () => {
    const { userId, auth, shop } = await openShop();
    const product = await createProduct(auth, [
      { title: 'Bleu', price: 15000, stock: 3 },
      { title: 'Rouge', price: 16000 },
    ]);
    const productId = product.product_id as string;
    expect(product).toMatchObject({
      status: 'draft',
      currency: 'XOF',
      images: [],
      variants: [
        {
          title: 'Bleu',
          price: 15000,
          in_stock: true,
          stock_on_hand: 3,
          stock_reserved: 0,
          stock_available: 3,
        },
        { title: 'Rouge', price: 16000, in_stock: false, stock_on_hand: 0 },
      ],
    });
    expect((await request(server).get(`/api/v1/products/${productId}`)).status).toBe(404);

    const noPhoto = await request(server)
      .patch(`/api/v1/shops/me/products/${productId}`)
      .set('Authorization', auth)
      .send({ status: 'active' });
    expect([noPhoto.status, noPhoto.body.error.code]).toEqual([422, 'image_required']);

    const withPhoto = await addPhoto(auth, userId, productId);
    const put = s3.commandCalls(PutObjectCommand)[0].args[0].input;
    expect(put.Key).toMatch(
      new RegExp(`^products/${String(shop.shop_id)}/${productId}/[0-9a-f-]{36}\\.webp$`),
    );
    expect(await sharp(put.Body as Buffer).metadata()).toMatchObject({ width: 1080, height: 810 });
    expect(withPhoto.images).toEqual([
      { image_id: expect.any(String), url: `${CDN}/${String(put.Key)}` },
    ]);

    const active = await request(server)
      .patch(`/api/v1/shops/me/products/${productId}`)
      .set('Authorization', auth)
      .send({ status: 'active' });
    expect(active.body.data.status).toBe('active');

    const publicView = await request(server).get(`/api/v1/products/${productId}`);
    expect(publicView.status).toBe(200);
    expect(publicView.body.data.variants[0]).toEqual({
      variant_id: expect.any(String),
      title: 'Bleu',
      price: 15000,
      in_stock: true,
    });
    const list = await request(server).get(`/api/v1/shops/${String(shop.handle)}/products`);
    const listed = list.body.data.items as { product_id: string }[];
    expect(listed.map((p) => p.product_id)).toEqual([productId]);

    const snapshots = await outboxEvents(pg, 'poro.shop.product.updated', productId);
    expect(snapshots.map((s) => s.status)).toEqual(['draft', 'draft', 'active']);
    expect(snapshots[2]).toMatchObject({
      owner_id: userId,
      currency: 'XOF',
      image_urls: [`${CDN}/${String(put.Key)}`],
    });
    const times = snapshots.map((s) => Date.parse(String(s.updated_at)));
    expect([...times].sort((a, b) => a - b)).toEqual(times);

    const lastPhoto = await request(server)
      .delete(
        `/api/v1/shops/me/products/${productId}/images/${String(withPhoto.images && (withPhoto.images as { image_id: string }[])[0].image_id)}`,
      )
      .set('Authorization', auth);
    expect([lastPhoto.status, lastPhoto.body.error.code]).toEqual([422, 'image_required']);
  });

  it('pages through products with a cursor and filters by status', async () => {
    const { auth, shop } = await openShop();
    const ids: string[] = [];
    for (let i = 0; i < 3; i++) {
      ids.push((await createProduct(auth)).product_id as string);
    }
    const first = await request(server)
      .get('/api/v1/shops/me/products?limit=2')
      .set('Authorization', auth);
    expect(first.body.data.items).toHaveLength(2);
    const second = await request(server)
      .get(`/api/v1/shops/me/products?limit=2&cursor=${String(first.body.data.next_cursor)}`)
      .set('Authorization', auth);
    expect(second.body.data.next_cursor).toBeNull();
    const seen = [...first.body.data.items, ...second.body.data.items].map(
      (p: { product_id: string }) => p.product_id,
    );
    expect(seen).toEqual([...ids].reverse());

    const drafts = await request(server)
      .get('/api/v1/shops/me/products?status=active')
      .set('Authorization', auth);
    expect(drafts.body.data.items).toEqual([]);
    const publicList = await request(server).get(`/api/v1/shops/${String(shop.handle)}/products`);
    expect(publicList.body.data).toEqual({ items: [], next_cursor: null });
    const bad = await request(server)
      .get('/api/v1/shops/me/products?cursor=bm9wZQ')
      .set('Authorization', auth);
    expect([bad.status, bad.body.error.code]).toEqual([400, 'invalid_cursor']);
  });

  it('syncs variants and guards the stock', async () => {
    const { auth } = await openShop();
    const product = await createProduct(auth, [
      { title: 'S', price: 5000, stock: 2 },
      { title: 'M', price: 5000, stock: 1 },
    ]);
    const productId = product.product_id as string;
    const [small, medium] = product.variants as { variant_id: string }[];

    const synced = await request(server)
      .patch(`/api/v1/shops/me/products/${productId}`)
      .set('Authorization', auth)
      .send({
        variants: [
          { variant_id: medium.variant_id, title: 'M', price: 5500 },
          { title: 'L', price: 6000, stock: 4 },
        ],
      });
    expect(synced.status).toBe(200);
    expect(synced.body.data.variants).toMatchObject([
      { variant_id: medium.variant_id, price: 5500, stock_on_hand: 1 },
      { title: 'L', stock_on_hand: 4 },
    ]);
    const syncedVariants = synced.body.data.variants as { variant_id: string }[];
    expect(syncedVariants.map((v) => v.variant_id)).not.toContain(small.variant_id);

    const cases: [Record<string, unknown>, number, string][] = [
      [
        { variants: [{ variant_id: small.variant_id, title: 'S', price: 1 }] },
        422,
        'variant_not_found',
      ],
      [
        { variants: [{ variant_id: medium.variant_id, title: 'M', price: 1, stock: 9 }] },
        422,
        'stock_not_editable_here',
      ],
      [{ variants: [{ title: 'Z', price: 0 }] }, 400, 'invalid_request'],
      [{ variants: [] }, 400, 'invalid_request'],
      [{ title: '' }, 422, 'product_title_invalid'],
    ];
    for (const [body, status, code] of cases) {
      const res = await request(server)
        .patch(`/api/v1/shops/me/products/${productId}`)
        .set('Authorization', auth)
        .send(body);
      expect([res.status, res.body.error?.code]).toEqual([status, code]);
    }

    await pg.query('UPDATE variants SET stock_reserved = 1 WHERE id = $1', [medium.variant_id]);
    const below = await request(server)
      .put(`/api/v1/shops/me/products/${productId}/variants/${medium.variant_id}/stock`)
      .set('Authorization', auth)
      .send({ stock_on_hand: 0 });
    expect([below.status, below.body.error.code]).toEqual([409, 'stock_below_reserved']);
    const dropHeld = await request(server)
      .patch(`/api/v1/shops/me/products/${productId}`)
      .set('Authorization', auth)
      .send({ variants: [{ title: 'Seul', price: 1 }] });
    expect([dropHeld.status, dropHeld.body.error.code]).toEqual([409, 'variant_has_reservations']);
    const deleteHeld = await request(server)
      .delete(`/api/v1/shops/me/products/${productId}`)
      .set('Authorization', auth);
    expect([deleteHeld.status, deleteHeld.body.error.code]).toEqual([
      409,
      'product_has_reservations',
    ]);

    const restock = await request(server)
      .put(`/api/v1/shops/me/products/${productId}/variants/${medium.variant_id}/stock`)
      .set('Authorization', auth)
      .send({ stock_on_hand: 5 });
    expect(restock.body.data.variants[0]).toMatchObject({
      stock_on_hand: 5,
      stock_reserved: 1,
      stock_available: 4,
    });
    const unknownVariant = await request(server)
      .put(`/api/v1/shops/me/products/${productId}/variants/${uuidv7()}/stock`)
      .set('Authorization', auth)
      .send({ stock_on_hand: 5 });
    expect(unknownVariant.status).toBe(404);

    await pg.query('UPDATE variants SET stock_reserved = 0 WHERE id = $1', [medium.variant_id]);
    const deleted = await request(server)
      .delete(`/api/v1/shops/me/products/${productId}`)
      .set('Authorization', auth);
    expect(deleted.status).toBe(204);
    expect(await outboxEvents(pg, 'poro.shop.product.deleted', productId)).toHaveLength(1);
    expect(
      (
        await request(server)
          .get(`/api/v1/shops/me/products/${productId}`)
          .set('Authorization', auth)
      ).status,
    ).toBe(404);
  });

  it('publishes a snapshot when a variant goes out of stock and back', async () => {
    const { auth } = await openShop();
    const product = await createProduct(auth, [{ title: 'Unique', price: 2000, stock: 1 }]);
    const productId = product.product_id as string;
    const variantId = (product.variants as { variant_id: string }[])[0].variant_id;
    const stock = (n: number): Promise<request.Response> =>
      request(server)
        .put(`/api/v1/shops/me/products/${productId}/variants/${variantId}/stock`)
        .set('Authorization', auth)
        .send({ stock_on_hand: n });
    await stock(0);
    await stock(0);
    await stock(4);
    await stock(7);
    const snapshots = await outboxEvents(pg, 'poro.shop.product.updated', productId);
    expect(snapshots.map((s) => (s.variants as { in_stock: boolean }[])[0].in_stock)).toEqual([
      true,
      false,
      true,
    ]);
  });

  it('caps a product at ten photos', async () => {
    const { userId, auth } = await openShop();
    const productId = (await createProduct(auth)).product_id as string;
    await pg.query(
      `INSERT INTO product_images (id, product_id, key, position)
       SELECT gen_random_uuid(), $1, 'products/x/' || g, g FROM generate_series(0, 9) g`,
      [productId],
    );
    const res = await request(server)
      .post(`/api/v1/shops/me/products/${productId}/images/upload-url`)
      .set('Authorization', auth)
      .send({ content_type: 'image/webp' });
    expect([res.status, res.body.error.code]).toEqual([422, 'too_many_images']);
    expect(userId).toBeDefined();
  });

  it('serves health probes', async () => {
    const ready = await request(server).get('/health/ready');
    expect(ready.status).toBe(200);
    expect(ready.body).toMatchObject({ status: 'ok', service: 'shop' });
  });
});
