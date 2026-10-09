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
  Query,
} from '@nestjs/common';
import { Throttle } from '@nestjs/throttler';

import { CurrentUser, Public } from '../auth/auth.guard';
import type { AuthUser } from '../common/request';
import type { ImageUploadView } from '../media/media.service';
import { ConfirmImageDto, ImageUploadDto } from '../shops/shop.dto';
import {
  CreateProductDto,
  ListQueryDto,
  MyListQueryDto,
  SetStockDto,
  UpdateProductDto,
  type MyProductView,
  type Page,
  type ProductView,
} from './product.dto';
import { ProductsService } from './products.service';

const uuid = new ParseUUIDPipe();

/**
 * Seller routes under /shops/me come before the public /shops/:handle ones:
 * Express matches in registration order, and "me" is a reserved handle anyway.
 */
@Controller('api/v1')
export class ProductsController {
  constructor(private readonly products: ProductsService) {}

  @Post('shops/me/products')
  @HttpCode(201)
  create(@CurrentUser() user: AuthUser, @Body() dto: CreateProductDto): Promise<MyProductView> {
    return this.products.create(user.userId, dto);
  }

  @Get('shops/me/products')
  listMine(
    @CurrentUser() user: AuthUser,
    @Query() query: MyListQueryDto,
  ): Promise<Page<MyProductView>> {
    return this.products.listMine(user.userId, query);
  }

  @Get('shops/me/products/:productId')
  mine(
    @CurrentUser() user: AuthUser,
    @Param('productId', uuid) productId: string,
  ): Promise<MyProductView> {
    return this.products.mine(user.userId, productId.toLowerCase());
  }

  @Patch('shops/me/products/:productId')
  update(
    @CurrentUser() user: AuthUser,
    @Param('productId', uuid) productId: string,
    @Body() dto: UpdateProductDto,
  ): Promise<MyProductView> {
    return this.products.update(user.userId, productId.toLowerCase(), dto);
  }

  @Delete('shops/me/products/:productId')
  @HttpCode(204)
  remove(
    @CurrentUser() user: AuthUser,
    @Param('productId', uuid) productId: string,
  ): Promise<void> {
    return this.products.remove(user.userId, productId.toLowerCase());
  }

  @Put('shops/me/products/:productId/variants/:variantId/stock')
  setStock(
    @CurrentUser() user: AuthUser,
    @Param('productId', uuid) productId: string,
    @Param('variantId', uuid) variantId: string,
    @Body() dto: SetStockDto,
  ): Promise<MyProductView> {
    return this.products.setStock(
      user.userId,
      productId.toLowerCase(),
      variantId.toLowerCase(),
      dto.stock_on_hand,
    );
  }

  @Throttle({ default: { limit: 30, ttl: 60_000 } })
  @Post('shops/me/products/:productId/images/upload-url')
  @HttpCode(201)
  imageUpload(
    @CurrentUser() user: AuthUser,
    @Param('productId', uuid) productId: string,
    @Body() dto: ImageUploadDto,
  ): Promise<ImageUploadView> {
    return this.products.createImageUpload(user.userId, productId.toLowerCase(), dto.content_type);
  }

  @Throttle({ default: { limit: 30, ttl: 60_000 } })
  @Put('shops/me/products/:productId/images')
  addImage(
    @CurrentUser() user: AuthUser,
    @Param('productId', uuid) productId: string,
    @Body() dto: ConfirmImageDto,
  ): Promise<MyProductView> {
    return this.products.addImage(user.userId, productId.toLowerCase(), dto.upload_key);
  }

  @Delete('shops/me/products/:productId/images/:imageId')
  removeImage(
    @CurrentUser() user: AuthUser,
    @Param('productId', uuid) productId: string,
    @Param('imageId', uuid) imageId: string,
  ): Promise<MyProductView> {
    return this.products.removeImage(user.userId, productId.toLowerCase(), imageId.toLowerCase());
  }

  @Public()
  @Get('shops/:handle/products')
  publicByShop(
    @Param('handle') handle: string,
    @Query() query: ListQueryDto,
  ): Promise<Page<ProductView>> {
    return this.products.publicByShop(handle, query);
  }

  @Public()
  @Get('products/:productId')
  publicById(@Param('productId', uuid) productId: string): Promise<ProductView> {
    return this.products.publicById(productId.toLowerCase());
  }
}
