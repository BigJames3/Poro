import {
  Inject,
  Injectable,
  Logger,
  OnApplicationBootstrap,
  OnApplicationShutdown,
} from '@nestjs/common';

import { appConfig, type AppConfigType } from '../config/app.config';
import { TYPE_PRODUCT_DELETED, TYPE_PRODUCT_UPDATED, TYPE_SHOP_UPDATED } from '../events/catalog';
import { Envelope } from '../events/envelope';
import { KafkaEventConsumer, PermanentError } from '../events/event-consumer';
import { Data, amountField, arrayField, stringField, timeField, uuidField } from '../events/fields';
import { claim } from '../events/outbox';
import { PrismaService } from '../prisma/prisma.service';

export const CATALOG_CONSUMER_GROUP = 'poro-order-catalog';
export const CATALOG_TOPICS = [TYPE_SHOP_UPDATED, TYPE_PRODUCT_UPDATED, TYPE_PRODUCT_DELETED];

/**
 * Keeps a local copy of the shop catalogue so carts and checkout never call
 * the shop service. Snapshots older than the stored one are ignored.
 */
@Injectable()
export class CatalogConsumer implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly logger = new Logger(CatalogConsumer.name);
  private consumer: KafkaEventConsumer | undefined;

  constructor(
    private readonly prisma: PrismaService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  onApplicationBootstrap(): void {
    if (!this.config.kafkaConsumerEnabled) {
      this.logger.warn('kafka consumer disabled by KAFKA_CONSUMER_ENABLED=false');
      return;
    }
    this.consumer = new KafkaEventConsumer(
      this.config.kafkaBrokers,
      { group: CATALOG_CONSUMER_GROUP, topics: CATALOG_TOPICS },
      (env) => this.handle(env),
    );
    this.consumer.start();
  }

  async onApplicationShutdown(): Promise<void> {
    await this.consumer?.stop();
  }

  async handle(env: Envelope): Promise<void> {
    if (!CATALOG_TOPICS.includes(env.type) || env.version !== 1) {
      throw new PermanentError(`unsupported event ${env.type} v${env.version}`);
    }
    const data = env.data as Data;
    if (env.type === TYPE_SHOP_UPDATED) {
      return this.shopUpdated(env.id, data);
    }
    if (env.type === TYPE_PRODUCT_UPDATED) {
      return this.productUpdated(env.id, data);
    }
    return this.productDeleted(env.id, data);
  }

  private async shopUpdated(eventId: string, data: Data): Promise<void> {
    const shop = {
      id: uuidField(data, 'shop_id'),
      ownerId: uuidField(data, 'owner_id'),
      name: stringField(data, 'name'),
      handle: stringField(data, 'handle'),
      currency: stringField(data, 'currency'),
      status: stringField(data, 'status'),
      sourceUpdatedAt: timeField(data, 'updated_at'),
    };
    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, CATALOG_CONSUMER_GROUP, eventId))) {
        return;
      }
      await tx.$executeRaw`
        INSERT INTO catalog_shops (id, owner_id, name, handle, currency, status, source_updated_at)
        VALUES (${shop.id}::uuid, ${shop.ownerId}::uuid, ${shop.name}, ${shop.handle},
                ${shop.currency}, ${shop.status}, ${shop.sourceUpdatedAt})
        ON CONFLICT (id) DO UPDATE SET
          owner_id = EXCLUDED.owner_id, name = EXCLUDED.name, handle = EXCLUDED.handle,
          currency = EXCLUDED.currency, status = EXCLUDED.status,
          source_updated_at = EXCLUDED.source_updated_at
        WHERE catalog_shops.source_updated_at < EXCLUDED.source_updated_at`;
    });
  }

  private async productUpdated(eventId: string, data: Data): Promise<void> {
    const productId = uuidField(data, 'product_id');
    const imageUrls = data.image_urls;
    const product = {
      shopId: uuidField(data, 'shop_id'),
      title: stringField(data, 'title'),
      currency: stringField(data, 'currency'),
      status: stringField(data, 'status'),
      imageUrl: Array.isArray(imageUrls) && typeof imageUrls[0] === 'string' ? imageUrls[0] : null,
      sourceUpdatedAt: timeField(data, 'updated_at'),
    };
    const variants = arrayField(data, 'variants').map((variant) => ({
      id: uuidField(variant, 'variant_id'),
      title: stringField(variant, 'title'),
      price: amountField(variant, 'price'),
      inStock: variant.in_stock === true,
    }));
    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, CATALOG_CONSUMER_GROUP, eventId))) {
        return;
      }
      await tx.$queryRaw`SELECT 1 FROM catalog_products WHERE id = ${productId}::uuid FOR UPDATE`;
      const stored = await tx.catalogProduct.findUnique({ where: { id: productId } });
      if (stored && stored.sourceUpdatedAt >= product.sourceUpdatedAt) {
        return;
      }
      await tx.catalogProduct.upsert({
        where: { id: productId },
        create: { id: productId, ...product },
        update: product,
      });
      await tx.catalogVariant.updateMany({
        where: { productId, id: { notIn: variants.map((variant) => variant.id) } },
        data: { removed: true },
      });
      for (const variant of variants) {
        await tx.catalogVariant.upsert({
          where: { id: variant.id },
          create: { ...variant, productId },
          update: { ...variant, productId, removed: false },
        });
      }
    });
  }

  private async productDeleted(eventId: string, data: Data): Promise<void> {
    const productId = uuidField(data, 'product_id');
    const shopId = uuidField(data, 'shop_id');
    const deletedAt = timeField(data, 'deleted_at');
    await this.prisma.$transaction(async (tx) => {
      if (!(await claim(tx, CATALOG_CONSUMER_GROUP, eventId))) {
        return;
      }
      // A tombstone without snapshot keeps a late product.updated from reviving it.
      await tx.catalogProduct.upsert({
        where: { id: productId },
        create: {
          id: productId,
          shopId,
          title: 'deleted',
          currency: 'XOF',
          status: 'archived',
          sourceUpdatedAt: deletedAt,
          deletedAt,
        },
        update: { deletedAt, sourceUpdatedAt: deletedAt },
      });
      await tx.catalogVariant.updateMany({ where: { productId }, data: { removed: true } });
    });
  }
}
