import { Injectable } from '@nestjs/common';

import { ApiError } from '../common/api-error';
import { decodeCursor, encodeCursor, pageSize } from '../common/cursor';
import { cleanLine, cleanText } from '../common/text';
import { uuidv7 } from '../common/uuid';
import {
  TYPE_PRODUCT_DELETED,
  type Currency,
  type ProductDeletedV1,
  type ProductStatus,
} from '../events/catalog';
import { newEnvelope } from '../events/envelope';
import { enqueue } from '../events/outbox';
import { Prisma, Shop } from '../generated/prisma/client';
import { ImageContentType, ImageUploadView, MediaService } from '../media/media.service';
import { PrismaService } from '../prisma/prisma.service';
import { handleProblem, normalizeHandle } from '../shops/handle';
import { ShopsService } from '../shops/shops.service';
import {
  CreateProductDto,
  ListQueryDto,
  MAX_IMAGES,
  MyListQueryDto,
  MyProductView,
  Page,
  ProductView,
  UpdateProductDto,
  VariantInputDto,
} from './product.dto';
import { FullProduct, available, productInclude, publishProduct } from './publish';

export const PRODUCT_TITLE_MAX = 120;
export const PRODUCT_DESCRIPTION_MAX = 2000;
const PRODUCT_DESCRIPTION_MAX_LINES = 50;
export const VARIANT_TITLE_MAX = 60;

type Tx = Prisma.TransactionClient;

function cleanTitle(raw: string): string {
  return cleanLine(raw, PRODUCT_TITLE_MAX, 'product_title_invalid', 'product title');
}

function cleanDescription(raw: string | null): string | null {
  return cleanText(
    raw,
    PRODUCT_DESCRIPTION_MAX,
    PRODUCT_DESCRIPTION_MAX_LINES,
    'product_description_invalid',
    'product description',
  );
}

function cleanVariantTitle(raw: string): string {
  return cleanLine(raw, VARIANT_TITLE_MAX, 'variant_title_invalid', 'variant title');
}

/** Locks one product row. Every writer locks products before variants, in id order. */
async function lockProduct(tx: Tx, productId: string): Promise<void> {
  await tx.$queryRaw`SELECT 1 FROM products WHERE id = ${productId}::uuid FOR UPDATE`;
}

async function lockVariantsOf(tx: Tx, productId: string): Promise<void> {
  await tx.$queryRaw`
    SELECT 1 FROM variants WHERE product_id = ${productId}::uuid ORDER BY id FOR UPDATE`;
}

