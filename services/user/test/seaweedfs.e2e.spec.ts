import { HeadObjectCommand } from '@aws-sdk/client-s3';
import sharp from 'sharp';

import { uuidv7 } from '../src/common/uuid';
import { AvatarService, s3ClientFor } from '../src/avatars/avatar.service';
import { loadConfig } from '../src/config/app.config';

/**
 * Runs the avatar flow against a SeaweedFS configured by
 * infra/storage/init.sh, which checks signatures and access: the presigned
 * POST must be accepted, the original must stay private, the avatar must be
 * public. infra/storage/tests/run.sh sets S3_INTEGRATION_ENDPOINT and the
 * user credentials; without them the suite is skipped.
 */
const endpoint = process.env.S3_INTEGRATION_ENDPOINT;
const suite = endpoint ? describe : describe.skip;

suite('avatar storage on SeaweedFS', () => {
  const anonymous = async (path: string): Promise<number> =>
    (await fetch(`${endpoint}/${path}`)).status;

  it('uploads privately, publishes publicly and deletes the original', async () => {
    const config = loadConfig({
      DATABASE_URL: 'postgresql://poro:x@localhost:5433/poro_user',
      S3_ENDPOINT: endpoint,
      S3_PUBLIC_ENDPOINT: endpoint,
      S3_ACCESS_KEY_ID: process.env.S3_USER_ACCESS_KEY_ID,
      S3_SECRET_ACCESS_KEY: process.env.S3_USER_SECRET_ACCESS_KEY,
    });
    const s3 = s3ClientFor(config, config.s3.endpoint);
    const avatars = new AvatarService(config, s3, s3ClientFor(config, config.s3.publicEndpoint));
    const userId = uuidv7();

    const upload = await avatars.createUpload(userId, 'image/png');
    const png = await sharp({
      create: { width: 64, height: 64, channels: 3, background: '#e07a1f' },
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
    expect(await anonymous(`poro-avatar-uploads/${upload.upload_key}`)).toBe(403);

    const key = await avatars.publish(userId, upload.upload_key);
    expect(await anonymous(`poro-avatars/${key}`)).toBe(200);
    expect(avatars.publicUrl(key)).toBe(`${endpoint}/poro-avatars/${key}`);

    await expect(
      s3.send(new HeadObjectCommand({ Bucket: 'poro-avatar-uploads', Key: upload.upload_key })),
    ).rejects.toMatchObject({ name: 'NotFound' });
    await avatars.remove(key);
    expect(await anonymous(`poro-avatars/${key}`)).toBe(404);
  });
});
