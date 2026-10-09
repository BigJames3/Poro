import { IsBoolean, IsOptional, IsString, MaxLength } from 'class-validator';

import { DeliveryInputDto, DeliveryView } from '../delivery/delivery';

export const MAX_ADDRESSES = 10;

export class AddressInputDto extends DeliveryInputDto {
  /** For instance "Maison" or "Bureau". */
  @IsOptional()
  @IsString()
  @MaxLength(100)
  label?: string | null;

  @IsOptional()
  @IsBoolean()
  is_default?: boolean;
}

export interface AddressView extends DeliveryView {
  address_id: string;
  label: string | null;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}
