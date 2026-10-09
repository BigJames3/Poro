import { Type } from 'class-transformer';
import {
  IsInt,
  IsNumber,
  IsOptional,
  IsString,
  Matches,
  Max,
  MaxLength,
  Min,
  ValidateNested,
} from 'class-validator';

import { ApiError } from '../common/api-error';
import { cleanLine, cleanText } from '../common/text';

export const PHONE_PATTERN = /^\+[1-9]\d{7,14}$/;

/** GPS position shared by the buyer's device, only when they allow it. */
export class LocationDto {
  @IsNumber({ allowNaN: false, allowInfinity: false })
  @Min(-90)
  @Max(90)
  latitude!: number;

  @IsNumber({ allowNaN: false, allowInfinity: false })
  @Min(-180)
  @Max(180)
  longitude!: number;

  /** Accuracy radius reported by the device, in metres. */
  @IsOptional()
  @IsInt()
  @Min(0)
  @Max(100_000)
  accuracy_m?: number | null;
}

/** Name, phone and city are required, plus a GPS position or an address. */
export class DeliveryInputDto {
  @IsString()
  @MaxLength(200)
  full_name!: string;

  @IsString()
  @Matches(PHONE_PATTERN, {
    message: 'phone must be in international format, e.g. +2250700000000',
  })
  phone!: string;

  @IsString()
  @MaxLength(200)
  city!: string;

  @IsOptional()
  @IsString()
  @MaxLength(1000)
  address?: string | null;

  /** Directions to find the place: landmark, gate colour… */
  @IsOptional()
  @IsString()
  @MaxLength(1000)
  landmark?: string | null;

  @IsOptional()
  @ValidateNested()
  @Type(() => LocationDto)
  location?: LocationDto | null;
}

/** A validated delivery, in the column names shared by orders and addresses. */
export interface Delivery {
  fullName: string;
  phone: string;
  city: string;
  address: string | null;
  landmark: string | null;
  latitude: number | null;
  longitude: number | null;
  locationAccuracyM: number | null;
}

export interface LocationView {
  latitude: number;
  longitude: number;
  accuracy_m: number | null;
}

export interface DeliveryView {
  full_name: string;
  phone: string;
  city: string;
  address: string | null;
  landmark: string | null;
  location: LocationView | null;
}

export function cleanDelivery(input: DeliveryInputDto): Delivery {
  const location = input.location ?? null;
  const delivery: Delivery = {
    fullName: cleanLine(input.full_name, 80, 'delivery_invalid', 'full name'),
    phone: input.phone,
    city: cleanLine(input.city, 80, 'delivery_invalid', 'city'),
    address: cleanText(input.address ?? null, 300, 3, 'delivery_invalid', 'address'),
    landmark: cleanText(input.landmark ?? null, 300, 3, 'delivery_invalid', 'directions'),
    latitude: location?.latitude ?? null,
    longitude: location?.longitude ?? null,
    locationAccuracyM: location?.accuracy_m ?? null,
  };
  if (delivery.address === null && delivery.latitude === null) {
    throw new ApiError(
      422,
      'delivery_location_required',
      'share your position or enter an address',
    );
  }
  return delivery;
}

/** Columns of a delivery as stored; null name, phone or city means it was erased. */
export interface StoredDelivery {
  fullName?: string | null;
  contactName?: string | null;
  phone?: string | null;
  contactPhone?: string | null;
  city: string | null;
  address: string | null;
  landmark: string | null;
  latitude: number | null;
  longitude: number | null;
  locationAccuracyM: number | null;
}

export function toDeliveryView(row: StoredDelivery): DeliveryView | null {
  const fullName = row.fullName ?? row.contactName ?? null;
  const phone = row.phone ?? row.contactPhone ?? null;
  if (fullName === null || phone === null || row.city === null) {
    return null;
  }
  return {
    full_name: fullName,
    phone,
    city: row.city,
    address: row.address,
    landmark: row.landmark,
    location:
      row.latitude === null || row.longitude === null
        ? null
        : { latitude: row.latitude, longitude: row.longitude, accuracy_m: row.locationAccuracyM },
  };
}

export const PAYMENT_METHOD_LABELS: Record<string, string> = {
  cash_on_delivery: 'Paiement à la livraison',
  wave: 'Wave',
  simulated: 'Paiement simulé',
};

export const DELIVERY_FEE_NOTICE =
  'Frais de livraison à régler au livreur, non inclus dans le total.';
