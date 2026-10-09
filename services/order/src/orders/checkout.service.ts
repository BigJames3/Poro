import { Inject, Injectable } from '@nestjs/common';
import { createHash } from 'node:crypto';

import { saveAddress, AddressesService } from '../addresses/addresses.service';
import { Offer, offers } from '../catalog/catalog.read';
import { ApiError } from '../common/api-error';
import { cleanText } from '../common/text';
import { uuidv7 } from '../common/uuid';
import { appConfig, type AppConfigType, type PaymentMethod } from '../config/app.config';
import { AccountClient } from '../delivery/account.client';
import {
  DELIVERY_FEE_NOTICE,
  Delivery,
  PAYMENT_METHOD_LABELS,
  cleanDelivery,
  toDeliveryView,
} from '../delivery/delivery';
import { CartItem, Prisma } from '../generated/prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import {
  CheckoutView,
  ConfirmDto,
  MAX_LINES_PER_ORDER,
  OrderWithLines,
  PrefillView,
  PreviewDto,
  PreviewShopView,
  PreviewView,
  toOrderView,
} from './order.dto';
import { publishCreated } from './publish';

type Tx = Prisma.TransactionClient;

/** What the buyer reviewed, stored with the preview and replayed at confirmation. */
interface PreviewRequest {
  delivery: Delivery;
  paymentMethod: PaymentMethod;
  saveAddress: boolean;
  addressLabel: string | null;
}

/** The cart checked against the catalogue, grouped by shop in cart order. */
interface CheckedCart {
  items: CartItem[];
  known: Map<string, Offer>;
  groups: Map<string, CartItem[]>;
  hash: string;
}

/**
 * Checkout in two steps. preview() validates the cart and the delivery and
 * returns the full recap; confirm() creates the orders only on the buyer's
 * explicit confirmation, and only while the cart still matches the recap.
 */
