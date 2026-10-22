import {
  DeleteObjectCommand,
  ListObjectsV2Command,
  NoSuchKey,
  S3Client,
} from '@aws-sdk/client-s3';

import { type AppConfigType } from '../config/app.config';
import { AvatarService } from './avatar.service';
import { ABANDONED_UPLOAD_AGE_MS, UploadSweeper } from './upload-sweeper';

const config = {
  s3: { bucket: 'poro-avatars', uploadBucket: 'poro-avatar-uploads' },
  avatarPublicBaseUrl: 'http://localhost:9000/poro-avatars',
} as AppConfigType;

/** A bucket of pending originals, listed two keys per page. */
function fakeS3(objects: Record<string, Date>) {
  const deleted: string[] = [];
  const s3 = {
    send: jest.fn((command: unknown) => {
      if (command instanceof ListObjectsV2Command) {
        expect(command.input).toMatchObject({ Bucket: 'poro-avatar-uploads', Prefix: 'avatars/uploads/' });
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
        if (key.endsWith('gone')) {
          return Promise.reject(new NoSuchKey({ message: 'gone', $metadata: {} }));
        }
        deleted.push(key);
        return Promise.resolve({});
      }
      return Promise.reject(new Error('unexpected command'));
    }),
  } as unknown as S3Client;
  return { s3, deleted };
}

describe('abandoned avatar uploads', () => {
  const now = new Date('2026-10-22T12:00:00Z');
  const old = new Date(now.getTime() - ABANDONED_UPLOAD_AGE_MS - 1000);
  const recent = new Date(now.getTime() - 60_000);

  it('deletes only the originals older than a day, across pages', async () => {
    const { s3, deleted } = fakeS3({
      'avatars/uploads/a/1': old,
      'avatars/uploads/a/2': recent,
      'avatars/uploads/b/3': old,
      'avatars/uploads/c/4': recent,
      'avatars/uploads/d/5': old,
    });
    const avatars = new AvatarService(config, s3, s3);
    const count = await avatars.purgeAbandonedUploads(new Date(now.getTime() - ABANDONED_UPLOAD_AGE_MS));
    expect(count).toBe(3);
    expect(deleted).toEqual(['avatars/uploads/a/1', 'avatars/uploads/b/3', 'avatars/uploads/d/5']);
  });

  it('counts an object already deleted by another replica as deleted', async () => {
    const { s3 } = fakeS3({ 'avatars/uploads/a/gone': old });
    const avatars = new AvatarService(config, s3, s3);
    await expect(avatars.purgeAbandonedUploads(now)).resolves.toBe(1);
  });

  it('the sweeper uses a one day cutoff and survives a storage failure', async () => {
    const purge = jest.fn().mockRejectedValueOnce(new Error('down')).mockResolvedValue(2);
    const sweeper = new UploadSweeper({ purgeAbandonedUploads: purge } as unknown as AvatarService);
    await expect(sweeper.tick(now)).resolves.toBeUndefined();
    await sweeper.tick(now);
    expect(purge).toHaveBeenLastCalledWith(new Date(now.getTime() - ABANDONED_UPLOAD_AGE_MS));
  });
});
