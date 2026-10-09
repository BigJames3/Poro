import { Inject, Injectable } from '@nestjs/common';
import { createHash } from 'node:crypto';

import { offers } from '../catalog/catalog.read';
import { ApiError } from '../common/api-error';
import { cleanLine, cleanText } from '../common/text';
import { uuidv7 } from '../common/uuid';
import { appConfig, type AppConfigType } from '../config/app.config';
import { Prisma } from '../generated/prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import {
  CheckoutDto,
  CheckoutView,
  MAX_LINES_PER_ORDER,
  OrderWithLines,
  toOrderView,
} from './order.dto';
import { publishCreated } from './publish';

const IDEMPOTENCY_KEY = /^[A-Za-z0-9_-]{8,100}$/;

function isUniqueViolation(err: unknown): boolean {
  return err instanceof Prisma.PrismaClientKnownRequestError && err.code === 'P2002';
}

interface Delivery {
  contactName: string;
  contactPhone: string;
  city: string;
  address: string;
}

/**
 * Turns the cart into one order per shop. Each order starts pending and asks
 * the shop to reserve its stock with poro.order.order.created.
 */
@Injectable()
export class CheckoutService {
  constructor(
    private readonly prisma: PrismaService,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  async checkout(
    buyerId: string,
    dto: CheckoutDto,
    idempotencyKey: string | undefined,
  ): Promise<CheckoutView> {
    if (idempotencyKey !== undefined && !IDEMPOTENCY_KEY.test(idempotencyKey)) {
      throw new ApiError(
        400,
        'invalid_idempotency_key',
        'Idempotency-Key must be 8 to 100 characters',
      );
    }
    if (!this.config.paymentMethods.includes(dto.payment_method)) {
      throw new ApiError(422, 'payment_method_unavailable', 'this payment method is not available');
    }
    const delivery: Delivery = {
      contactName: cleanLine(dto.delivery.name, 80, 'delivery_invalid', 'delivery name'),
      contactPhone: dto.delivery.phone,
      city: cleanLine(dto.delivery.city, 80, 'delivery_invalid', 'delivery city'),
      address:
        cleanText(dto.delivery.address, 300, 3, 'delivery_invalid', 'delivery address') ?? '',
    };
    if (delivery.address === '') {
      throw new ApiError(422, 'delivery_invalid', 'delivery address is required');
    }
    const requestHash = createHash('sha256')
      .update(JSON.stringify({ method: dto.payment_method, delivery }))
      .digest('hex');
    const key = idempotencyKey ?? uuidv7();

    const replay = await this.replay(buyerId, key, requestHash);
    if (replay) {
      return replay;
    }
    try {
      return await this.create(buyerId, dto, delivery, key, requestHash);
    } catch (err) {
      // A concurrent request with the same key won the race: answer like it.
      if (isUniqueViolation(err)) {
        const winner = await this.replay(buyerId, key, requestHash);
        if (winner) {
          return winner;
        }
      }
      throw err;
    }
  }

  private async create(
    buyerId: string,
    dto: CheckoutDto,
    delivery: Delivery,
    key: string,
    requestHash: string,
  ): Promise<CheckoutView> {
    return this.prisma.$transaction(async (tx) => {
      await tx.$executeRaw`SELECT pg_advisory_xact_lock(hashtext(${buyerId}))`;
      const items = await tx.cartItem.findMany({
        where: { buyerId },
        orderBy: [{ createdAt: 'asc' }, { variantId: 'asc' }],
      });
      if (items.length === 0) {
        throw new ApiError(422, 'cart_empty', 'your cart is empty');
      }
      const known = await offers(
        tx,
        items.map((item) => item.variantId),
      );
      const groups = new Map<string, typeof items>();
      for (const item of items) {
        const offer = known.get(item.variantId);
        if (!offer?.purchasable) {
          throw new ApiError(
            422,
            'cart_unavailable',
            'some items of your cart are no longer on sale: remove them first',
          );
        }
        if (offer.sellerId === buyerId) {
          throw new ApiError(422, 'own_shop', 'you cannot buy from your own shop');
        }
        const group = groups.get(offer.shopId) ?? [];
        group.push(item);
        groups.set(offer.shopId, group);
      }
      for (const group of groups.values()) {
        if (group.length > MAX_LINES_PER_ORDER) {
          throw new ApiError(
            422,
            'too_many_lines',
            `an order holds at most ${MAX_LINES_PER_ORDER} different items per shop`,
          );
        }
      }

      const checkout = await tx.checkout.create({
        data: { id: uuidv7(), buyerId, idempotencyKey: key, requestHash },
      });
      const orders: OrderWithLines[] = [];
      for (const group of groups.values()) {
        const first = known.get(group[0].variantId);
        if (!first) {
          continue;
        }
        const lines = group.map((item, position) => {
          const offer = known.get(item.variantId);
          return {
            position,
            productId: offer?.productId ?? '',
            variantId: item.variantId,
            title: `${offer?.productTitle ?? ''} — ${offer?.variantTitle ?? ''}`.slice(0, 200),
            quantity: item.quantity,
            unitPriceSeen: offer?.price ?? 0n,
          };
        });
        const order = await tx.order.create({
          data: {
            id: uuidv7(),
            checkoutId: checkout.id,
            buyerId,
            shopId: first.shopId,
            shopName: first.shopName,
            sellerId: first.sellerId,
            currency: first.currency,
            paymentMethod: dto.payment_method,
            status: 'pending',
            subtotalSeen: lines.reduce((sum, l) => sum + l.unitPriceSeen * BigInt(l.quantity), 0n),
            ...delivery,
            lines: { create: lines },
          },
          include: { lines: true },
        });
        await publishCreated(tx, order);
        orders.push(order);
      }
      await tx.cartItem.deleteMany({
        where: { buyerId, variantId: { in: items.map((item) => item.variantId) } },
      });
      return { checkout_id: checkout.id, orders: orders.map(toOrderView) };
    });
  }

  private async replay(
    buyerId: string,
    key: string,
    requestHash: string,
  ): Promise<CheckoutView | undefined> {
    const previous = await this.prisma.checkout.findUnique({
      where: { buyerId_idempotencyKey: { buyerId, idempotencyKey: key } },
      include: { orders: { include: { lines: true }, orderBy: { id: 'asc' } } },
    });
    if (!previous) {
      return undefined;
    }
    if (previous.requestHash !== requestHash) {
      throw new ApiError(
        409,
        'idempotency_key_reused',
        'this Idempotency-Key was used for a different checkout',
      );
    }
    return { checkout_id: previous.id, orders: previous.orders.map(toOrderView) };
  }
}