@Injectable()
export class CheckoutService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly addresses: AddressesService,
    private readonly account: AccountClient,
    @Inject(appConfig.KEY) private readonly config: AppConfigType,
  ) {}

  /** The saved address to start from, else what the account knows. */
  async prefill(buyerId: string, authorization: string): Promise<PrefillView> {
    const saved = await this.addresses.preferred(buyerId);
    const savedView = saved ? toDeliveryView(saved) : null;
    if (saved && savedView) {
      return {
        source: 'address_book',
        address_id: saved.id,
        delivery: savedView,
        account_lookup: 'skipped',
      };
    }
    const contact = await this.account.contact(authorization);
    return {
      source: contact.fullName === null && contact.phone === null ? 'empty' : 'account',
      address_id: null,
      delivery: {
        full_name: contact.fullName,
        phone: contact.phone,
        city: null,
        address: null,
        landmark: null,
        location: null,
      },
      account_lookup: contact.lookup,
    };
  }

  async preview(buyerId: string, dto: PreviewDto): Promise<PreviewView> {
    const paymentMethod = dto.payment_method ?? 'cash_on_delivery';
    this.checkMethod(paymentMethod);
    const request: PreviewRequest = {
      delivery: cleanDelivery(dto.delivery),
      paymentMethod,
      saveAddress: dto.save_address === true,
      addressLabel: cleanText(dto.address_label ?? null, 40, 1, 'address_label_invalid', 'label'),
    };
    const cart = await this.checkedCart(this.prisma, buyerId);
    const expiresAt = new Date(Date.now() + this.config.previewTtlMinutes * 60_000);
    const preview = await this.prisma.checkoutPreview.create({
      data: {
        id: uuidv7(),
        buyerId,
        cartHash: cart.hash,
        request: request as unknown as Prisma.InputJsonObject,
        expiresAt,
      },
    });
    const shops: PreviewShopView[] = [];
    const totals = new Map<string, bigint>();
    for (const group of cart.groups.values()) {
      const first = cart.known.get(group[0].variantId);
      if (!first) {
        continue;
      }
      let subtotal = 0n;
      const items = group.map((item) => {
        const offer = cart.known.get(item.variantId) ?? first;
        const lineTotal = offer.price * BigInt(item.quantity);
        subtotal += lineTotal;
        return {
          product_id: offer.productId,
          variant_id: item.variantId,
          title: offer.productTitle,
          variant_title: offer.variantTitle,
          image_url: offer.imageUrl,
          quantity: item.quantity,
          unit_price: Number(offer.price),
          line_total: Number(lineTotal),
        };
      });
      totals.set(first.currency, (totals.get(first.currency) ?? 0n) + subtotal);
      shops.push({
        shop: { shop_id: first.shopId, name: first.shopName, handle: first.shopHandle },
        currency: first.currency,
        items,
        subtotal: Number(subtotal),
      });
    }
    const delivery = toDeliveryView(request.delivery);
    if (!delivery) {
      throw new Error('a cleaned delivery always has a contact');
    }
    return {
      preview_id: preview.id,
      expires_at: expiresAt.toISOString(),
      shops,
      totals: [...totals].map(([currency, amount]) => ({ currency, amount: Number(amount) })),
      item_count: cart.items.reduce((sum, item) => sum + item.quantity, 0),
      delivery,
      payment_method: paymentMethod,
      payment_method_label: PAYMENT_METHOD_LABELS[paymentMethod] ?? paymentMethod,
      delivery_fee_notice: DELIVERY_FEE_NOTICE,
    };
  }

  /** Creates the orders of a preview. Confirming it again returns the same orders. */
  async confirm(buyerId: string, dto: ConfirmDto): Promise<CheckoutView> {
    if (!dto.confirmed) {
      throw new ApiError(422, 'confirmation_required', 'confirm the recap to place the order');
    }
    const previewId = dto.preview_id.toLowerCase();
    return this.prisma.$transaction(async (tx) => {
      // Serialises the checkouts of one buyer: a double tap confirms once.
      await tx.$executeRaw`SELECT pg_advisory_xact_lock(hashtext(${buyerId}))`;
      const preview = await tx.checkoutPreview.findFirst({ where: { id: previewId, buyerId } });
      if (!preview) {
        throw new ApiError(404, 'preview_not_found', 'recap not found');
      }
      if (preview.checkoutId !== null) {
        return this.replay(tx, preview.checkoutId);
      }
      if (preview.expiresAt < new Date()) {
        throw new ApiError(409, 'preview_expired', 'the recap expired: review your order again');
      }
      const request = preview.request as unknown as PreviewRequest;
      this.checkMethod(request.paymentMethod);
      const cart = await this.checkedCart(tx, buyerId);
      if (cart.hash !== preview.cartHash) {
        throw new ApiError(
          409,
          'preview_outdated',
          'your cart or its prices changed: review your order again',
        );
      }

      const checkout = await tx.checkout.create({
        data: { id: uuidv7(), buyerId, idempotencyKey: preview.id, requestHash: cart.hash },
      });
      const orders: OrderWithLines[] = [];
      for (const group of cart.groups.values()) {
        orders.push(await this.createOrder(tx, checkout.id, buyerId, group, cart.known, request));
      }
      await tx.cartItem.deleteMany({
        where: { buyerId, variantId: { in: cart.items.map((item) => item.variantId) } },
      });
      await tx.checkoutPreview.update({
        where: { id: preview.id },
        data: { checkoutId: checkout.id, confirmedAt: new Date() },
      });
      const addressSaved = request.saveAddress
        ? (await saveAddress(tx, buyerId, request.delivery, request.addressLabel, false)) !== null
        : false;
      return {
        checkout_id: checkout.id,
        orders: orders.map(toOrderView),
        address_saved: addressSaved,
      };
    });
  }

  private checkMethod(method: PaymentMethod): void {
    if (!this.config.paymentMethods.includes(method)) {
      throw new ApiError(422, 'payment_method_unavailable', 'this payment method is not available');
    }
  }

  private async createOrder(
    tx: Tx,
    checkoutId: string,
    buyerId: string,
    group: CartItem[],
    known: Map<string, Offer>,
    request: PreviewRequest,
  ): Promise<OrderWithLines> {
    const offerOf = (item: CartItem): Offer => {
      const offer = known.get(item.variantId);
      if (!offer) {
        throw new Error(`variant ${item.variantId} vanished during checkout`);
      }
      return offer;
    };
    const first = offerOf(group[0]);
    const lines = group.map((item, position) => {
      const offer = offerOf(item);
      return {
        position,
        productId: offer.productId,
        variantId: item.variantId,
        title: `${offer.productTitle} — ${offer.variantTitle}`.slice(0, 200),
        quantity: item.quantity,
        unitPriceSeen: offer.price,
      };
    });
    const { fullName, phone, ...place } = request.delivery;
    const order = await tx.order.create({
      data: {
        id: uuidv7(),
        checkoutId,
        buyerId,
        shopId: first.shopId,
        shopName: first.shopName,
        sellerId: first.sellerId,
        currency: first.currency,
        paymentMethod: request.paymentMethod,
        status: 'pending',
        subtotalSeen: lines.reduce((sum, l) => sum + l.unitPriceSeen * BigInt(l.quantity), 0n),
        contactName: fullName,
        contactPhone: phone,
        ...place,
        lines: { create: lines },
      },
      include: { lines: true },
    });
    await publishCreated(tx, order);
    return order;
  }

  /**
   * Loads the cart, checks every line against the catalogue and fingerprints
   * what the buyer would pay: any change of item, quantity or price changes it.
   */
  private async checkedCart(db: Tx, buyerId: string): Promise<CheckedCart> {
    const items = await db.cartItem.findMany({
      where: { buyerId },
      orderBy: [{ createdAt: 'asc' }, { variantId: 'asc' }],
    });
    if (items.length === 0) {
      throw new ApiError(422, 'cart_empty', 'your cart is empty');
    }
    const known = await offers(
      db,
      items.map((item) => item.variantId),
    );
    const groups = new Map<string, CartItem[]>();
    const fingerprint: string[] = [];
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
      fingerprint.push(
        [item.variantId, item.quantity, offer.price, offer.currency, offer.shopId].join(':'),
      );
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
    const hash = createHash('sha256').update(fingerprint.sort().join('|')).digest('hex');
    return { items, known, groups, hash };
  }

  private async replay(tx: Tx, checkoutId: string): Promise<CheckoutView> {
    const orders = await tx.order.findMany({
      where: { checkoutId },
      include: { lines: true },
      orderBy: { id: 'asc' },
    });
    return { checkout_id: checkoutId, orders: orders.map(toOrderView), address_saved: false };
  }
}
