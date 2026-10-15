import { DeleteObjectCommand, ListObjectsV2Command, S3Client } from '@aws-sdk/client-s3';
import { NestExpressApplication } from '@nestjs/platform-express';
import { mockClient } from 'aws-sdk-client-mock';
import { Client } from 'pg';

import { AccountErasureConsumer } from '../src/account/account-erasure.consumer';
import { uuidv7 } from '../src/common/uuid';
import { Envelope, newEnvelope } from '../src/events/envelope';
import { PermanentError } from '../src/events/event-consumer';
import { ProductsService } from '../src/products/products.service';
import { ShopsService } from '../src/shops/shops.service';
import { createApp } from './support/app';
import { outboxEvents } from './support/outbox';
import { TestDatabase, startDatabase } from './support/postgres';

const s3 = mockClient(S3Client);
const CDN = 'https://cdn.poro.test/shop';
const BUCKET = 'poro-shop';
const UPLOADS = 'poro-shop-uploads';

interface Seller {
  ownerId: string;
  shopId: string;
  handle: string;
  /** active with photos, draft with a photo, archived without, deleted with a photo */
  products: { active: string; draft: string; archived: string; deleted: string };
}

describe('account erasure', () => {
  let db: TestDatabase;
  let pg: Client;
  let app: NestExpressApplication;
  let consumer: AccountErasureConsumer;
  let handles = 0;
  /** Objects per bucket, as the mocked storage holds them. */
  const objects = new Map<string, Set<string>>();

  beforeAll(async () => {
    db = await startDatabase();
    pg = new Client({ connectionString: db.url });
    await pg.connect();
    app = await createApp({
      DATABASE_URL: db.url,
      JWKS_URL: 'http://127.0.0.1:1/jwks.json',
      MEDIA_PUBLIC_BASE_URL: CDN,
    });
    consumer = app.get(AccountErasureConsumer);
  });

  afterAll(async () => {
    await (app as NestExpressApplication | undefined)?.close();
    await (pg as Client | undefined)?.end();
    await (db as TestDatabase | undefined)?.container.stop();
  });

  beforeEach(() => {
    objects.clear();
    s3.reset();
    // One key per page, so the deletion follows continuation tokens.
    s3.on(ListObjectsV2Command).callsFake((input: ListObjectsV2Command['input']) => {
      const keys = [...(objects.get(input.Bucket ?? '') ?? [])]
        .filter((key) => key.startsWith(input.Prefix ?? ''))
        .filter((key) => input.ContinuationToken === undefined || key > input.ContinuationToken)
        .sort();
      return {
        Contents: keys.slice(0, 1).map((Key) => ({ Key })),
        IsTruncated: keys.length > 1,
        NextContinuationToken: keys.length > 1 ? keys[0] : undefined,
      };
    });
    s3.on(DeleteObjectCommand).callsFake(remove);
  });

  function remove(input: DeleteObjectCommand['input']): object {
    objects.get(input.Bucket ?? '')?.delete(input.Key ?? '');
    return {};
  }

  function store(bucket: string, key: string): string {
    const keys = objects.get(bucket) ?? new Set<string>();
    keys.add(key);
    objects.set(bucket, keys);
    return key;
  }

  function stored(bucket: string, prefix: string): string[] {
    return [...(objects.get(bucket) ?? [])].filter((key) => key.startsWith(prefix)).sort();
  }

  async function addImage(shopId: string, productId: string): Promise<void> {
    const key = store(BUCKET, `products/${shopId}/${productId}/${uuidv7()}.webp`);
    await pg.query(
      `INSERT INTO product_images (id, product_id, key, position)
       VALUES ($1, $2, $3, (SELECT count(*) FROM product_images WHERE product_id = $2))`,
      [uuidv7(), productId, key],
    );
  }

  async function seller(status = 'active'): Promise<Seller> {
    const ownerId = uuidv7();
    handles += 1;
    const handle = `erased_shop${handles}`;
    const shop = await app.get(ShopsService).create(ownerId, {
      name: 'Pagnes Awa',
      handle,
      description: 'Wax et bazin, livraison à Abidjan',
      country_code: 'CI',
    });
    const logo = store(BUCKET, `shops/${shop.shop_id}/${uuidv7()}.webp`);
    await pg.query(`UPDATE shops SET logo_key = $2 WHERE id = $1`, [shop.shop_id, logo]);
    store(UPLOADS, `uploads/${ownerId}/${uuidv7()}`);
    const products = app.get(ProductsService);
    const ids: string[] = [];
    for (const title of ['Pagne wax', 'Bazin', 'Boubou', 'Foulard']) {
      const product = await products.create(ownerId, {
        title,
        variants: [{ title: 'Unique', price: 5000, stock: 2 }],
      });
      ids.push(product.product_id);
    }
    const [active, draft, archived, deleted] = ids;
    await addImage(shop.shop_id, active);
    await addImage(shop.shop_id, active);
    await addImage(shop.shop_id, draft);
    await addImage(shop.shop_id, deleted);
    await pg.query(`UPDATE products SET status = 'active' WHERE id = $1`, [active]);
    await pg.query(`UPDATE products SET status = 'archived' WHERE id = $1`, [archived]);
    await pg.query(`UPDATE products SET deleted_at = now() WHERE id = $1`, [deleted]);
    await pg.query(`UPDATE shops SET status = $2 WHERE id = $1`, [shop.shop_id, status]);
    return {
      ownerId,
      shopId: shop.shop_id,
      handle,
      products: { active, draft, archived, deleted },
    };
  }

  function deleted(userId: string): Envelope {
    return newEnvelope('poro.auth.user.deleted', 1, userId, {
      user_id: userId,
      deleted_at: new Date().toISOString(),
    });
  }

  async function shopRow(shopId: string): Promise<Record<string, unknown>> {
    const { rows } = await pg.query(
      'SELECT name, handle, description, logo_key, status FROM shops WHERE id = $1',
      [shopId],
    );
    return rows[0] as Record<string, unknown>;
  }

  async function productRows(shopId: string): Promise<Record<string, unknown>[]> {
    const { rows } = await pg.query(
      `SELECT p.id, p.status, p.deleted_at IS NOT NULL AS deleted,
              (SELECT count(*)::int FROM product_images i WHERE i.product_id = p.id) AS images
         FROM products p WHERE p.shop_id = $1 ORDER BY p.id`,
      [shopId],
    );
    return rows as Record<string, unknown>[];
  }

  async function events(s: Seller): Promise<number> {
    const { rows } = await pg.query<{ n: number }>(
      `SELECT count(*)::int AS n FROM outbox_events
        WHERE event_key = ANY($1::text[])`,
      [[s.shopId, ...Object.values(s.products)]],
    );
    return rows[0].n;
  }

  async function claimed(eventId: string): Promise<boolean> {
    const { rows } = await pg.query(
      `SELECT 1 FROM processed_events WHERE consumer = 'poro-shop-account-erasure' AND event_id = $1`,
      [eventId],
    );
    return rows.length === 1;
  }

  it('closes a suspended shop, archives its products and clears its media', async () => {
    const s = await seller('suspended');
    const other = await seller();
    const otherShop = await shopRow(other.shopId);
    const otherProducts = await productRows(other.shopId);
    const otherObjects = [
      ...stored(BUCKET, `shops/${other.shopId}/`),
      ...stored(BUCKET, `products/${other.shopId}/`),
      ...stored(UPLOADS, `uploads/${other.ownerId}/`),
    ];
    const otherEvents = await events(other);
    const env = deleted(s.ownerId);

    await consumer.handle(env);

    expect(await shopRow(s.shopId)).toEqual({
      name: 'Pagnes Awa',
      handle: s.handle,
      description: null,
      logo_key: null,
      status: 'closed',
    });
    const { active, draft, archived } = s.products;
    expect(await productRows(s.shopId)).toEqual(
      [
        { id: active, status: 'archived', deleted: false, images: 0 },
        { id: draft, status: 'archived', deleted: false, images: 0 },
        { id: archived, status: 'archived', deleted: false, images: 0 },
        { id: s.products.deleted, status: 'draft', deleted: true, images: 0 },
      ].sort((a, b) => a.id.localeCompare(b.id)),
    );
    expect(stored(BUCKET, `shops/${s.shopId}/`)).toEqual([]);
    expect(stored(BUCKET, `products/${s.shopId}/`)).toEqual([]);
    expect(stored(UPLOADS, `uploads/${s.ownerId}/`)).toEqual([]);

    const shopUpdates = await outboxEvents(pg, 'poro.shop.shop.updated', s.shopId);
    expect(shopUpdates.at(-1)).toMatchObject({
      shop_id: s.shopId,
      name: 'Pagnes Awa',
      handle: s.handle,
      description: null,
      logo_url: null,
      status: 'closed',
    });
    for (const id of [active, draft]) {
      const updates = await outboxEvents(pg, 'poro.shop.product.updated', id);
      expect(updates.at(-1)).toMatchObject({ product_id: id, status: 'archived', image_urls: [] });
    }
    // Already archived without photos, or deleted: nothing new to publish.
    expect(await outboxEvents(pg, 'poro.shop.product.updated', archived)).toHaveLength(1);
    expect(await outboxEvents(pg, 'poro.shop.product.updated', s.products.deleted)).toHaveLength(1);
    expect(await claimed(env.id)).toBe(true);

    expect(await shopRow(other.shopId)).toEqual(otherShop);
    expect(await productRows(other.shopId)).toEqual(otherProducts);
    expect([
      ...stored(BUCKET, `shops/${other.shopId}/`),
      ...stored(BUCKET, `products/${other.shopId}/`),
      ...stored(UPLOADS, `uploads/${other.ownerId}/`),
    ]).toEqual(otherObjects);
    expect(await events(other)).toBe(otherEvents);
  });

  it('applies an event once and a second erasure of the account changes nothing', async () => {
    const s = await seller();
    const env = deleted(s.ownerId);
    await consumer.handle(env);
    const published = await events(s);
    const shop = await shopRow(s.shopId);
    const listed = s3.commandCalls(ListObjectsV2Command).length;

    await consumer.handle(env);
    // The replay of a claimed event does not even reach storage.
    expect(s3.commandCalls(ListObjectsV2Command)).toHaveLength(listed);

    const again = deleted(s.ownerId);
    await consumer.handle(again);

    expect(await events(s)).toBe(published);
    expect(await shopRow(s.shopId)).toEqual(shop);
    expect(await claimed(again.id)).toBe(true);
  });

  it('retries when storage fails and publishes each snapshot once', async () => {
    const s = await seller();
    const env = deleted(s.ownerId);
    s3.on(DeleteObjectCommand).rejectsOnce(new Error('storage down'));

    await expect(consumer.handle(env)).rejects.toThrow('storage down');
    expect(await claimed(env.id)).toBe(false);
    expect((await shopRow(s.shopId)).status).toBe('closed');
    const published = await events(s);

    s3.on(DeleteObjectCommand).callsFake(remove);
    await consumer.handle(env);

    expect(await claimed(env.id)).toBe(true);
    expect(await events(s)).toBe(published);
    expect(stored(BUCKET, `shops/${s.shopId}/`)).toEqual([]);
    expect(stored(BUCKET, `products/${s.shopId}/`)).toEqual([]);
    expect(stored(UPLOADS, `uploads/${s.ownerId}/`)).toEqual([]);
  });

  it('clears the pending uploads of an account without a shop', async () => {
    const userId = uuidv7();
    store(UPLOADS, `uploads/${userId}/${uuidv7()}`);
    const env = deleted(userId);

    await consumer.handle(env);

    expect(stored(UPLOADS, `uploads/${userId}/`)).toEqual([]);
    expect(await claimed(env.id)).toBe(true);
  });

  it.each([
    ['another type', { type: 'poro.auth.user.created' }],
    ['another version', { version: 2 }],
    [
      'a user_id that is not a UUID',
      { data: { user_id: 'x', deleted_at: '2026-10-15T00:00:00Z' } },
    ],
    ['no user_id', { data: { deleted_at: '2026-10-15T00:00:00Z' } }],
  ])('dead-letters %s before any change', async (_name, overrides) => {
    const s = await seller();
    const shop = await shopRow(s.shopId);
    const env = { ...deleted(s.ownerId), ...overrides } as Envelope;

    await expect(consumer.handle(env)).rejects.toBeInstanceOf(PermanentError);

    expect(await shopRow(s.shopId)).toEqual(shop);
    expect(await claimed(env.id)).toBe(false);
    expect(s3.commandCalls(ListObjectsV2Command)).toHaveLength(0);
  });
});
