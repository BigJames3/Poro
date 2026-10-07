import { Injectable } from '@nestjs/common';

import { AvatarService } from '../avatars/avatar.service';
import { ApiError } from '../common/api-error';
import {
  TYPE_USER_CREATOR_ACTIVATED,
  TYPE_USER_PROFILE_UPDATED,
  UserCreatorActivatedV1,
  UserProfileUpdatedV1,
} from '../events/catalog';
import { newEnvelope } from '../events/envelope';
import { enqueue } from '../events/outbox';
import { Prisma, Profile } from '../generated/prisma/client';
import { PrismaService } from '../prisma/prisma.service';
import {
  AvatarContentType,
  AvatarUploadView,
  MyProfileView,
  PublicProfileView,
  UpdateProfileDto,
  UsernameAvailabilityView,
  toMyView,
  toPublicView,
} from './profile.dto';
import { normalizeUsername, usernameProblem } from './username';

export const DISPLAY_NAME_MAX = 50;
export const BIO_MAX = 160;
const BIO_MAX_LINES = 5;
// Control characters, bidi overrides and zero-width characters are used to spoof names.
// eslint-disable-next-line no-control-regex
const FORBIDDEN_CHARS = /[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u202a-\u202e\u2066-\u2069\ufeff]/;

function isUniqueViolation(err: unknown): boolean {
  return err instanceof Prisma.PrismaClientKnownRequestError && err.code === 'P2002';
}

// Postgres VARCHAR(n) counts code points, not UTF-16 units or graphemes.
function codePoints(value: string): number {
  // eslint-disable-next-line @typescript-eslint/no-misused-spread
  return [...value].length;
}

function cleanDisplayName(raw: string | null): string | null {
  if (raw === null) {
    return null;
  }
  const value = raw.normalize('NFC').trim().replace(/\s+/g, ' ');
  if (value === '') {
    return null;
  }
  if (codePoints(value) > DISPLAY_NAME_MAX || FORBIDDEN_CHARS.test(value)) {
    throw new ApiError(
      422,
      'display_name_invalid',
      `display name must be at most ${DISPLAY_NAME_MAX} visible characters`,
    );
  }
  return value;
}

function cleanBio(raw: string | null): string | null {
  if (raw === null) {
    return null;
  }
  const value = raw.normalize('NFC').replace(/\r\n?/g, '\n').trim();
  if (value === '') {
    return null;
  }
  const lines = value.split('\n');
  if (
    codePoints(value) > BIO_MAX ||
    lines.length > BIO_MAX_LINES ||
    lines.some((line) => FORBIDDEN_CHARS.test(line))
  ) {
    throw new ApiError(
      422,
      'bio_invalid',
      `bio must be at most ${BIO_MAX} characters and ${BIO_MAX_LINES} lines`,
    );
  }
  return value;
}

/** Fields other services see; a change to one of them publishes poro.user.profile.updated. */
function samePublicFields(a: Profile, b: Profile): boolean {
  return (
    a.username === b.username &&
    a.displayName === b.displayName &&
    a.avatarKey === b.avatarKey &&
    a.isCreator === b.isCreator
  );
}

@Injectable()
export class ProfilesService {
  constructor(
    private readonly prisma: PrismaService,
    private readonly avatars: AvatarService,
  ) {}

  async me(userId: string): Promise<MyProfileView> {
    return this.myView(await this.ensure(userId));
  }

  async update(userId: string, dto: UpdateProfileDto): Promise<MyProfileView> {
    await this.ensure(userId);
    const data: Prisma.ProfileUpdateInput = {};
    if (dto.username !== undefined) {
      data.username = this.checkedUsername(dto.username);
    }
    if (dto.display_name !== undefined) {
      data.displayName = cleanDisplayName(dto.display_name);
    }
    if (dto.bio !== undefined) {
      data.bio = cleanBio(dto.bio);
    }
    try {
      return this.myView(await this.updatePublishing(userId, data));
    } catch (err) {
      if (isUniqueViolation(err)) {
        throw new ApiError(409, 'username_taken', 'username already taken');
      }
      throw err;
    }
  }

  async usernameAvailability(raw: string): Promise<UsernameAvailabilityView> {
    const username = normalizeUsername(raw);
    const problem = usernameProblem(username);
    if (problem) {
      return { username, available: false, reason: problem };
    }
    const taken = await this.prisma.profile.findUnique({
      where: { username },
      select: { userId: true },
    });
    return taken
      ? { username, available: false, reason: 'taken' }
      : { username, available: true, reason: null };
  }

  async byUsername(raw: string): Promise<PublicProfileView> {
    const username = normalizeUsername(raw);
    const profile = usernameProblem(username)
      ? null
      : await this.prisma.profile.findFirst({ where: { username, deletedAt: null } });
    if (!profile) {
      throw new ApiError(404, 'profile_not_found', 'profile not found');
    }
    return toPublicView(profile, (key) => this.avatars.publicUrl(key));
  }

