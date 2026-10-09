import { Injectable } from '@nestjs/common';

import { ApiError } from '../common/api-error';
import { cleanLine, cleanText } from '../common/text';
import { uuidv7 } from '../common/uuid';
import {
  CURRENCY_OF,
  TYPE_SHOP_CREATED,
  type ShopCreatedV1,
  type ShopStatus,
} from '../events/catalog';
import { newEnvelope } from '../events/envelope';
import { enqueue } from '../events/outbox';
import { Prisma, Shop } from '../generated/prisma/client';
import { ImageContentType, ImageUploadView, MediaService } from '../media/media.service';
import { PrismaService } from '../prisma/prisma.service';
import { handleProblem, normalizeHandle } from './handle';
import { publishShop, sameShopFields } from './publish';
import {
  CreateShopDto,
  MyShopView,
  ShopView,
  UpdateShopDto,
  toMyShopView,
  toShopView,
} from './shop.dto';

export const SHOP_NAME_MAX = 60;
export const SHOP_DESCRIPTION_MAX = 500;
const SHOP_DESCRIPTION_MAX_LINES = 10;

function uniqueTarget(err: unknown): string | undefined {
  if (err instanceof Prisma.PrismaClientKnownRequestError && err.code === 'P2002') {
    return JSON.stringify(err.meta ?? {});
  }
  return undefined;
}

function cleanName(raw: string): string {
  return cleanLine(raw, SHOP_NAME_MAX, 'shop_name_invalid', 'shop name');
}

function cleanDescription(raw: string | null): string | null {
  return cleanText(
    raw,
    SHOP_DESCRIPTION_MAX,
    SHOP_DESCRIPTION_MAX_LINES,
    'shop_description_invalid',
    'shop description',
  );
}

