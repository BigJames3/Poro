import {
  DeleteObjectCommand,
  ListObjectsV2Command,
  S3Client,
} from '@aws-sdk/client-s3';

import { type AppConfigType } from '../config/app.config';
import { MediaService } from './media.service';
import { ABANDONED_UPLOAD_AGE_MS, UploadSweeper } from './upload-sweeper';

const config = {
  s3: { bucket: 'poro-shop-media', uploadBucket: 'poro-shop-uploads' },
  mediaPublicBaseUrl: 'http://localhost:9000/poro-shop-media',
} as AppConfigType;

/** A bucket of pending originals, listed two keys per page. */
function fakeS3(objects: Record<string, Date>) {
  const deleted: string[] = [];
  const s3 = {
    send: jest.fn((command: unknown) => {
      if (command instanceof ListObjectsV2Command) {
        expect(command.input).toMatchObject({ Bucket: 'poro-shop-uploads', Prefix: 'uploads/' });
        const keys = Object.keys(objects).sort();
        const start = Number(command.input.ContinuationToken ?? 0);
        const page = keys.slice(start, start + 2);
        const more = start + 2 < keys.length;
        return Promise.resolve({
          Contents: page.map((Key) => ({ Key, LastModified: objects[Key] })),
          IsTruncated: more,
          NextContinuationToken: more ? String(start + 2) : undefined,
        });
      }
      if (command instanceof DeleteObjectCommand) {
        const key = command.input.Key as string;
        deleted.push(key);
        return Promise.resolve({});
      }
      return Promise.reject(new Error('unexpected command'));
    }),
  } as unknown as S3Client;
  return { s3, deleted };
}

describe('abandoned image uploads', () => {
  const now = new Date('2026-10-22T12:00:00Z');
  const old = new Date(now.getTime() - ABANDONED_UPLOAD_AGE_MS - 1000);
  const recent = new Date(now.getTime() - 60_000);

  it('deletes only the originals older than a day, across pages', async () => {
    const { s3, deleted } = fakeS3({
      'uploads/a/1': old,
      'uploads/a/2': recent,
      'uploads/b/3': old,
      'uploads/c/4': recent,
      'uploads/d/5': old,
    });
    const avatars = new MediaService(config, s3, s3);
    const count = await avatars.purgeAbandonedUploads(new Date(now.getTime() - ABANDONED_UPLOAD_AGE_MS));
    expect(count).toBe(3);
    expect(deleted).toEqual(['uploads/a/1', 'uploads/b/3', 'uploads/d/5']);
  });

  it('keeps every original when none is old enough', async () => {
    const { s3, deleted } = fakeS3({ 'uploads/a/1': recent });
    const media = new MediaService(config, s3, s3);
    await expect(media.purgeAbandonedUploads(new Date(now.getTime() - ABANDONED_UPLOAD_AGE_MS))).resolves.toBe(0);
    expect(deleted).toEqual([]);
  });

  it('the sweeper uses a one day cutoff and survives a storage failure', async () => {
    const purge = jest.fn().mockRejectedValueOnce(new Error('down')).mockResolvedValue(2);
    const sweeper = new UploadSweeper({ purgeAbandonedUploads: purge } as unknown as MediaService);
    await expect(sweeper.tick(now)).resolves.toBeUndefined();
    await sweeper.tick(now);
    expect(purge).toHaveBeenLastCalledWith(new Date(now.getTime() - ABANDONED_UPLOAD_AGE_MS));
  });
});