  async createAvatarUpload(
    userId: string,
    contentType: AvatarContentType,
  ): Promise<AvatarUploadView> {
    await this.ensure(userId);
    return this.avatars.createUpload(userId, contentType);
  }

  async setAvatar(userId: string, uploadKey: string): Promise<MyProfileView> {
    const before = await this.ensure(userId);
    const key = await this.avatars.publish(userId, uploadKey);
    let after: Profile;
    try {
      after = await this.updatePublishing(userId, { avatarKey: key });
    } catch (err) {
      await this.avatars.remove(key);
      throw err;
    }
    if (before.avatarKey !== null) {
      await this.avatars.remove(before.avatarKey);
    }
    return this.myView(after);
  }

  async removeAvatar(userId: string): Promise<MyProfileView> {
    const before = await this.ensure(userId);
    if (before.avatarKey === null) {
      return this.myView(before);
    }
    const after = await this.updatePublishing(userId, { avatarKey: null });
    await this.avatars.remove(before.avatarKey);
    return this.myView(after);
  }

  /**
   * Turns the profile into a creator and publishes poro.user.creator.activated
   * in the same transaction; auth then grants the CREATOR role. Idempotent: a
   * second call returns the profile and publishes nothing.
   */
  async activateCreator(userId: string): Promise<MyProfileView> {
    await this.ensure(userId);
    const now = new Date();
    const profile = await this.prisma.$transaction(async (tx) => {
      const { count } = await tx.profile.updateMany({
        where: { userId, isCreator: false, deletedAt: null, username: { not: null } },
        data: { isCreator: true, creatorSince: now },
      });
      const current = await tx.profile.findUniqueOrThrow({ where: { userId } });
      if (count === 1 && current.username !== null) {
        const data: UserCreatorActivatedV1 = {
          user_id: userId,
          username: current.username,
          activated_at: now.toISOString(),
        };
        await enqueue(tx, newEnvelope(TYPE_USER_CREATOR_ACTIVATED, 1, userId, data, now));
        await this.publishProfile(tx, current);
      }
      return current;
    });
    if (!profile.isCreator) {
      throw new ApiError(422, 'username_required', 'choose a username before becoming a creator');
    }
    return this.myView(profile);
  }

  /** Returns the active profile, creating it when poro.auth.user.created has not arrived yet. */
  private async ensure(userId: string): Promise<Profile> {
    let profile: Profile;
    try {
      profile = await this.prisma.profile.upsert({
        where: { userId },
        create: { userId },
        update: {},
      });
    } catch (err) {
      if (!isUniqueViolation(err)) {
        throw err;
      }
      profile = await this.prisma.profile.findUniqueOrThrow({ where: { userId } });
    }
    if (profile.deletedAt !== null) {
      throw new ApiError(404, 'profile_not_found', 'profile not found');
    }
    return profile;
  }

  /**
   * Applies data and, when a public field changed, enqueues
   * poro.user.profile.updated in the same transaction. The row lock orders
   * concurrent updates so each snapshot is compared with the state it replaced.
   */
  private async updatePublishing(
    userId: string,
    data: Prisma.ProfileUpdateInput,
  ): Promise<Profile> {
    return this.prisma.$transaction(async (tx) => {
      await tx.$queryRaw`SELECT 1 FROM profiles WHERE user_id = ${userId}::uuid FOR UPDATE`;
      const before = await tx.profile.findUniqueOrThrow({ where: { userId } });
      const after = await tx.profile.update({ where: { userId }, data });
      if (!samePublicFields(before, after)) {
        await this.publishProfile(tx, after);
      }
      return after;
    });
  }

  private async publishProfile(tx: Prisma.TransactionClient, profile: Profile): Promise<void> {
    const data: UserProfileUpdatedV1 = {
      user_id: profile.userId,
      username: profile.username,
      display_name: profile.displayName,
      avatar_url: profile.avatarKey === null ? null : this.avatars.publicUrl(profile.avatarKey),
      is_creator: profile.isCreator,
      updated_at: profile.updatedAt.toISOString(),
    };
    await enqueue(
      tx,
      newEnvelope(TYPE_USER_PROFILE_UPDATED, 1, profile.userId, data, profile.updatedAt),
    );
  }

  private checkedUsername(raw: string): string {
    const username = normalizeUsername(raw);
    const problem = usernameProblem(username);
    if (problem === 'invalid') {
      throw new ApiError(
        422,
        'username_invalid',
        'username must be 3 to 30 characters: letters, digits, "." or "_"',
      );
    }
    if (problem === 'reserved') {
      throw new ApiError(422, 'username_reserved', 'this username is reserved');
    }
    return username;
  }

  private myView(profile: Profile): MyProfileView {
    return toMyView(profile, (key) => this.avatars.publicUrl(key));
  }
}
