import { Injectable } from '@nestjs/common';

import { ApiError } from '../common/api-error';
import { cleanText } from '../common/text';
import { uuidv7 } from '../common/uuid';
import { Delivery, cleanDelivery, toDeliveryView } from '../delivery/delivery';
import { Address, Prisma } from '../generated/prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import { AddressInputDto, AddressView, MAX_ADDRESSES } from './address.dto';

type Tx = Prisma.TransactionClient;

export function toAddressView(address: Address): AddressView {
  const delivery = toDeliveryView(address);
  if (!delivery) {
    throw new Error(`address ${address.id} has no contact`);
  }
  return {
    address_id: address.id,
    label: address.label,
    ...delivery,
    is_default: address.isDefault,
    created_at: address.createdAt.toISOString(),
    updated_at: address.updatedAt.toISOString(),
  };
}

function cleanLabel(raw: string | null | undefined): string | null {
  return cleanText(raw ?? null, 40, 1, 'address_label_invalid', 'label');
}

/** Delivery addresses of an account; the first one becomes the default. */
@Injectable()
export class AddressesService {
  constructor(private readonly prisma: PrismaService) {}

  async list(buyerId: string): Promise<AddressView[]> {
    const addresses = await this.prisma.address.findMany({
      where: { buyerId },
      orderBy: [{ isDefault: 'desc' }, { createdAt: 'desc' }],
    });
    return addresses.map(toAddressView);
  }

  async create(buyerId: string, dto: AddressInputDto): Promise<AddressView> {
    const delivery = cleanDelivery(dto);
    const label = cleanLabel(dto.label);
    const address = await this.prisma.$transaction(async (tx) => {
      const saved = await saveAddress(tx, buyerId, delivery, label, dto.is_default === true);
      if (!saved) {
        throw new ApiError(
          422,
          'address_book_full',
          `an account keeps at most ${MAX_ADDRESSES} addresses`,
        );
      }
      return saved;
    });
    return toAddressView(address);
  }

  async update(buyerId: string, addressId: string, dto: AddressInputDto): Promise<AddressView> {
    const delivery = cleanDelivery(dto);
    const label = cleanLabel(dto.label);
    const address = await this.prisma.$transaction(async (tx) => {
      await this.owned(tx, buyerId, addressId);
      if (dto.is_default === true) {
        await tx.address.updateMany({ where: { buyerId }, data: { isDefault: false } });
      }
      return tx.address.update({
        where: { id: addressId },
        data: { ...delivery, label, ...(dto.is_default === true ? { isDefault: true } : {}) },
      });
    });
    return toAddressView(address);
  }

  async makeDefault(buyerId: string, addressId: string): Promise<AddressView> {
    const address = await this.prisma.$transaction(async (tx) => {
      await this.owned(tx, buyerId, addressId);
      await tx.address.updateMany({ where: { buyerId }, data: { isDefault: false } });
      return tx.address.update({ where: { id: addressId }, data: { isDefault: true } });
    });
    return toAddressView(address);
  }

  /** Deleting the default promotes the most recent remaining address. */
  async remove(buyerId: string, addressId: string): Promise<void> {
    await this.prisma.$transaction(async (tx) => {
      const address = await this.owned(tx, buyerId, addressId);
      await tx.address.delete({ where: { id: addressId } });
      if (address.isDefault) {
        const next = await tx.address.findFirst({
          where: { buyerId },
          orderBy: { createdAt: 'desc' },
        });
        if (next) {
          await tx.address.update({ where: { id: next.id }, data: { isDefault: true } });
        }
      }
    });
  }

  /** The default address, else the most recent one. */
  async preferred(buyerId: string): Promise<Address | null> {
    return this.prisma.address.findFirst({
      where: { buyerId },
      orderBy: [{ isDefault: 'desc' }, { createdAt: 'desc' }],
    });
  }

  private async owned(tx: Tx, buyerId: string, addressId: string): Promise<Address> {
    const address = await tx.address.findFirst({ where: { id: addressId, buyerId } });
    if (!address) {
      throw new ApiError(404, 'address_not_found', 'address not found');
    }
    return address;
  }
}

/**
 * Adds an address to the book of buyerId, or returns null when the book is
 * full. Serialised per buyer so the limit and the single default hold.
 */
export async function saveAddress(
  tx: Tx,
  buyerId: string,
  delivery: Delivery,
  label: string | null,
  makeDefault: boolean,
): Promise<Address | null> {
  await tx.$executeRaw`SELECT pg_advisory_xact_lock(hashtext(${`addresses:${buyerId}`}))`;
  const count = await tx.address.count({ where: { buyerId } });
  if (count >= MAX_ADDRESSES) {
    return null;
  }
  const isDefault = makeDefault || count === 0;
  if (isDefault) {
    await tx.address.updateMany({ where: { buyerId }, data: { isDefault: false } });
  }
  return tx.address.create({
    data: { id: uuidv7(), buyerId, label, ...delivery, isDefault },
  });
}
