import { IsIn, IsOptional, IsString, MaxLength } from 'class-validator';

import { COUNTRIES, type CountryCode, type Currency, type ShopStatus } from '../events/catalog';
import { Shop } from '../generated/prisma/client';
import { IMAGE_CONTENT_TYPES, type ImageContentType } from '../media/media.service';

export class CreateShopDto {
  @IsString()
  @MaxLength(200)
  name!: string;

  @IsString()
  @MaxLength(64)
  handle!: string;

  @IsIn(COUNTRIES)
  country_code!: CountryCode;

  @IsOptional()
  @IsString()
  @MaxLength(2000)
  description?: string | null;
}

/** Undefined keeps a field; null clears the description. Handle and country never change. */
export class UpdateShopDto {
  @IsOptional()
  @IsString()
  @MaxLength(200)
  name?: string;

  @IsOptional()
  @IsString()
  @MaxLength(2000)
  description?: string | null;
}

export class ImageUploadDto {
  @IsIn(IMAGE_CONTENT_TYPES)
  content_type!: ImageContentType;
}

export class ConfirmImageDto {
  @IsString()
  @MaxLength(200)
  upload_key!: string;
}

export interface ShopView {
  shop_id: string;
  handle: string;
  name: string;
  description: string | null;
  logo_url: string | null;
  country_code: CountryCode;
  currency: Currency;
  created_at: string;
}

export interface MyShopView extends ShopView {
  owner_id: string;
  status: ShopStatus;
  updated_at: string;
}

export function toShopView(shop: Shop, url: (key: string) => string): ShopView {
  return {
    shop_id: shop.id,
    handle: shop.handle,
    name: shop.name,
    description: shop.description,
    logo_url: shop.logoKey === null ? null : url(shop.logoKey),
    country_code: shop.countryCode as CountryCode,
    currency: shop.currency as Currency,
    created_at: shop.createdAt.toISOString(),
  };
}

export function toMyShopView(shop: Shop, url: (key: string) => string): MyShopView {
  return {
    ...toShopView(shop, url),
    owner_id: shop.ownerId,
    status: shop.status as ShopStatus,
    updated_at: shop.updatedAt.toISOString(),
  };
}
