import {
  TYPE_SHOP_UPDATED,
  type CountryCode,
  type Currency,
  type ShopStatus,
  type ShopUpdatedV1,
} from '../events/catalog';
import { newEnvelope } from '../events/envelope';
import { enqueue } from '../events/outbox';
import { Prisma, Shop } from '../generated/prisma/client';

/** Fields other services see; a change to one of them publishes poro.shop.shop.updated. */
export function sameShopFields(a: Shop, b: Shop): boolean {
  return (
    a.name === b.name &&
    a.description === b.description &&
    a.logoKey === b.logoKey &&
    a.status === b.status
  );
}

export async function publishShop(
  tx: Prisma.TransactionClient,
  shop: Shop,
  url: (key: string) => string,
): Promise<void> {
  const data: ShopUpdatedV1 = {
    shop_id: shop.id,
    owner_id: shop.ownerId,
    name: shop.name,
    handle: shop.handle,
    description: shop.description,
    logo_url: shop.logoKey === null ? null : url(shop.logoKey),
    country_code: shop.countryCode as CountryCode,
    currency: shop.currency as Currency,
    status: shop.status as ShopStatus,
    updated_at: shop.updatedAt.toISOString(),
  };
  await enqueue(tx, newEnvelope(TYPE_SHOP_UPDATED, 1, shop.id, data, shop.updatedAt));
}
