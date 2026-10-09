import { Injectable } from '@nestjs/common';

import { Offer, offers } from '../catalog/catalog.read';
import { ApiError } from '../common/api-error';
import { PrismaService } from '../prisma/prisma.service';
import { CartLineView, CartShopView, CartView, MAX_CART_LINES } from './cart.dto';

function line(variantId: string, quantity: number, offer: Offer | undefined): CartLineView {
  const available = offer?.purchasable === true;
  return {
    variant_id: variantId,
    product_id: offer?.productId ?? null,
    title: offer?.productTitle ?? null,
    variant_title: offer?.variantTitle ?? null,
    image_url: offer?.imageUrl ?? null,
    quantity,
    unit_price: offer ? Number(offer.price) : null,
    line_total: offer ? Number(offer.price * BigInt(quantity)) : null,
    in_stock: offer?.inStock === true,
    available,
  };
}

/** One cart per account; prices shown are the latest catalogue snapshot. */
@Injectable()
export class CartService {
  constructor(private readonly prisma: PrismaService) {}

  async view(buyerId: string): Promise<CartView> {
    const items = await this.prisma.cartItem.findMany({
      where: { buyerId },
      orderBy: [{ createdAt: 'asc' }, { variantId: 'asc' }],
    });
    const known = await offers(
      this.prisma,
      items.map((item) => item.variantId),
    );
    const shops = new Map<string, CartShopView>();
    const unavailable: CartLineView[] = [];
    for (const item of items) {
      const offer = known.get(item.variantId);
      const view = line(item.variantId, item.quantity, offer);
      if (!offer || !view.available) {
        unavailable.push(view);
        continue;
      }
      let group = shops.get(offer.shopId);
      if (!group) {
        group = {
          shop: {
            shop_id: offer.shopId,
            name: offer.shopName,
            handle: offer.shopHandle,
            currency: offer.currency,
          },
          items: [],
          subtotal: 0,
        };
        shops.set(offer.shopId, group);
      }
      group.items.push(view);
      group.subtotal += view.line_total ?? 0;
    }
    return {
      shops: [...shops.values()],
      unavailable,
      item_count: items.reduce((sum, item) => sum + item.quantity, 0),
    };
  }

  /** Sets the quantity of a variant; it must be on sale and not from the buyer's own shop. */
  async setItem(buyerId: string, variantId: string, quantity: number): Promise<CartView> {
    const offer = (await offers(this.prisma, [variantId])).get(variantId);
    if (!offer?.purchasable) {
      throw new ApiError(422, 'variant_unavailable', 'this item is not on sale');
    }
    if (offer.sellerId === buyerId) {
      throw new ApiError(422, 'own_shop', 'you cannot buy from your own shop');
    }
    await this.prisma.$transaction(async (tx) => {
      // Serialises the cart of one buyer so the line limit holds under concurrency.
      await tx.$executeRaw`SELECT pg_advisory_xact_lock(hashtext(${buyerId}))`;
      const existing = await tx.cartItem.findUnique({
        where: { buyerId_variantId: { buyerId, variantId } },
      });
      if (!existing && (await tx.cartItem.count({ where: { buyerId } })) >= MAX_CART_LINES) {
        throw new ApiError(422, 'cart_full', `a cart holds at most ${MAX_CART_LINES} items`);
      }
      await tx.cartItem.upsert({
        where: { buyerId_variantId: { buyerId, variantId } },
        create: { buyerId, variantId, quantity },
        update: { quantity },
      });
    });
    return this.view(buyerId);
  }

  async removeItem(buyerId: string, variantId: string): Promise<CartView> {
    await this.prisma.cartItem.deleteMany({ where: { buyerId, variantId } });
    return this.view(buyerId);
  }

  async clear(buyerId: string): Promise<CartView> {
    await this.prisma.cartItem.deleteMany({ where: { buyerId } });
    return this.view(buyerId);
  }
}
