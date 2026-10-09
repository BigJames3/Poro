import {
  Body,
  Controller,
  Get,
  Headers,
  HttpCode,
  Param,
  ParseUUIDPipe,
  Post,
  Query,
} from '@nestjs/common';
import { Throttle } from '@nestjs/throttler';

import { CurrentUser } from '../auth/auth.guard';
import type { AuthUser } from '../common/request';
import { CheckoutService } from './checkout.service';
import {
  ConfirmDto,
  OrderListQueryDto,
  PreviewDto,
  ShipDto,
  type CheckoutView,
  type OrderView,
  type Page,
  type PrefillView,
  type PreviewView,
} from './order.dto';
import { OrdersService } from './orders.service';

const uuid = new ParseUUIDPipe();

@Controller('api/v1')
export class OrdersController {
  constructor(
    private readonly checkouts: CheckoutService,
    private readonly orders: OrdersService,
  ) {}

  /** The delivery contact to start from: a saved address, else the account. */
  @Get('checkout/prefill')
  prefill(
    @CurrentUser() user: AuthUser,
    @Headers('authorization') authorization: string,
  ): Promise<PrefillView> {
    return this.checkouts.prefill(user.userId, authorization);
  }

  /** The recap to review. Nothing is ordered until it is confirmed. */
  @Throttle({ default: { limit: 30, ttl: 60_000 } })
  @Post('checkout/preview')
  @HttpCode(201)
  preview(@CurrentUser() user: AuthUser, @Body() dto: PreviewDto): Promise<PreviewView> {
    return this.checkouts.preview(user.userId, dto);
  }

  /** Places the orders of a reviewed recap; confirming twice returns the same orders. */
  @Throttle({ default: { limit: 10, ttl: 60_000 } })
  @Post('checkout/confirm')
  @HttpCode(201)
  confirm(@CurrentUser() user: AuthUser, @Body() dto: ConfirmDto): Promise<CheckoutView> {
    return this.checkouts.confirm(user.userId, dto);
  }

  @Get('orders')
  list(@CurrentUser() user: AuthUser, @Query() query: OrderListQueryDto): Promise<Page<OrderView>> {
    return this.orders.list('buyer', user.userId, query);
  }

  @Get('orders/:orderId')
  get(@CurrentUser() user: AuthUser, @Param('orderId', uuid) orderId: string): Promise<OrderView> {
    return this.orders.get('buyer', user.userId, orderId.toLowerCase());
  }

  @Post('orders/:orderId/cancel')
  @HttpCode(200)
  cancel(
    @CurrentUser() user: AuthUser,
    @Param('orderId', uuid) orderId: string,
  ): Promise<OrderView> {
    return this.orders.cancel('buyer', user.userId, orderId.toLowerCase());
  }

  @Post('orders/:orderId/confirm-receipt')
  @HttpCode(200)
  confirmReceipt(
    @CurrentUser() user: AuthUser,
    @Param('orderId', uuid) orderId: string,
  ): Promise<OrderView> {
    return this.orders.confirmReceipt(user.userId, orderId.toLowerCase());
  }

  @Get('seller/orders')
  sellerList(
    @CurrentUser() user: AuthUser,
    @Query() query: OrderListQueryDto,
  ): Promise<Page<OrderView>> {
    return this.orders.list('seller', user.userId, query);
  }

  @Get('seller/orders/:orderId')
  sellerGet(
    @CurrentUser() user: AuthUser,
    @Param('orderId', uuid) orderId: string,
  ): Promise<OrderView> {
    return this.orders.get('seller', user.userId, orderId.toLowerCase());
  }

  @Post('seller/orders/:orderId/ship')
  @HttpCode(200)
  ship(
    @CurrentUser() user: AuthUser,
    @Param('orderId', uuid) orderId: string,
    @Body() dto: ShipDto,
  ): Promise<OrderView> {
    return this.orders.ship(user.userId, orderId.toLowerCase(), dto.tracking);
  }

  @Post('seller/orders/:orderId/cancel')
  @HttpCode(200)
  sellerCancel(
    @CurrentUser() user: AuthUser,
    @Param('orderId', uuid) orderId: string,
  ): Promise<OrderView> {
    return this.orders.cancel('seller', user.userId, orderId.toLowerCase());
  }
}
