import { DeleteObjectCommand, ListObjectsV2Command, S3Client } from '@aws-sdk/client-s3';
import { mockClient } from 'aws-sdk-client-mock';

import { loadConfig } from '../config/app.config';
import { AvatarService } from './avatar.service';
import { UploadPurgeTask } from './upload-purge.task';

const HOUR = 3_600_000;

describe('UploadPurgeTask', () => {
  const now = new Date('2026-10-20T12:00:00Z');
  const config = loadConfig({
    APP_ENV: 'dev',
    DATABASE_URL: 'postgresql://poro:x@localhost:5433/poro',
    S3_ACCESS_KEY_ID: 'k',
    S3_SECRET_ACCESS_KEY: 's',
  });
  const s3 = mockClient(S3Client);
  const client = new S3Client({ region: 'us-east-1' });
  const task = new UploadPurgeTask(new AvatarService(config, client, client), config);

  beforeEach(() => s3.reset());

  it('deletes the originals older than the limit, page after page, and only them', async () => {
    const prefix = 'avatars/uploads/';
    s3.on(ListObjectsV2Command, { ContinuationToken: undefined }).resolves({
      Contents: [
        { Key: `${prefix}u1/old`, LastModified: new Date(now.getTime() - 25 * HOUR) },
        { Key: `${prefix}u1/recent`, LastModified: new Date(now.getTime() - 23 * HOUR) },
      ],
      IsTruncated: true,
      NextContinuationToken: 'p2',
    });
    s3.on(ListObjectsV2Command, { ContinuationToken: 'p2' }).resolves({
      Contents: [
        { Key: `${prefix}u2/older`, LastModified: new Date(now.getTime() - 72 * HOUR) },
        { Key: `${prefix}u2/undated` },
      ],
      IsTruncated: false,
    });
    s3.on(DeleteObjectCommand).resolves({});

    await expect(task.run(now)).resolves.toBe(2);
    const listed = s3.commandCalls(ListObjectsV2Command).map((c) => c.args[0].input);
    expect(listed.every((input) => input.Bucket === config.s3.uploadBucket)).toBe(true);
    expect(listed.every((input) => input.Prefix === prefix)).toBe(true);
    const deleted = s3.commandCalls(DeleteObjectCommand).map((c) => c.args[0].input);
    expect(deleted).toEqual([
      { Bucket: config.s3.uploadBucket, Key: `${prefix}u1/old` },
      { Bucket: config.s3.uploadBucket, Key: `${prefix}u2/older` },
    ]);
  });

  it('logs a storage failure and lets the next pass retry', async () => {
    s3.on(ListObjectsV2Command).rejects(new Error('503 slow down'));
    await expect(task.run(now)).resolves.toBe(0);
  });

  it('runs on a timer that stops with the application', async () => {
    jest.useFakeTimers();
    try {
      s3.on(ListObjectsV2Command).resolves({ Contents: [], IsTruncated: false });
      task.onApplicationBootstrap();
      expect(s3.commandCalls(ListObjectsV2Command)).toHaveLength(0);
      await jest.advanceTimersByTimeAsync(HOUR);
      expect(s3.commandCalls(ListObjectsV2Command)).toHaveLength(1);
      await task.onApplicationShutdown();
      await jest.advanceTimersByTimeAsync(2 * HOUR);
      expect(s3.commandCalls(ListObjectsV2Command)).toHaveLength(1);
    } finally {
      jest.useRealTimers();
    }
  });
});
