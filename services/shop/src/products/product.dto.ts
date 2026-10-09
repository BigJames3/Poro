import { Type } from 'class-transformer';
import {
  ArrayMaxSize,
  ArrayMinSize,
  IsIn,
  IsInt,
  IsOptional,
  IsString,
  IsUUID,
  Max,
  MaxLength,
  Min,
  ValidateNested,
} from 'class-validator';

import type { Currency, ProductStatus } from '../events/catalog';

export const PRODUCT_STATUSES = ['draft', 'active', 'archived'] as const;
export const MAX_VARIANTS = 50;
export const MAX_IMAGES = 10;
export const MAX_PRICE = 1_000_000_000_000;
export const MAX_STOCK = 1_000_000;

export class VariantInputDto {
  /** Omitted for a new variant. */
  @IsOptional()
  @IsUUID()
  variant_id?: string;

  @IsString()
  @MaxLength(200)
  title!: string;

  /** Minor units of the shop currency: XOF and XAF have none, NGN is in kobo. */
  @IsInt()
  @Min(1)
  @Max(MAX_PRICE)
  price!: number;

  /** Initial stock of a new variant; an existing one changes through the stock route. */
  @IsOptional()
  @IsInt()
  @Min(0)
  @Max(MAX_STOCK)
  stock?: number;
}

export class CreateProductDto {
  @IsString()
  @MaxLength(400)
  title!: string;

  @IsOptional()
  @IsString()
  @MaxLength(6000)
  description?: string | null;

  @ArrayMinSize(1)
  @ArrayMaxSize(MAX_VARIANTS)
  @ValidateNested({ each: true })
  @Type(() => VariantInputDto)
  variants!: VariantInputDto[];
}

/**
 * Undefined keeps a field. variants, when present, is the full list: listed
 * ids are updated, entries without id are created and missing ones removed.
 */
export class UpdateProductDto {
  @IsOptional()
  @IsString()
  @MaxLength(400)
  title?: string;

  @IsOptional()
  @IsString()
  @MaxLength(6000)
  description?: string | null;

  @IsOptional()
  @IsIn(PRODUCT_STATUSES)
  status?: ProductStatus;

  @IsOptional()
  @ArrayMinSize(1)
  @ArrayMaxSize(MAX_VARIANTS)
  @ValidateNested({ each: true })
  @Type(() => VariantInputDto)
  variants?: VariantInputDto[];
}

export class SetStockDto {
  @IsInt()
  @Min(0)
  @Max(MAX_STOCK)
  stock_on_hand!: number;
}

export class ListQueryDto {
  @IsOptional()
  @IsString()
  @MaxLength(200)
  cursor?: string;

  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(1)
  @Max(50)
  limit?: number;
}

export class MyListQueryDto extends ListQueryDto {
  @IsOptional()
  @IsIn(PRODUCT_STATUSES)
  status?: ProductStatus;
}

export interface VariantView {
  variant_id: string;
  title: string;
  price: number;
  in_stock: boolean;
}

export interface MyVariantView extends VariantView {
  stock_on_hand: number;
  stock_reserved: number;
  stock_available: number;
}

export interface ImageView {
  image_id: string;
  url: string;
}

export interface ProductView {
  product_id: string;
  shop: { shop_id: string; handle: string; name: string };
  title: string;
  description: string | null;
  currency: Currency;
  images: ImageView[];
  variants: VariantView[];
  created_at: string;
  updated_at: string;
}

export interface MyProductView extends Omit<ProductView, 'variants'> {
  status: ProductStatus;
  variants: MyVariantView[];
}

export interface Page<T> {
  items: T[];
  next_cursor: string | null;
}
