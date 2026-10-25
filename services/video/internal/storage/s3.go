package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/poro/video/internal/config"
	"github.com/poro/video/internal/model"
)

// S3 talks to a path-style S3-compatible store (SeaweedFS in development).
// Sources go to a private bucket; only HLS renditions and thumbnails go to the
// public media bucket.
type S3 struct {
	internal   *s3.Client
	presign    *s3.PresignClient
	sources    string
	media      string
	quarantine string
}

// NewS3 builds two clients: one for the service, one whose presigned URLs
// point at S3_PUBLIC_ENDPOINT so the mobile app can PUT parts.
func NewS3(ctx context.Context, cfg *config.Config) (*S3, error) {
	internal, err := s3Client(ctx, cfg, cfg.S3Endpoint)
	if err != nil {
		return nil, err
	}
	public, err := s3Client(ctx, cfg, cfg.S3PublicEndpoint)
	if err != nil {
		return nil, err
	}
	return &S3{
		internal:   internal,
		presign:    s3.NewPresignClient(public),
		sources:    cfg.S3SourceBucket,
		media:      cfg.S3Bucket,
		quarantine: cfg.S3QuarantineBucket,
	}, nil
}

func s3Client(ctx context.Context, cfg *config.Config, endpoint string) (*s3.Client, error) {
	load, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return s3.NewFromConfig(load, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = cfg.S3ForcePathStyle
	}), nil
}

func (s *S3) CreateMultipart(ctx context.Context, key, contentType string) (string, error) {
	out, err := s.internal.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.sources),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("create multipart: %w", err)
	}
	return aws.ToString(out.UploadId), nil
}

func (s *S3) PresignPart(ctx context.Context, key, uploadID string, part int32, size int64) (string, error) {
	out, err := s.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:        aws.String(s.sources),
		Key:           aws.String(key),
		UploadId:      aws.String(uploadID),
		PartNumber:    aws.Int32(part),
		ContentLength: aws.Int64(size),
	}, func(o *s3.PresignOptions) {
		o.Expires = time.Hour
	})
	if err != nil {
		return "", fmt.Errorf("presign part: %w", err)
	}
	return out.URL, nil
}

func (s *S3) PresignPut(ctx context.Context, key, contentType string, size int64) (string, map[string]string, error) {
	out, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.sources),
		Key:           aws.String(key),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	}, func(o *s3.PresignOptions) {
		o.Expires = time.Hour
	})
	if err != nil {
		return "", nil, fmt.Errorf("presign put: %w", err)
	}
	return out.URL, map[string]string{
		"Content-Type":   contentType,
		"Content-Length": strconv.FormatInt(size, 10),
	}, nil
}

func (s *S3) CompleteMultipart(ctx context.Context, key, uploadID string, parts []CompletedPart) error {
	completed := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		etag := p.ETag
		completed[i] = types.CompletedPart{ETag: aws.String(etag), PartNumber: aws.Int32(p.Number)}
	}
	_, err := s.internal.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.sources),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completed,
		},
	})
	if err != nil {
		return fmt.Errorf("complete multipart: %w", err)
	}
	return nil
}

func (s *S3) ListParts(ctx context.Context, key, uploadID string) ([]UploadedPart, error) {
	var out []UploadedPart
	var marker *string
	for {
		page, err := s.internal.ListParts(ctx, &s3.ListPartsInput{
			Bucket:           aws.String(s.sources),
			Key:              aws.String(key),
			UploadId:         aws.String(uploadID),
			PartNumberMarker: marker,
		})
		// ListParts does not model NoSuchUpload: it arrives as a generic API error.
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchUpload" {
			return nil, fmt.Errorf("list parts %s: %w", key, ErrMissing)
		}
		if err != nil {
			return nil, fmt.Errorf("list parts %s: %w", key, err)
		}
		for _, p := range page.Parts {
			out = append(out, UploadedPart{
				Number: aws.ToInt32(p.PartNumber), ETag: aws.ToString(p.ETag), Size: aws.ToInt64(p.Size),
			})
		}
		if !aws.ToBool(page.IsTruncated) || page.NextPartNumberMarker == nil {
			return out, nil
		}
		marker = page.NextPartNumberMarker
	}
}

