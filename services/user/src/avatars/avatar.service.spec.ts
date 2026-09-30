import sharp from 'sharp';

import { ApiError } from '../common/api-error';
import { uuidv7 } from '../common/uuid';
import { isUploadKeyOf, reencode } from './avatar.service';

describe('avatar processing', () => {
  it('re-encodes to a 512px WebP square and strips metadata', async () => {
    const withGps = await sharp({
      create: { width: 1200, height: 800, channels: 3, background: '#0a7' },
    })
      .jpeg()
      .withExif({
        IFD0: { Make: 'PhoneMaker', Copyright: 'Awa' },
        IFD3: { GPSLatitudeRef: 'N', GPSLatitude: '5/1 19/1 0/1' },
      })
      .toBuffer();
    expect((await sharp(withGps).metadata()).exif).toBeDefined();

    const out = await reencode(withGps);
    const meta = await sharp(out).metadata();
    expect(meta).toMatchObject({ format: 'webp', width: 512, height: 512 });
    expect(meta.exif).toBeUndefined();
    expect(meta.icc).toBeUndefined();
  });

  it.each([
    ['html', Buffer.from('<svg onload=alert(1)>')],
    ['empty', Buffer.alloc(0)],
    ['truncated png', Buffer.from('89504e470d0a1a0a0000000d49484452', 'hex')],
  ])('rejects %s', async (_name, input) => {
    await expect(reencode(input)).rejects.toMatchObject({ code: 'avatar_invalid' });
  });

  it('rejects formats outside jpeg, png and webp', async () => {
    const gif = await sharp({ create: { width: 10, height: 10, channels: 3, background: '#000' } })
      .gif()
      .toBuffer();
    const err = await reencode(gif).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).getStatus()).toBe(422);
  });

  it('only accepts upload keys issued to the same account', () => {
    const userId = uuidv7();
    expect(isUploadKeyOf(userId, `avatars/uploads/${userId}/${uuidv7()}`)).toBe(true);
    expect(isUploadKeyOf(userId, `avatars/uploads/${uuidv7()}/${uuidv7()}`)).toBe(false);
    expect(isUploadKeyOf(userId, `avatars/uploads/${userId}/../${uuidv7()}`)).toBe(false);
    expect(isUploadKeyOf(userId, `avatars/${userId}/${uuidv7()}.webp`)).toBe(false);
  });
});