@Injectable()
export class ShopsService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly media: MediaService,
  ) {}

  /**
   * Opens the shop of ownerId. The shop.created and shop.updated events are
   * written in the same transaction; auth then grants BUSINESS to the account.
   */
  async create(ownerId: string, dto: CreateShopDto): Promise<MyShopView> {
    const handle = this.checkedHandle(dto.handle);
    const name = cleanName(dto.name);
    const description = cleanDescription(dto.description ?? null);
    const currency = CURRENCY_OF[dto.country_code];
    if (await this.prisma.shop.findUnique({ where: { ownerId }, select: { id: true } })) {
      throw new ApiError(409, 'shop_exists', 'this account already has a shop');
    }
    try {
      const shop = await this.prisma.$transaction(async (tx) => {
        const created = await tx.shop.create({
          data: {
            id: uuidv7(),
            ownerId,
            name,
            handle,
            description,
            countryCode: dto.country_code,
            currency,
          },
        });
        const data: ShopCreatedV1 = {
          shop_id: created.id,
          owner_id: ownerId,
          name,
          handle,
          country_code: dto.country_code,
          currency,
          created_at: created.createdAt.toISOString(),
        };
        await enqueue(tx, newEnvelope(TYPE_SHOP_CREATED, 1, created.id, data, created.createdAt));
        await publishShop(tx, created, this.url);
        return created;
      });
      return toMyShopView(shop, this.url);
    } catch (err) {
      const target = uniqueTarget(err);
      if (target !== undefined && /owner_?id/i.test(target)) {
        throw new ApiError(409, 'shop_exists', 'this account already has a shop');
      }
      if (target !== undefined) {
        throw new ApiError(409, 'handle_taken', 'shop handle already taken');
      }
      throw err;
    }
  }

  async mine(ownerId: string): Promise<MyShopView> {
    return toMyShopView(await this.ownedShop(ownerId), this.url);
  }

  async update(ownerId: string, dto: UpdateShopDto): Promise<MyShopView> {
    const shop = await this.ownedShop(ownerId);
    const data: Prisma.ShopUpdateInput = {};
    if (dto.name !== undefined) {
      data.name = cleanName(dto.name);
    }
    if (dto.description !== undefined) {
      data.description = cleanDescription(dto.description);
    }
    return toMyShopView(await this.updatePublishing(shop.id, data), this.url);
  }

  async byHandle(raw: string): Promise<ShopView> {
    const handle = normalizeHandle(raw);
    const shop = handleProblem(handle)
      ? null
      : await this.prisma.shop.findFirst({ where: { handle, status: 'active' } });
    if (!shop) {
      throw new ApiError(404, 'shop_not_found', 'shop not found');
    }
    return toShopView(shop, this.url);
  }

  async createLogoUpload(ownerId: string, contentType: ImageContentType): Promise<ImageUploadView> {
    await this.ownedShop(ownerId);
    return this.media.createUpload(ownerId, contentType);
  }

  async setLogo(ownerId: string, uploadKey: string): Promise<MyShopView> {
    const before = await this.ownedShop(ownerId);
    const key = await this.media.publish(ownerId, uploadKey, 'logo', `shops/${before.id}`);
    let after: Shop;
    try {
      after = await this.updatePublishing(before.id, { logoKey: key });
    } catch (err) {
      await this.media.remove(key);
      throw err;
    }
    if (before.logoKey !== null) {
      await this.media.remove(before.logoKey);
    }
    return toMyShopView(after, this.url);
  }

  async removeLogo(ownerId: string): Promise<MyShopView> {
    const before = await this.ownedShop(ownerId);
    if (before.logoKey === null) {
      return toMyShopView(before, this.url);
    }
    const after = await this.updatePublishing(before.id, { logoKey: null });
    await this.media.remove(before.logoKey);
    return toMyShopView(after, this.url);
  }

  /** The owner closes an active shop; it disappears from the catalogue. */
  async close(ownerId: string): Promise<MyShopView> {
    const shop = await this.ownedShop(ownerId);
    return toMyShopView(await this.transition(shop.id, 'active', 'closed'), this.url);
  }

  /** The owner reopens a shop they closed. A suspended shop stays suspended. */
  async reopen(ownerId: string): Promise<MyShopView> {
    const shop = await this.ownedShop(ownerId);
    return toMyShopView(await this.transition(shop.id, 'closed', 'active'), this.url);
  }

  async suspend(shopId: string): Promise<MyShopView> {
    return toMyShopView(await this.transition(shopId, 'active', 'suspended'), this.url);
  }

  async reinstate(shopId: string): Promise<MyShopView> {
    return toMyShopView(await this.transition(shopId, 'suspended', 'active'), this.url);
  }

  /** Returns the shop of ownerId; every seller route goes through it. */
  async ownedShop(ownerId: string): Promise<Shop> {
    const shop = await this.prisma.shop.findUnique({ where: { ownerId } });
    if (!shop) {
      throw new ApiError(404, 'shop_not_found', 'open a shop first');
    }
    return shop;
  }

  private readonly url = (key: string): string => this.media.publicUrl(key);

  private async transition(shopId: string, from: ShopStatus, to: ShopStatus): Promise<Shop> {
    const shop = await this.prisma.shop.findUnique({ where: { id: shopId } });
    if (!shop) {
      throw new ApiError(404, 'shop_not_found', 'shop not found');
    }
    if (shop.status === to) {
      return shop;
    }
    if (shop.status !== from) {
      throw new ApiError(409, 'shop_status_conflict', `the shop is ${shop.status}`);
    }
    return this.updatePublishing(shopId, { status: to }, from);
  }

  /**
   * Applies data and, when a public field changed, enqueues poro.shop.shop.updated
   * in the same transaction. The row lock orders concurrent updates so each
   * snapshot is compared with the state it replaced.
   */
  private async updatePublishing(
    shopId: string,
    data: Prisma.ShopUpdateInput,
    requiredStatus?: ShopStatus,
  ): Promise<Shop> {
    return this.prisma.$transaction(async (tx) => {
      await tx.$queryRaw`SELECT 1 FROM shops WHERE id = ${shopId}::uuid FOR UPDATE`;
      const before = await tx.shop.findUniqueOrThrow({ where: { id: shopId } });
      if (requiredStatus !== undefined && before.status !== requiredStatus) {
        throw new ApiError(409, 'shop_status_conflict', `the shop is ${before.status}`);
      }
      const after = await tx.shop.update({ where: { id: shopId }, data });
      if (!sameShopFields(before, after)) {
        await publishShop(tx, after, this.url);
      }
      return after;
    });
  }

  private checkedHandle(raw: string): string {
    const handle = normalizeHandle(raw);
    const problem = handleProblem(handle);
    if (problem === 'invalid') {
      throw new ApiError(
        422,
        'handle_invalid',
        'shop handle must be 3 to 30 characters: letters, digits, "." or "_"',
      );
    }
    if (problem === 'reserved') {
      throw new ApiError(422, 'handle_reserved', 'this shop handle is reserved');
    }
    return handle;
  }
}
