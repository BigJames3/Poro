import { Body, Controller, Delete, Get, Param, ParseUUIDPipe, Put } from '@nestjs/common';

import { CurrentUser } from '../auth/auth.guard';
import type { AuthUser } from '../common/request';
import { SetQuantityDto, type CartView } from './cart.dto';
import { CartService } from './cart.service';

@Controller('api/v1/cart')
export class CartController {
  constructor(private readonly cart: CartService) {}

  @Get()
  view(@CurrentUser() user: AuthUser): Promise<CartView> {
    return this.cart.view(user.userId);
  }

  @Put('items/:variantId')
  setItem(
    @CurrentUser() user: AuthUser,
    @Param('variantId', ParseUUIDPipe) variantId: string,
    @Body() dto: SetQuantityDto,
  ): Promise<CartView> {
    return this.cart.setItem(user.userId, variantId.toLowerCase(), dto.quantity);
  }

  @Delete('items/:variantId')
  removeItem(
    @CurrentUser() user: AuthUser,
    @Param('variantId', ParseUUIDPipe) variantId: string,
  ): Promise<CartView> {
    return this.cart.removeItem(user.userId, variantId.toLowerCase());
  }

  @Delete()
  clear(@CurrentUser() user: AuthUser): Promise<CartView> {
    return this.cart.clear(user.userId);
  }
}