func (s *S3) AbortMultipart(ctx context.Context, key, uploadID string) error {
	_, err := s.internal.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.sources),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	var missing *types.NoSuchUpload
	if errors.As(err, &missing) {
		return nil // already completed or aborted
	}
	if err != nil {
		return fmt.Errorf("abort multipart: %w", err)
	}
	return nil
}

func (s *S3) HeadSource(ctx context.Context, key string) (Object, error) {
	out, err := s.internal.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.sources),
		Key:    aws.String(key),
	})
	var missing *types.NotFound
	if errors.As(err, &missing) {
		return Object{}, fmt.Errorf("head %s: %w", key, ErrMissing)
	}
	if err != nil {
		return Object{}, fmt.Errorf("head %s: %w", key, err)
	}
	return Object{Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType)}, nil
}

func (s *S3) GetSourceRange(ctx context.Context, key string, start, end int64) ([]byte, error) {
	out, err := s.internal.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.sources),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
	})
	if err != nil {
		return nil, fmt.Errorf("get range %s: %w", key, err)
	}
	defer func() { _ = out.Body.Close() }()
	return io.ReadAll(io.LimitReader(out.Body, 64<<10))
}

func (s *S3) GetSource(ctx context.Context, key string, dest io.Writer) error {
	out, err := s.internal.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.sources),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get %s: %w", key, err)
	}
	defer func() { _ = out.Body.Close() }()
	_, err = io.Copy(dest, out.Body)
	return err
}

func (s *S3) PutMedia(ctx context.Context, key, contentType string, body io.Reader) error {
	_, err := s.internal.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.media),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (s *S3) DeleteSource(ctx context.Context, key string) error {
	_, err := s.internal.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.sources),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

func (s *S3) Hide(ctx context.Context, prefix string) (int, error) {
	return s.move(ctx, s.media, s.quarantine, prefix)
}

func (s *S3) Reveal(ctx context.Context, prefix string) (int, error) {
	return s.move(ctx, s.quarantine, s.media, prefix)
}

// move copies then deletes each object, so an interrupted move leaves every
// object in at least one bucket and a retry finishes it.
func (s *S3) move(ctx context.Context, from, to, prefix string) (int, error) {
	return s.moveMatching(ctx, from, to, prefix, func(string) bool { return true })
}

// MoveLegacySources moves the source objects that predate the private source
// bucket out of the media and quarantine buckets. It is idempotent and
// returns how many objects moved.
func (s *S3) MoveLegacySources(ctx context.Context) (int, error) {
	total := 0
	for _, from := range []string{s.media, s.quarantine} {
		n, err := s.moveMatching(ctx, from, s.sources, model.MediaRoot, model.IsSourceKey)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (s *S3) moveMatching(ctx context.Context, from, to, prefix string, match func(string) bool) (int, error) {
	moved := 0
	pages := s3.NewListObjectsV2Paginator(s.internal, &s3.ListObjectsV2Input{
		Bucket: aws.String(from),
		Prefix: aws.String(prefix),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return moved, fmt.Errorf("list %s/%s: %w", from, prefix, err)
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if !match(key) {
				continue
			}
			if _, err := s.internal.CopyObject(ctx, &s3.CopyObjectInput{
				Bucket:     aws.String(to),
				Key:        aws.String(key),
				CopySource: aws.String(copySource(from, key)),
			}); err != nil {
				return moved, fmt.Errorf("copy %s/%s: %w", from, key, err)
			}
			if _, err := s.internal.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(from),
				Key:    aws.String(key),
			}); err != nil {
				return moved, fmt.Errorf("delete %s/%s: %w", from, key, err)
			}
			moved++
		}
	}
	return moved, nil
}

// copySource is bucket/key with each path segment URL-encoded, as S3 expects.
func copySource(bucket, key string) string {
	parts := strings.Split(bucket+"/"+key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
