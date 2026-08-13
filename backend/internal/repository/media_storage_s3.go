package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// S3MediaStore implements service.MediaObjectStore. The bucket is treated as private:
// PutObject never sets a public ACL and always requests SSE-S3 AES256.
type S3MediaStore struct {
	client *s3.Client
	bucket string
}

func NewS3MediaStoreFactory() service.MediaObjectStoreFactory {
	return func(ctx context.Context, cfg *service.BackupS3Config) (service.MediaObjectStore, error) {
		if cfg == nil || !cfg.IsConfigured() {
			return nil, service.ErrMediaStorageNotConfigured
		}
		client, err := newS3Client(ctx, s3ClientParams{
			Endpoint:        cfg.Endpoint,
			Region:          cfg.Region,
			AccessKeyID:     cfg.AccessKeyID,
			SecretAccessKey: cfg.SecretAccessKey,
			ForcePathStyle:  cfg.ForcePathStyle,
		})
		if err != nil {
			return nil, err
		}
		return &S3MediaStore{client: client, bucket: cfg.Bucket}, nil
	}
}

func buildMediaPutObjectInput(bucket, key, contentType string, data []byte) *s3.PutObjectInput {
	sse := types.ServerSideEncryptionAes256
	return &s3.PutObjectInput{
		Bucket:               &bucket,
		Key:                  &key,
		Body:                 bytes.NewReader(data),
		ContentType:          &contentType,
		ServerSideEncryption: sse,
	}
}

func (s *S3MediaStore) Put(ctx context.Context, key, contentType string, data []byte) error {
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.PutObject(ctx, buildMediaPutObjectInput(s.bucket, key, contentType, data))
	finish()
	if err != nil {
		return fmt.Errorf("S3 PutObject: %w", err)
	}
	return nil
}

func (s *S3MediaStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	finish := servertiming.ObserveDependency(ctx, "s3")
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	finish()
	if err != nil {
		return nil, fmt.Errorf("S3 GetObject: %w", err)
	}
	return result.Body, nil
}

func (s *S3MediaStore) Delete(ctx context.Context, key string) error {
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	finish()
	if err != nil {
		return fmt.Errorf("S3 DeleteObject: %w", err)
	}
	return nil
}

func (s *S3MediaStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = service.DefaultMediaTTLMinutes * time.Minute
	}
	if ttl > time.Duration(service.MaxMediaTTLMinutes)*time.Minute {
		ttl = time.Duration(service.MaxMediaTTLMinutes) * time.Minute
	}
	presignClient := s3.NewPresignClient(s.client)
	result, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign url: %w", err)
	}
	return result.URL, nil
}
