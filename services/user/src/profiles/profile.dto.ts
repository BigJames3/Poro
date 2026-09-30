import { IsIn, IsOptional, IsString, MaxLength } from 'class-validator';

import { Profile } from '../generated/prisma/client';

export const AVATAR_CONTENT_TYPES = ['image/jpeg', 'image/png', 'image/webp'] as const;
export type AvatarContentType = (typeof AVATAR_CONTENT_TYPES)[number];

/** Undefined keeps a field; null clears display_name or bio. */
export class UpdateProfileDto {
  @IsOptional()
  @IsString()
  @MaxLength(64)
  username?: string;

  @IsOptional()
  @IsString()
  @MaxLength(200)
  display_name?: string | null;

  @IsOptional()
  @IsString()
  @MaxLength(1000)
  bio?: string | null;
}

export class UsernameQueryDto {
  @IsString()
  @MaxLength(64)
  username!: string;
}

export class AvatarUploadDto {
  @IsIn(AVATAR_CONTENT_TYPES)
  content_type!: AvatarContentType;
}

export class ConfirmAvatarDto {
  @IsString()
  @MaxLength(200)
  upload_key!: string;
}

export interface PublicProfileView {
  user_id: string;
  username: string | null;
  display_name: string | null;
  bio: string | null;
  avatar_url: string | null;
  is_creator: boolean;
  creator_since: string | null;
}

export interface MyProfileView extends PublicProfileView {
  country_code: string | null;
  created_at: string;
  updated_at: string;
}

export interface UsernameAvailabilityView {
  username: string;
  available: boolean;
  reason: 'invalid' | 'reserved' | 'taken' | null;
}

export interface AvatarUploadView {
  upload_url: string;
  fields: Record<string, string>;
  upload_key: string;
  max_bytes: number;
  expires_in: number;
}

export function toPublicView(
  profile: Profile,
  avatarUrl: (key: string) => string,
): PublicProfileView {
  return {
    user_id: profile.userId,
    username: profile.username,
    display_name: profile.displayName,
    bio: profile.bio,
    avatar_url: profile.avatarKey === null ? null : avatarUrl(profile.avatarKey),
    is_creator: profile.isCreator,
    creator_since: profile.creatorSince?.toISOString() ?? null,
  };
}

export function toMyView(profile: Profile, avatarUrl: (key: string) => string): MyProfileView {
  return {
    ...toPublicView(profile, avatarUrl),
    country_code: profile.countryCode,
    created_at: profile.createdAt.toISOString(),
    updated_at: profile.updatedAt.toISOString(),
  };
}
