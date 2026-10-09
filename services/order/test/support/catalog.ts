import { uuidv7 } from '../../src/common/uuid';
import { Envelope, newEnvelope } from '../../src/events/envelope';

export interface Listing {
  shopId: string;
  sellerId: string;
  productId: string;
  variants: string[];
}

export function shopUpdated(
  listing: Pick<Listing, 'shopId' | 'sellerId'>,
  overrides: Record<string, unknown> = {},
): Envelope {
  return newEnvelope('poro.shop.shop.updated', 1, listing.shopId, {
    shop_id: listing.shopId,
    owner_id: listing.sellerId,
    name: 'Pagnes Awa',
    handle: 'pagnes.awa',
    description: null,
    logo_url: null,
    country_code: 'CI',
    currency: 'XOF',
    status: 'active',
    updated_at: new Date().toISOString(),
    ...overrides,
  });
}

export function productUpdated(
  listing: Listing,
  prices: number[],
  overrides: Record<string, unknown> = {},
): Envelope {
  return newEnvelope('poro.shop.product.updated', 1, listing.productId, {
    product_id: listing.productId,
    shop_id: listing.shopId,
    owner_id: listing.sellerId,
    title: 'Pagne wax',
    description: null,
    currency: 'XOF',
    status: 'active',
    image_urls: ['https://cdn.poro.test/p.webp'],
    variants: listing.variants.map((variantId, i) => ({
      variant_id: variantId,
      title: `V${i}`,
      price: prices[i],
      in_stock: true,
    })),
    updated_at: new Date().toISOString(),
    ...overrides,
  });
}

export function newListing(variantCount = 2): Listing {
  return {
    shopId: uuidv7(),
    sellerId: uuidv7(),
    productId: uuidv7(),
    variants: Array.from({ length: variantCount }, () => uuidv7()),
  };
}
