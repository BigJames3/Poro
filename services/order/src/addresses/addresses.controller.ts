import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  ParseUUIDPipe,
  Post,
  Put,
} from '@nestjs/common';

import { CurrentUser } from '../auth/auth.guard';
import type { AuthUser } from '../common/request';
import { AddressInputDto, type AddressView } from './address.dto';
import { AddressesService } from './addresses.service';

const uuid = new ParseUUIDPipe();

@Controller('api/v1/addresses')
export class AddressesController {
  constructor(private readonly addresses: AddressesService) {}

  @Get()
  list(@CurrentUser() user: AuthUser): Promise<AddressView[]> {
    return this.addresses.list(user.userId);
  }

  @Post()
  @HttpCode(201)
  create(@CurrentUser() user: AuthUser, @Body() dto: AddressInputDto): Promise<AddressView> {
    return this.addresses.create(user.userId, dto);
  }

  @Put(':addressId')
  update(
    @CurrentUser() user: AuthUser,
    @Param('addressId', uuid) addressId: string,
    @Body() dto: AddressInputDto,
  ): Promise<AddressView> {
    return this.addresses.update(user.userId, addressId.toLowerCase(), dto);
  }

  @Post(':addressId/default')
  @HttpCode(200)
  makeDefault(
    @CurrentUser() user: AuthUser,
    @Param('addressId', uuid) addressId: string,
  ): Promise<AddressView> {
    return this.addresses.makeDefault(user.userId, addressId.toLowerCase());
  }

  @Delete(':addressId')
  @HttpCode(204)
  remove(
    @CurrentUser() user: AuthUser,
    @Param('addressId', uuid) addressId: string,
  ): Promise<void> {
    return this.addresses.remove(user.userId, addressId.toLowerCase());
  }
}