@Injectable()
export class ProductsService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly media: MediaService,
    private readonly shops: ShopsService,
  ) {}

  async create(ownerId: string, dto: CreateProductDto): Promise<MyProductView> {
    const shop = await this.writableShop(ownerId);
    const title = cleanTitle(dto.title);
    const description = cleanDescription(dto.description ?? null);
    const variants = dto.variants.map((input, position) => {
      if (input.variant_id !== undefined) {
        throw new ApiError(422, 'variant_not_found', 'a new product has no variant ids yet');
      }
      return {
        id: uuidv7(),
        title: cleanVariantTitle(input.title),
        price: BigInt(input.price),
        stockOnHand: input.stock ?? 0,
        position,
      };
    });
    const product = await this.prisma.$transaction(async (tx) => {
      const created = await tx.product.create({
        data: { id: uuidv7(), shopId: shop.id, title, description, status: 'draft' },
      });
      await tx.variant.createMany({
        data: variants.map((variant) => ({ ...variant, productId: created.id })),
      });
      return publishProduct(tx, created.id, this.url);
    });
    return this.myView(product);
  }

  async mine(ownerId: string, productId: string): Promise<MyProductView> {
    const shop = await this.shops.ownedShop(ownerId);
    return this.myView(await this.ownedProduct(this.prisma, shop, productId));
  }

  async listMine(ownerId: string, query: MyListQueryDto): Promise<Page<MyProductView>> {
    const shop = await this.shops.ownedShop(ownerId);
    const where: Prisma.ProductWhereInput = { shopId: shop.id, deletedAt: null };
    if (query.status !== undefined) {
      where.status = query.status;
    }
    return this.page(where, query, (product) => this.myView(product));
  }

  async update(ownerId: string, productId: string, dto: UpdateProductDto): Promise<MyProductView> {
    const shop = await this.writableShop(ownerId);
    const data: Prisma.ProductUpdateInput = {};
    if (dto.title !== undefined) {
      data.title = cleanTitle(dto.title);
    }
    if (dto.description !== undefined) {
      data.description = cleanDescription(dto.description);
    }
    if (dto.status !== undefined) {
      data.status = dto.status;
    }
    const product = await this.prisma.$transaction(async (tx) => {
      await lockProduct(tx, productId);
      const current = await this.ownedProduct(tx, shop, productId);
      if (dto.variants !== undefined) {
        await lockVariantsOf(tx, productId);
        await this.syncVariants(tx, current, dto.variants);
      }
      if (dto.status === 'active' && current.images.length === 0) {
        throw new ApiError(422, 'image_required', 'add a photo before publishing the product');
      }
      await tx.product.update({ where: { id: productId }, data });
      return publishProduct(tx, productId, this.url);
    });
    return this.myView(product);
  }

  /** Soft-deletes the product. Refused while an order holds some of its stock. */
  async remove(ownerId: string, productId: string): Promise<void> {
    const shop = await this.writableShop(ownerId);
    const keys = await this.prisma.$transaction(async (tx) => {
      await lockProduct(tx, productId);
      const product = await this.ownedProduct(tx, shop, productId);
      await lockVariantsOf(tx, productId);
      if (product.variants.some((variant) => variant.stockReserved > 0)) {
        throw new ApiError(
          409,
          'product_has_reservations',
          'orders still hold stock of this product',
        );
      }
      const now = new Date();
      await tx.variant.updateMany({ where: { productId }, data: { deletedAt: now } });
      await tx.product.update({ where: { id: productId }, data: { deletedAt: now } });
      const data: ProductDeletedV1 = {
        product_id: productId,
        shop_id: shop.id,
        owner_id: shop.ownerId,
        deleted_at: now.toISOString(),
      };
      await enqueue(tx, newEnvelope(TYPE_PRODUCT_DELETED, 1, productId, data, now));
      return product.images.map((image) => image.key);
    });
    await Promise.all(keys.map((key) => this.media.remove(key)));
  }

  /** Sets the stock on hand of a variant; it never drops below what orders hold. */
  async setStock(
    ownerId: string,
    productId: string,
    variantId: string,
    stockOnHand: number,
  ): Promise<MyProductView> {
    const shop = await this.writableShop(ownerId);
    const product = await this.prisma.$transaction(async (tx) => {
      await lockProduct(tx, productId);
      const current = await this.ownedProduct(tx, shop, productId);
      if (!current.variants.some((variant) => variant.id === variantId)) {
        throw new ApiError(404, 'variant_not_found', 'variant not found');
      }
      await tx.$queryRaw`SELECT 1 FROM variants WHERE id = ${variantId}::uuid FOR UPDATE`;
      const variant = await tx.variant.findUniqueOrThrow({ where: { id: variantId } });
      if (stockOnHand < variant.stockReserved) {
        throw new ApiError(
          409,
          'stock_below_reserved',
          `orders hold ${variant.stockReserved} units of this variant`,
        );
      }
      await tx.variant.update({ where: { id: variantId }, data: { stockOnHand } });
      const wasInStock = available(variant) > 0;
      const isInStock = stockOnHand - variant.stockReserved > 0;
      return wasInStock === isInStock
        ? this.ownedProduct(tx, shop, productId)
        : publishProduct(tx, productId, this.url);
    });
    return this.myView(product);
  }

  async createImageUpload(
    ownerId: string,
    productId: string,
    contentType: ImageContentType,
  ): Promise<ImageUploadView> {
    const shop = await this.writableShop(ownerId);
    const product = await this.ownedProduct(this.prisma, shop, productId);
    if (product.images.length >= MAX_IMAGES) {
      throw new ApiError(422, 'too_many_images', `a product has at most ${MAX_IMAGES} photos`);
    }
    return this.media.createUpload(ownerId, contentType);
  }

  async addImage(ownerId: string, productId: string, uploadKey: string): Promise<MyProductView> {
    const shop = await this.writableShop(ownerId);
    await this.ownedProduct(this.prisma, shop, productId);
    const key = await this.media.publish(
      ownerId,
      uploadKey,
      'product',
      `products/${shop.id}/${productId}`,
    );
    try {
      const product = await this.prisma.$transaction(async (tx) => {
        await lockProduct(tx, productId);
        const current = await this.ownedProduct(tx, shop, productId);
        if (current.images.length >= MAX_IMAGES) {
          throw new ApiError(422, 'too_many_images', `a product has at most ${MAX_IMAGES} photos`);
        }
        const last = current.images.at(-1);
        await tx.productImage.create({
          data: { id: uuidv7(), productId, key, position: (last?.position ?? -1) + 1 },
        });
        return publishProduct(tx, productId, this.url);
      });
      return this.myView(product);
    } catch (err) {
      await this.media.remove(key);
      throw err;
    }
  }

  async removeImage(ownerId: string, productId: string, imageId: string): Promise<MyProductView> {
    const shop = await this.writableShop(ownerId);
    const { product, key } = await this.prisma.$transaction(async (tx) => {
      await lockProduct(tx, productId);
      const current = await this.ownedProduct(tx, shop, productId);
      const image = current.images.find((candidate) => candidate.id === imageId);
      if (!image) {
        throw new ApiError(404, 'image_not_found', 'photo not found');
      }
      if (current.status === 'active' && current.images.length === 1) {
        throw new ApiError(422, 'image_required', 'a published product keeps at least one photo');
      }
      await tx.productImage.delete({ where: { id: imageId } });
      return { product: await publishProduct(tx, productId, this.url), key: image.key };
    });
    await this.media.remove(key);
    return this.myView(product);
  }

  /** A published product of an active shop; anything else is not found. */
  async publicById(productId: string): Promise<ProductView> {
    const product = await this.prisma.product.findFirst({
      where: { id: productId, deletedAt: null, status: 'active', shop: { status: 'active' } },
      include: productInclude,
    });
    if (!product) {
      throw new ApiError(404, 'product_not_found', 'product not found');
    }
    return this.publicView(product);
  }

  async publicByShop(rawHandle: string, query: ListQueryDto): Promise<Page<ProductView>> {
    const handle = normalizeHandle(rawHandle);
    const shop = handleProblem(handle)
      ? null
      : await this.prisma.shop.findFirst({ where: { handle, status: 'active' } });
    if (!shop) {
      throw new ApiError(404, 'shop_not_found', 'shop not found');
    }
    return this.page({ shopId: shop.id, deletedAt: null, status: 'active' }, query, (product) =>
      this.publicView(product),
    );
  }

  private readonly url = (key: string): string => this.media.publicUrl(key);

  /** The seller's shop, unless a moderator suspended it. */
  private async writableShop(ownerId: string): Promise<Shop> {
    const shop = await this.shops.ownedShop(ownerId);
    if (shop.status === 'suspended') {
      throw new ApiError(403, 'shop_suspended', 'this shop is suspended');
    }
    return shop;
  }

  private async ownedProduct(db: Tx, shop: Shop, productId: string): Promise<FullProduct> {
    const product = await db.product.findFirst({
      where: { id: productId, shopId: shop.id, deletedAt: null },
      include: productInclude,
    });
    if (!product) {
      throw new ApiError(404, 'product_not_found', 'product not found');
    }
    return product;
  }

  /** Applies the full variant list: update listed ids, create new ones, remove the rest. */
  private async syncVariants(
    tx: Tx,
    product: FullProduct,
    inputs: VariantInputDto[],
  ): Promise<void> {
    const existing = new Map(product.variants.map((variant) => [variant.id, variant]));
    const kept = new Set<string>();
    for (const input of inputs) {
      if (input.variant_id === undefined) {
        continue;
      }
      const id = input.variant_id.toLowerCase();
      if (!existing.has(id) || kept.has(id)) {
        throw new ApiError(422, 'variant_not_found', `unknown variant ${input.variant_id}`);
      }
      if (input.stock !== undefined) {
        throw new ApiError(
          422,
          'stock_not_editable_here',
          'change the stock of an existing variant through its stock route',
        );
      }
      kept.add(id);
    }
    const removed = product.variants.filter((variant) => !kept.has(variant.id));
    if (removed.some((variant) => variant.stockReserved > 0)) {
      throw new ApiError(
        409,
        'variant_has_reservations',
        'orders still hold stock of a removed variant',
      );
    }
    if (removed.length > 0) {
      await tx.variant.updateMany({
        where: { id: { in: removed.map((variant) => variant.id) } },
        data: { deletedAt: new Date() },
      });
    }
    for (const [position, input] of inputs.entries()) {
      const title = cleanVariantTitle(input.title);
      const price = BigInt(input.price);
      if (input.variant_id === undefined) {
        await tx.variant.create({
          data: {
            id: uuidv7(),
            productId: product.id,
            title,
            price,
            stockOnHand: input.stock ?? 0,
            position,
          },
        });
      } else {
        await tx.variant.update({
          where: { id: input.variant_id.toLowerCase() },
          data: { title, price, position },
        });
      }
    }
  }

  private async page<T>(
    where: Prisma.ProductWhereInput,
    query: ListQueryDto,
    view: (product: FullProduct) => T,
  ): Promise<Page<T>> {
    const limit = pageSize(query.limit);
    const after = decodeCursor(query.cursor);
    const products = await this.prisma.product.findMany({
      where: after
        ? {
            AND: [
              where,
              {
                OR: [
                  { createdAt: { lt: after.createdAt } },
                  { createdAt: after.createdAt, id: { lt: after.id } },
                ],
              },
            ],
          }
        : where,
      orderBy: [{ createdAt: 'desc' }, { id: 'desc' }],
      take: limit + 1,
      include: productInclude,
    });
    const items = products.slice(0, limit);
    const last = items.at(-1);
    return {
      items: items.map(view),
      next_cursor:
        products.length > limit && last
          ? encodeCursor({ createdAt: last.createdAt, id: last.id })
          : null,
    };
  }

  private publicView(product: FullProduct): ProductView {
    return {
      product_id: product.id,
      shop: { shop_id: product.shop.id, handle: product.shop.handle, name: product.shop.name },
      title: product.title,
      description: product.description,
      currency: product.shop.currency as Currency,
      images: product.images.map((image) => ({ image_id: image.id, url: this.url(image.key) })),
      variants: product.variants.map((variant) => ({
        variant_id: variant.id,
        title: variant.title,
        price: Number(variant.price),
        in_stock: available(variant) > 0,
      })),
      created_at: product.createdAt.toISOString(),
      updated_at: product.updatedAt.toISOString(),
    };
  }

  private myView(product: FullProduct): MyProductView {
    return {
      ...this.publicView(product),
      status: product.status as ProductStatus,
      variants: product.variants.map((variant) => ({
        variant_id: variant.id,
        title: variant.title,
        price: Number(variant.price),
        in_stock: available(variant) > 0,
        stock_on_hand: variant.stockOnHand,
        stock_reserved: variant.stockReserved,
        stock_available: available(variant),
      })),
    };
  }
}
