import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  Patch,
  Post,
  Put,
  Query,
} from '@nestjs/common';
import { Throttle } from '@nestjs/throttler';

import { CurrentUser } from '../auth/auth.guard';
import type { AuthUser } from '../common/request';
import {
  ListNotificationsDto,
  RegisterDeviceDto,
  UpdatePreferencesDto,
  type DeviceView,
  type NotificationPage,
  type PreferencesView,
} from './notification.dto';
import { NotificationsService } from './notifications.service';

@Controller('api/v1/notifications')
export class NotificationsController {
  constructor(private readonly notifications: NotificationsService) {}

  @Get()
  list(
    @CurrentUser() user: AuthUser,
    @Query() query: ListNotificationsDto,
  ): Promise<NotificationPage> {
    return this.notifications.list(user.userId, query.cursor, query.limit);
  }

  @Get('unread-count')
  unreadCount(@CurrentUser() user: AuthUser): Promise<{ count: number }> {
    return this.notifications.unreadCount(user.userId);
  }

  @Post('read-all')
  @HttpCode(200)
  readAll(@CurrentUser() user: AuthUser): Promise<{ updated: number }> {
    return this.notifications.markAllRead(user.userId);
  }

  @Patch(':id/read')
  read(
    @CurrentUser() user: AuthUser,
    @Param('id') id: string,
  ): Promise<{ id: string; read_at: string }> {
    return this.notifications.markRead(user.userId, id);
  }

  @Put('devices')
  @Throttle({ default: { limit: 10, ttl: 60_000 } })
  registerDevice(
    @CurrentUser() user: AuthUser,
    @Body() dto: RegisterDeviceDto,
  ): Promise<DeviceView> {
    return this.notifications.registerDevice(user.userId, dto.token, dto.platform);
  }

  @Delete('devices/:token')
  @HttpCode(204)
  async removeDevice(@CurrentUser() user: AuthUser, @Param('token') token: string): Promise<void> {
    await this.notifications.removeDevice(user.userId, token);
  }

  @Get('preferences')
  preferences(@CurrentUser() user: AuthUser): Promise<PreferencesView> {
    return this.notifications.preferences(user.userId);
  }

  @Patch('preferences')
  updatePreferences(
    @CurrentUser() user: AuthUser,
    @Body() dto: UpdatePreferencesDto,
  ): Promise<PreferencesView> {
    return this.notifications.updatePreferences(user.userId, dto);
  }
}
