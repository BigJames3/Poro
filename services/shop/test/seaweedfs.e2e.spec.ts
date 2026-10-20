import { HeadObjectCommand } from '@aws-sdk/client-s3';
import sharp from 'sharp';

import { uuidv7 } from '../src/common/uuid';
import { loadConfig } from '../src/config/app.config';
import { MediaService, s3ClientFor } from '../src/media/media.service';

/**
 * Runs the shop image flow against a SeaweedFS configured by
 * infra/storage/init.sh, which checks signatures and access: the presigned
 * POST must be accepted, the original must stay private, the image must be
 * public. infra/storage/tests/run.sh sets S3_INTEGRATION_ENDPOINT and the
 * shop credentials; without them the suite is skipped.
 */
const endpoint = process.env.S3_INTEGRATION_ENDPOINT;
const suite = endpoint ? describe : describe.skip;

suite('shop image storage on SeaweedFS', () => {
  const anonymous = async (path: string): Promise<number> =>
    (await fetch(`${endpoint}/${path}`)).status;

  it('uploads privately, publishes publicly and deletes the original', async () => {
    const config = loadConfig({
      APP_ENV: 'dev',
      DATABASE_URL: 'postgresql://poro:x@localhost:5433/poro_shop',
      S3_ENDPOINT: endpoint,
      S3_PUBLIC_ENDPOINT: endpoint,
      S3_ACCESS_KEY_ID: process.env.S3_SHOP_ACCESS_KEY_ID,
      S3_SECRET_ACCESS_KEY: process.env.S3_SHOP_SECRET_ACCESS_KEY,
    });
    const s3 = s3ClientFor(config, config.s3.endpoint);
    const media = new MediaService(config, s3, s3ClientFor(config, config.s3.publicEndpoint));
    const ownerId = uuidv7();

    const upload = await media.createUpload(ownerId, 'image/png');
    const png = await sharp({
      create: { width: 64, height: 64, channels: 3, background: '#123456' },
    })
      .png()
      .toBuffer();
    const form = new FormData();
    for (const [name, value] of Object.entries(upload.fields)) {
      form.append(name, value);
    }
    form.append('file', new Blob([png], { type: 'image/png' }));
    const posted = await fetch(upload.upload_url, { method: 'POST', body: form });
    expect(posted.status).toBe(204);
    expect(await anonymous(`poro-shop-uploads/${upload.upload_key}`)).toBe(403);

    const key = await media.publish(ownerId, upload.upload_key, 'logo', `shops/${uuidv7()}`);
    expect(await anonymous(`poro-shop/${key}`)).toBe(200);
    expect(media.publicUrl(key)).toBe(`${endpoint}/poro-shop/${key}`);
    await expect(
      s3.send(new HeadObjectCommand({ Bucket: 'poro-shop-uploads', Key: upload.upload_key })),
    ).rejects.toMatchObject({ name: 'NotFound' });

    // An original never confirmed is purged once it is older than the limit
    // (PORO-008): SeaweedFS lists it with its LastModified date.
    const abandoned = await media.createUpload(ownerId, 'image/png');
    const abandonedForm = new FormData();
    for (const [name, value] of Object.entries(abandoned.fields)) {
      abandonedForm.append(name, value);
    }
    abandonedForm.append('file', new Blob([png], { type: 'image/png' }));
    expect(
      (await fetch(abandoned.upload_url, { method: 'POST', body: abandonedForm })).status,
    ).toBe(204);
    expect(await media.purgeStaleUploads(new Date(Date.now() - 3_600_000))).toBe(0);
    expect(await media.purgeStaleUploads(new Date(Date.now() + 60_000))).toBeGreaterThanOrEqual(1);
    await expect(
      s3.send(new HeadObjectCommand({ Bucket: 'poro-shop-uploads', Key: abandoned.upload_key })),
    ).rejects.toMatchObject({ name: 'NotFound' });

    await media.remove(key);
    expect(await anonymous(`poro-shop/${key}`)).toBe(404);
  });
});
