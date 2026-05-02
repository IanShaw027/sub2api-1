package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type s3MediaObjectStore struct {
	enabled bool
	client  *s3.Client
}

func NewS3MediaObjectStore(cfg *config.Config) service.MediaObjectStore {
	if cfg == nil || !cfg.Media.Enabled {
		return &s3MediaObjectStore{enabled: false}
	}

	region := strings.TrimSpace(cfg.Media.Region)
	if region == "" {
		region = "auto"
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.Media.AccessKeyID, cfg.Media.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return &s3MediaObjectStore{enabled: false}
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint := strings.TrimSpace(cfg.Media.Endpoint); endpoint != "" {
			o.BaseEndpoint = &endpoint
		}
		if cfg.Media.ForcePathStyle {
			o.UsePathStyle = true
		}
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})

	return &s3MediaObjectStore{
		enabled: true,
		client:  client,
	}
}

func (s *s3MediaObjectStore) Upload(ctx context.Context, bucket, objectKey string, body []byte, contentType string) error {
	if s == nil || !s.enabled || s.client == nil {
		return service.ErrMediaStorageDisabled
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3 put object: %w", err)
	}
	return nil
}

func (s *s3MediaObjectStore) Download(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	if s == nil || !s.enabled || s.client == nil {
		return nil, service.ErrMediaStorageDisabled
	}
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}
	return result.Body, nil
}

func (s *s3MediaObjectStore) Delete(ctx context.Context, bucket, objectKey string) error {
	if s == nil || !s.enabled || s.client == nil {
		return service.ErrMediaStorageDisabled
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}
	return nil
}

func (s *s3MediaObjectStore) Stat(ctx context.Context, bucket, objectKey string) (int64, error) {
	if s == nil || !s.enabled || s.client == nil {
		return 0, service.ErrMediaStorageDisabled
	}
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return 0, fmt.Errorf("s3 head object: %w", err)
	}
	if result.ContentLength == nil {
		return 0, nil
	}
	return *result.ContentLength, nil
}
