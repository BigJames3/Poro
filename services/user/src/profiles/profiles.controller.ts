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

import { CurrentUser, Public } from '../auth/auth.guard';
import type { AuthUser } from '../common/request';
import {
  AvatarUploadDto,
  ConfirmAvatarDto,
  UpdateProfileDto,
  UsernameQueryDto,
  type AvatarUploadView,
  type MyProfileView,
  type PublicProfileView,
  type UsernameAvailabilityView,
} from './profile.dto';
import { ProfilesService } from './profiles.service';

@Controller('api/v1/users')
export class ProfilesController {
  constructor(private readonly profiles: ProfilesService) {}

  @Get('me')
  me(@CurrentUser() user: AuthUser): Promise<MyProfileView> {
    return this.profiles.me(user.userId);
  }

  @Patch('me')
  update(@CurrentUser() user: AuthUser, @Body() dto: UpdateProfileDto): Promise<MyProfileView> {
    return this.profiles.update(user.userId, dto);
  }

  @Throttle({ default: { limit: 30, ttl: 60_000 } })
  @Get('username-availability')
  usernameAvailability(@Query() query: UsernameQueryDto): Promise<UsernameAvailabilityView> {
    return this.profiles.usernameAvailability(query.username);
  }

  @Public()
  @Get('by-username/:username')
  byUsername(@Param('username') username: string): Promise<PublicProfileView> {
    return this.profiles.byUsername(username);
  }

  @Throttle({ default: { limit: 10, ttl: 60_000 } })
  @Post('me/avatar/upload-url')
  @HttpCode(201)
  avatarUpload(
    @CurrentUser() user: AuthUser,
    @Body() dto: AvatarUploadDto,
  ): Promise<AvatarUploadView> {
    return this.profiles.createAvatarUpload(user.userId, dto.content_type);
  }

  @Throttle({ default: { limit: 10, ttl: 60_000 } })
  @Put('me/avatar')
  setAvatar(@CurrentUser() user: AuthUser, @Body() dto: ConfirmAvatarDto): Promise<MyProfileView> {
    return this.profiles.setAvatar(user.userId, dto.upload_key);
  }

  @Delete('me/avatar')
  removeAvatar(@CurrentUser() user: AuthUser): Promise<MyProfileView> {
    return this.profiles.removeAvatar(user.userId);
  }

  @Post('me/creator')
  @HttpCode(200)
  activateCreator(@CurrentUser() user: AuthUser): Promise<MyProfileView> {
    return this.profiles.activateCreator(user.userId);
  }
}
