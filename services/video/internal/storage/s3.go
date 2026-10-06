package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/poro/video/internal/config"
)

// S3 talks to a path-style S3-compatible store (SeaweedFS in development).
type S3 struct {
	internal *s3.Client
	presign  *s3.PresignClient
	bucket   string
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
		internal: internal,
		presign:  s3.NewPresignClient(public),
		bucket:   cfg.S3Bucket,
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
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("create multipart: %w", err)
	}
	return aws.ToString(out.UploadId), nil
}

func (s *S3) PresignPart(ctx context.Context, key, uploadID string, part int32) (string, error) {
	out, err := s.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(part),
	}, func(o *s3.PresignOptions) {
		o.Expires = time.Hour
	})
	if err != nil {
		return "", fmt.Errorf("presign part: %w", err)
	}
	return out.URL, nil
}

func (s *S3) CompleteMultipart(ctx context.Context, key, uploadID string, parts []CompletedPart) error {
	completed := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		etag := p.ETag
		completed[i] = types.CompletedPart{ETag: aws.String(etag), PartNumber: aws.Int32(p.Number)}
	}
	_, err := s.internal.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
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

func (s *S3) AbortMultipart(ctx context.Context, key, uploadID string) error {
	_, err := s.internal.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return fmt.Errorf("abort multipart: %w", err)
	}
	return nil
}

func (s *S3) Head(ctx context.Context, key string) (Object, error) {
	out, err := s.internal.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Object{}, fmt.Errorf("head %s: %w", key, err)
	}
	return Object{Size: aws.ToInt64(out.ContentLength)}, nil
}

func (s *S3) GetRange(ctx context.Context, key string, start, end int64) ([]byte, error) {
	out, err := s.internal.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
	})
	if err != nil {
		return nil, fmt.Errorf("get range %s: %w", key, err)
	}
	defer func() { _ = out.Body.Close() }()
	return io.ReadAll(io.LimitReader(out.Body, 64<<10))
}

func (s *S3) Get(ctx context.Context, key string, dest io.Writer) error {
	out, err := s.internal.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get %s: %w", key, err)
	}
	defer func() { _ = out.Body.Close() }()
	_, err = io.Copy(dest, out.Body)
	return err
}

func (s *S3) Put(ctx context.Context, key, contentType string, body io.Reader) error {
	_, err := s.internal.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.internal.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}
