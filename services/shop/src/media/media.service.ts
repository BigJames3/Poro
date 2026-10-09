import {
  DeleteObjectCommand,
  GetObjectCommand,
  NoSuchKey,
  PutObjectCommand,
  S3Client,
} from '@aws-sdk/client-s3';
import { createPresignedPost } from '@aws-sdk/s3-presigned-post';
import { Inject, Injectable, Logger } from '@nestjs/common';
import sharp from 'sharp';

import { ApiError } from '../common/api-error';
import { uuidv7 } from '../common/uuid';
import { appConfig, type AppConfigType } from '../config/app.config';

export const S3_CLIENT = Symbol('S3_CLIENT');
export const S3_PRESIGN_CLIENT = Symbol('S3_PRESIGN_CLIENT');

export const IMAGE_CONTENT_TYPES = ['image/jpeg', 'image/png', 'image/webp'] as const;
export type ImageContentType = (typeof IMAGE_CONTENT_TYPES)[number];

export const IMAGE_MAX_BYTES = 5 * 1024 * 1024;
export const IMAGE_UPLOAD_TTL_SECONDS = 300;
export const LOGO_SIZE_PX = 512;
export const PRODUCT_IMAGE_MAX_PX = 1080;
const MAX_INPUT_PIXELS = 40_000_000;
const ACCEPTED_FORMATS = new Set(['jpeg', 'png', 'webp']);
const UPLOAD_PREFIX = 'uploads';

/** Logos are square crops; product photos keep their ratio within 1080 px. */
export type ImageKind = 'logo' | 'product';

export interface ImageUploadView {
  upload_url: string;
  fields: Record<string, string>;
  upload_key: string;
  max_bytes: number;
  expires_in: number;
}

export function s3ClientFor(config: AppConfigType, endpoint: string): S3Client {
  return new S3Client({
    endpoint,
    region: config.s3.region,
    forcePathStyle: config.s3.forcePathStyle,
    credentials: {
      accessKeyId: config.s3.accessKeyId,
      secretAccessKey: config.s3.secretAccessKey,
    },
  });
}

/**
 * Clients upload the original straight to object storage with a short-lived
 * presigned POST. On confirmation the service re-encodes it to WebP: this
 * rejects anything that is not a real image and strips metadata such as EXIF
 * GPS coordinates before the image becomes public.
 */
@Injectable()
export class MediaService {
  private readonly logger = new Logger(MediaService.name);
  private readonly bucket: string;
  private readonly publicBaseUrl: string;

  constructor(
    @Inject(appConfig.KEY) config: AppConfigType,
    @Inject(S3_CLIENT) private readonly s3: S3Client,
    @Inject(S3_PRESIGN_CLIENT) private readonly presignS3: S3Client,
  ) {
    this.bucket = config.s3.bucket;
    this.publicBaseUrl = config.mediaPublicBaseUrl;
  }

  publicUrl(key: string): string {
    return `${this.publicBaseUrl}/${key}`;
  }

  async createUpload(ownerId: string, contentType: ImageContentType): Promise<ImageUploadView> {
    const key = `${UPLOAD_PREFIX}/${ownerId}/${uuidv7()}`;
    const { url, fields } = await createPresignedPost(this.presignS3, {
      Bucket: this.bucket,
      Key: key,
      Conditions: [
        ['content-length-range', 1, IMAGE_MAX_BYTES],
        ['eq', '$Content-Type', contentType],
      ],
      Fields: { 'Content-Type': contentType },
      Expires: IMAGE_UPLOAD_TTL_SECONDS,
    });
    return {
      upload_url: url,
      fields,
      upload_key: key,
      max_bytes: IMAGE_MAX_BYTES,
      expires_in: IMAGE_UPLOAD_TTL_SECONDS,
    };
  }

  /**
   * Validates and re-encodes an upload of ownerId, stores it under prefix and
   * returns its public key. The raw upload is deleted.
   */
  async publish(
    ownerId: string,
    uploadKey: string,
    kind: ImageKind,
    prefix: string,
  ): Promise<string> {
    if (!isUploadKeyOf(ownerId, uploadKey)) {
      throw new ApiError(422, 'image_upload_invalid', 'unknown upload key');
    }
    const original = await this.download(uploadKey);
    const encoded = await reencode(original, kind);
    const key = `${prefix}/${uuidv7()}.webp`;
    await this.s3.send(
      new PutObjectCommand({
        Bucket: this.bucket,
        Key: key,
        Body: encoded,
        ContentType: 'image/webp',
        CacheControl: 'public, max-age=31536000, immutable',
      }),
    );
    await this.remove(uploadKey);
    return key;
  }

  /** Best effort: an orphan object costs storage, not correctness. */
  async remove(key: string): Promise<void> {
    try {
      await this.s3.send(new DeleteObjectCommand({ Bucket: this.bucket, Key: key }));
    } catch (err) {
      this.logger.warn({ msg: 'image object not deleted', key, err });
    }
  }

  private async download(key: string): Promise<Buffer> {
    let body;
    try {
      const res = await this.s3.send(new GetObjectCommand({ Bucket: this.bucket, Key: key }));
      if ((res.ContentLength ?? 0) > IMAGE_MAX_BYTES) {
        await this.remove(key);
        throw new ApiError(422, 'image_too_large', 'image is larger than 5 MB');
      }
      body = await res.Body?.transformToByteArray();
    } catch (err) {
      if (err instanceof ApiError) {
        throw err;
      }
      if (err instanceof NoSuchKey || (err as { name?: string }).name === 'NoSuchKey') {
        throw new ApiError(422, 'image_not_uploaded', 'upload the image before confirming it');
      }
      throw new ApiError(503, 'unavailable', 'image storage unavailable', err);
    }
    if (!body || body.byteLength === 0 || body.byteLength > IMAGE_MAX_BYTES) {
      throw new ApiError(422, 'image_invalid', 'image is not a supported image');
    }
    return Buffer.from(body);
  }
}

export function isUploadKeyOf(ownerId: string, key: string): boolean {
  const prefix = `${UPLOAD_PREFIX}/${ownerId}/`;
  return (
    key.startsWith(prefix) &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(
      key.slice(prefix.length),
    )
  );
}

export async function reencode(input: Buffer, kind: ImageKind): Promise<Buffer> {
  try {
    const image = sharp(input, { limitInputPixels: MAX_INPUT_PIXELS, failOn: 'error' });
    const { format } = await image.metadata();
    if (!ACCEPTED_FORMATS.has(format)) {
      throw new Error(`format ${format} not accepted`);
    }
    const resized =
      kind === 'logo'
        ? image.rotate().resize(LOGO_SIZE_PX, LOGO_SIZE_PX, { fit: 'cover' })
        : image.rotate().resize(PRODUCT_IMAGE_MAX_PX, PRODUCT_IMAGE_MAX_PX, {
            fit: 'inside',
            withoutEnlargement: true,
          });
    return await resized.webp({ quality: 82 }).toBuffer();
  } catch (err) {
    throw new ApiError(422, 'image_invalid', 'image is not a supported image', err);
  }
}
