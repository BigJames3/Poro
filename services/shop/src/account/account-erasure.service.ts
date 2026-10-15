import { Injectable } from '@nestjs/common';

import { claim } from '../events/outbox';
import { Prisma } from '../generated/prisma/client';
import { MediaService } from '../media/media.service';
import { PrismaService } from '../prisma/prisma.service';
import { publishProduct } from '../products/publish';
import { publishShop, sameShopFields } from '../shops/publish';

export const ACCOUNT_ERASURE_GROUP = 'poro-shop-account-erasure';

type Tx = Prisma.TransactionClient;

/**
 * Erases what the shop of a deleted account exposes (ADR-0018): the shop is
 * closed whatever its status, its description and logo are cleared, its
 * products are archived without photos, and its images and pending uploads
 * leave storage. The name and handle stay: past orders refer to them.
 */
@Injectable()
export class AccountErasureService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly media: MediaService,
  ) {}

  /**
   * The database effects commit first and are idempotent: a second run finds
   * nothing to change and publishes nothing. Storage is then cleared by prefix,
   * from the shop id that the database keeps, so a run that failed on storage
   * is retried in full. The inbox is claimed last, once storage is clear.
   */
  async erase(eventId: string, userId: string): Promise<void> {
    const done = await this.prisma.processedEvent.findFirst({
      where: { consumer: ACCOUNT_ERASURE_GROUP, eventId },
    });
    if (done) {
      return;
    }
    const shopId = await this.prisma.$transaction((tx) => this.closeShop(tx, userId));
    if (shopId !== null) {
      await this.media.removeAll(`shops/${shopId}/`);
      await this.media.removeAll(`products/${shopId}/`);
    }
    await this.media.removeUploadsOf(userId);
    await this.prisma.$transaction((tx) => claim(tx, ACCOUNT_ERASURE_GROUP, eventId));
  }

  /** Returns the id of the shop of userId, or null when the account never opened one. */
  private async closeShop(tx: Tx, userId: string): Promise<string | null> {
    const locked = await tx.$queryRaw<{ id: string }[]>`
      SELECT id::text FROM shops WHERE owner_id = ${userId}::uuid FOR UPDATE`;
    const shopId = locked.at(0)?.id;
    if (shopId === undefined) {
      return null;
    }
    const before = await tx.shop.findUniqueOrThrow({ where: { id: shopId } });
    const after = await tx.shop.update({
      where: { id: before.id },
      data: { status: 'closed', description: null, logoKey: null },
    });
    if (!sameShopFields(before, after)) {
      await publishShop(tx, after, this.url);
    }

    // Same order as the seller routes and the stock saga: products by id.
    await tx.$queryRaw`
      SELECT 1 FROM products WHERE shop_id = ${before.id}::uuid ORDER BY id FOR UPDATE`;
    const products = await tx.product.findMany({
      where: { shopId: before.id },
      select: { id: true, status: true, deletedAt: true, _count: { select: { images: true } } },
      orderBy: { id: 'asc' },
    });
    for (const product of products) {
      const hadImages = product._count.images > 0;
      if (hadImages) {
        await tx.productImage.deleteMany({ where: { productId: product.id } });
      }
      // A deleted product already published product.deleted: no snapshot to send.
      if (product.deletedAt !== null || (product.status === 'archived' && !hadImages)) {
        continue;
      }
      await tx.product.update({ where: { id: product.id }, data: { status: 'archived' } });
      await publishProduct(tx, product.id, this.url);
    }
    return before.id;
  }

  private readonly url = (key: string): string => this.media.publicUrl(key);
}
