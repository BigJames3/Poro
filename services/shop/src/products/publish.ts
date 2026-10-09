import {
  TYPE_PRODUCT_UPDATED,
  type Currency,
  type ProductStatus,
  type ProductUpdatedV1,
} from '../events/catalog';
import { newEnvelope } from '../events/envelope';
import { enqueue } from '../events/outbox';
import { Prisma } from '../generated/prisma/client';

/** Product rows with what views and snapshots need. */
export const productInclude = {
  shop: true,
  images: { orderBy: { position: 'asc' } },
  variants: { where: { deletedAt: null }, orderBy: { position: 'asc' } },
} satisfies Prisma.ProductInclude;

export type FullProduct = Prisma.ProductGetPayload<{ include: typeof productInclude }>;

export function available(variant: { stockOnHand: number; stockReserved: number }): number {
  return variant.stockOnHand - variant.stockReserved;
}

/**
 * Enqueues the full snapshot of productId in tx. Call it in the transaction
 * that changed the product, its images, its variants or their availability.
 * It bumps updated_at: consumers keep the snapshot with the latest one, so a
 * change outside the product row (stock, images) must still move it forward.
 */
export async function publishProduct(
  tx: Prisma.TransactionClient,
  productId: string,
  url: (key: string) => string,
): Promise<FullProduct> {
  const product = await tx.product.update({
    where: { id: productId },
    data: { updatedAt: new Date() },
    include: productInclude,
  });
  const data: ProductUpdatedV1 = {
    product_id: product.id,
    shop_id: product.shopId,
    owner_id: product.shop.ownerId,
    title: product.title,
    description: product.description,
    currency: product.shop.currency as Currency,
    status: product.status as ProductStatus,
    image_urls: product.images.map((image) => url(image.key)),
    variants: product.variants.map((variant) => ({
      variant_id: variant.id,
      title: variant.title,
      price: Number(variant.price),
      in_stock: available(variant) > 0,
    })),
    updated_at: product.updatedAt.toISOString(),
  };
  await enqueue(tx, newEnvelope(TYPE_PRODUCT_UPDATED, 1, product.id, data, product.updatedAt));
  return product;
}
