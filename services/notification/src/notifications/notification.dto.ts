import { Type } from 'class-transformer';
import {
  IsBoolean,
  IsIn,
  IsInt,
  IsOptional,
  IsString,
  Matches,
  Max,
  MaxLength,
  Min,
} from 'class-validator';

import type { NotificationType } from './notification.types';

export const DEFAULT_PAGE_SIZE = 20;
export const MAX_PAGE_SIZE = 50;
export const PLATFORMS = ['android', 'ios', 'web'] as const;
export type Platform = (typeof PLATFORMS)[number];

export class ListNotificationsDto {
  @IsOptional()
  @IsString()
  @MaxLength(200)
  cursor?: string;

  @IsOptional()
  @Type(() => Number)
  @IsInt({ message: 'limit must be an integer' })
  @Min(1, { message: `limit must be between 1 and ${MAX_PAGE_SIZE}` })
  @Max(MAX_PAGE_SIZE, {
    message: `limit must be between 1 and ${MAX_PAGE_SIZE}`,
  })
  limit?: number;
}

export class RegisterDeviceDto {
  // FCM tokens are opaque; this bounds them to the characters they use.
  @IsString()
  @Matches(/^[A-Za-z0-9:_-]{20,512}$/, {
    message: 'token is not a valid FCM token',
  })
  token!: string;

  @IsIn(PLATFORMS, {
    message: `platform must be one of ${PLATFORMS.join(', ')}`,
  })
  platform!: Platform;
}

export class UpdatePreferencesDto {
  @IsOptional()
  @IsBoolean()
  push_enabled?: boolean;

  @IsOptional()
  @IsBoolean()
  likes?: boolean;

  @IsOptional()
  @IsBoolean()
  comments?: boolean;

  @IsOptional()
  @IsBoolean()
  follows?: boolean;

  @IsOptional()
  @IsBoolean()
  video_ready?: boolean;
}

export interface ActorView {
  id: string;
  username: string | null;
  display_name: string | null;
  avatar_url: string | null;
}

export interface NotificationView {
  id: string;
  type: NotificationType;
  title: string;
  body: string;
  data: Record<string, string>;
  actor: ActorView | null;
  actor_count: number;
  entity_type: string;
  entity_id: string;
  read: boolean;
  read_at: string | null;
  created_at: string;
  last_activity_at: string;
}

export interface NotificationPage {
  items: NotificationView[];
  next_cursor: string | null;
}

export interface DeviceView {
  token: string;
  platform: Platform;
  created_at: string;
  updated_at: string;
}

export interface PreferencesView {
  push_enabled: boolean;
  likes: boolean;
  comments: boolean;
  follows: boolean;
  video_ready: boolean;
}
