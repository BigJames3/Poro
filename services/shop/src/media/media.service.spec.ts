import sharp from 'sharp';

import { ApiError } from '../common/api-error';
import { uuidv7 } from '../common/uuid';
import { isUploadKeyOf, reencode } from './media.service';

async function jpegWithGps(width: number, height: number): Promise<Buffer> {
  return sharp({ create: { width, height, channels: 3, background: '#0a7' } })
    .jpeg()
    .withExif({
      IFD0: { Make: 'PhoneMaker', Copyright: 'Awa' },
      IFD3: { GPSLatitudeRef: 'N', GPSLatitude: '5/1 19/1 0/1' },
    })
    .toBuffer();
}

describe('image processing', () => {
  it('re-encodes a logo to a 512px WebP square and strips metadata', async () => {
    const input = await jpegWithGps(1200, 800);
    expect((await sharp(input).metadata()).exif).toBeDefined();

    const meta = await sharp(await reencode(input, 'logo')).metadata();
    expect(meta).toMatchObject({ format: 'webp', width: 512, height: 512 });
    expect(meta.exif).toBeUndefined();
    expect(meta.icc).toBeUndefined();
  });

  it('keeps the ratio of a product photo within 1080px', async () => {
    const meta = await sharp(await reencode(await jpegWithGps(2400, 1200), 'product')).metadata();
    expect(meta).toMatchObject({ format: 'webp', width: 1080, height: 540 });
    expect(meta.exif).toBeUndefined();
  });

  it('never enlarges a small product photo', async () => {
    const meta = await sharp(await reencode(await jpegWithGps(300, 200), 'product')).metadata();
    expect(meta).toMatchObject({ width: 300, height: 200 });
  });

  it.each([
    ['html', Buffer.from('<svg onload=alert(1)>')],
    ['empty', Buffer.alloc(0)],
    ['truncated png', Buffer.from('89504e470d0a1a0a0000000d49484452', 'hex')],
  ])('rejects %s', async (_name, input) => {
    await expect(reencode(input, 'product')).rejects.toMatchObject({ code: 'image_invalid' });
  });

  it('rejects formats outside jpeg, png and webp', async () => {
    const gif = await sharp({ create: { width: 10, height: 10, channels: 3, background: '#000' } })
      .gif()
      .toBuffer();
    const err = await reencode(gif, 'logo').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).getStatus()).toBe(422);
  });

  it('only accepts upload keys issued to the same account', () => {
    const ownerId = uuidv7();
    expect(isUploadKeyOf(ownerId, `uploads/${ownerId}/${uuidv7()}`)).toBe(true);
    expect(isUploadKeyOf(ownerId, `uploads/${uuidv7()}/${uuidv7()}`)).toBe(false);
    expect(isUploadKeyOf(ownerId, `uploads/${ownerId}/../${uuidv7()}`)).toBe(false);
    expect(isUploadKeyOf(ownerId, `shops/${ownerId}/${uuidv7()}.webp`)).toBe(false);
  });
});
