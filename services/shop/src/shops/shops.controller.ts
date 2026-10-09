import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  ParseUUIDPipe,
  Patch,
  Post,
  Put,
} from '@nestjs/common';
import { Throttle } from '@nestjs/throttler';

import { CurrentUser, Public } from '../auth/auth.guard';
import { ApiError } from '../common/api-error';
import type { AuthUser } from '../common/request';
import type { ImageUploadView } from '../media/media.service';
import {
  ConfirmImageDto,
  CreateShopDto,
  ImageUploadDto,
  UpdateShopDto,
  type MyShopView,
  type ShopView,
} from './shop.dto';
import { ShopsService } from './shops.service';

/** Staff roles allowed to suspend and reinstate shops. */
const MODERATION_ROLES = new Set(['ADMIN', 'MODERATOR']);

@Controller('api/v1/shops')
export class ShopsController {
  constructor(private readonly shops: ShopsService) {}

  @Throttle({ default: { limit: 10, ttl: 60_000 } })
  @Post()
  @HttpCode(201)
  create(@CurrentUser() user: AuthUser, @Body() dto: CreateShopDto): Promise<MyShopView> {
    return this.shops.create(user.userId, dto);
  }

  @Get('me')
  mine(@CurrentUser() user: AuthUser): Promise<MyShopView> {
    return this.shops.mine(user.userId);
  }

  @Patch('me')
  update(@CurrentUser() user: AuthUser, @Body() dto: UpdateShopDto): Promise<MyShopView> {
    return this.shops.update(user.userId, dto);
  }

  @Throttle({ default: { limit: 10, ttl: 60_000 } })
  @Post('me/logo/upload-url')
  @HttpCode(201)
  logoUpload(@CurrentUser() user: AuthUser, @Body() dto: ImageUploadDto): Promise<ImageUploadView> {
    return this.shops.createLogoUpload(user.userId, dto.content_type);
  }

  @Throttle({ default: { limit: 10, ttl: 60_000 } })
  @Put('me/logo')
  setLogo(@CurrentUser() user: AuthUser, @Body() dto: ConfirmImageDto): Promise<MyShopView> {
    return this.shops.setLogo(user.userId, dto.upload_key);
  }

  @Delete('me/logo')
  removeLogo(@CurrentUser() user: AuthUser): Promise<MyShopView> {
    return this.shops.removeLogo(user.userId);
  }

  @Post('me/close')
  @HttpCode(200)
  close(@CurrentUser() user: AuthUser): Promise<MyShopView> {
    return this.shops.close(user.userId);
  }

  @Post('me/reopen')
  @HttpCode(200)
  reopen(@CurrentUser() user: AuthUser): Promise<MyShopView> {
    return this.shops.reopen(user.userId);
  }

  @Public()
  @Get(':handle')
  byHandle(@Param('handle') handle: string): Promise<ShopView> {
    return this.shops.byHandle(handle);
  }
}

@Controller('api/v1/admin/shops')
export class AdminShopsController {
  constructor(private readonly shops: ShopsService) {}

  @Post(':shopId/suspend')
  @HttpCode(200)
  suspend(
    @CurrentUser() user: AuthUser,
    @Param('shopId', ParseUUIDPipe) shopId: string,
  ): Promise<MyShopView> {
    requireModerator(user);
    return this.shops.suspend(shopId.toLowerCase());
  }

  @Post(':shopId/reinstate')
  @HttpCode(200)
  reinstate(
    @CurrentUser() user: AuthUser,
    @Param('shopId', ParseUUIDPipe) shopId: string,
  ): Promise<MyShopView> {
    requireModerator(user);
    return this.shops.reinstate(shopId.toLowerCase());
  }
}

function requireModerator(user: AuthUser): void {
  if (!user.roles.some((role) => MODERATION_ROLES.has(role))) {
    throw new ApiError(403, 'forbidden', 'forbidden');
  }
}
