import { Prisma } from '../generated/prisma/client';

type Db = Pick<Prisma.TransactionClient, 'catalogVariant' | 'catalogShop'>;

/** A variant as the catalogue projection knows it, with its product and shop. */
export interface Offer {
  variantId: string;
  variantTitle: string;
  price: bigint;
  inStock: boolean;
  productId: string;
  productTitle: string;
  imageUrl: string | null;
  shopId: string;
  shopName: string;
  shopHandle: string;
  sellerId: string;
  currency: string;
  /** Can be bought now: live variant, active product of an active shop. */
  purchasable: boolean;
}

export async function offers(db: Db, variantIds: string[]): Promise<Map<string, Offer>> {
  const variants = await db.catalogVariant.findMany({
    where: { id: { in: variantIds } },
    include: { product: true },
  });
  const shops = await db.catalogShop.findMany({
    where: { id: { in: [...new Set(variants.map((variant) => variant.product.shopId))] } },
  });
  const shopById = new Map(shops.map((shop) => [shop.id, shop]));
  const result = new Map<string, Offer>();
  for (const variant of variants) {
    const shop = shopById.get(variant.product.shopId);
    if (!shop) {
      continue;
    }
    result.set(variant.id, {
      variantId: variant.id,
      variantTitle: variant.title,
      price: variant.price,
      inStock: variant.inStock,
      productId: variant.productId,
      productTitle: variant.product.title,
      imageUrl: variant.product.imageUrl,
      shopId: shop.id,
      shopName: shop.name,
      shopHandle: shop.handle,
      sellerId: shop.ownerId,
      currency: shop.currency,
      purchasable:
        !variant.removed &&
        variant.product.deletedAt === null &&
        variant.product.status === 'active' &&
        shop.status === 'active' &&
        variant.product.currency === shop.currency,
    });
  }
  return result;
}
