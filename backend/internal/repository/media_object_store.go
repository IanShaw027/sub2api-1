package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type s3MediaObjectStore struct {
	mu          sync.Mutex
	client      *s3.Client
	clientCache string
}

func NewS3MediaObjectStore() service.MediaObjectStore {
	return &s3MediaObjectStore{}
}

func (s *s3MediaObjectStore) Upload(ctx context.Context, cfg service.MediaStorageRuntimeConfig, bucket, objectKey string, body []byte, contentType string) error {
	client, err := s.clientFor(ctx, cfg)
	if err != nil {
		return err
	}
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
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

func (s *s3MediaObjectStore) Download(ctx context.Context, cfg service.MediaStorageRuntimeConfig, bucket, objectKey string) (io.ReadCloser, error) {
	client, err := s.clientFor(ctx, cfg)
	if err != nil {
		return nil, err
	}
	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}
	return result.Body, nil
}

func (s *s3MediaObjectStore) Delete(ctx context.Context, cfg service.MediaStorageRuntimeConfig, bucket, objectKey string) error {
	client, err := s.clientFor(ctx, cfg)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("s3 delete object: %w", err)
	}
	return nil
}

func (s *s3MediaObjectStore) Stat(ctx context.Context, cfg service.MediaStorageRuntimeConfig, bucket, objectKey string) (int64, error) {
	client, err := s.clientFor(ctx, cfg)
	if err != nil {
		return 0, err
	}
	result, err := client.HeadObject(ctx, &s3.HeadObjectInput{
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

func (s *s3MediaObjectStore) clientFor(ctx context.Context, cfg service.MediaStorageRuntimeConfig) (*s3.Client, error) {
	if s == nil || !cfg.Enabled {
		return nil, service.ErrMediaStorageDisabled
	}

	cacheKey := strings.Join([]string{
		cfg.ProfileID,
		cfg.Endpoint,
		cfg.Region,
		cfg.AccessKeyID,
		cfg.SecretAccessKey,
		cfg.Bucket,
		fmt.Sprintf("%t", cfg.ForcePathStyle),
	}, "\x00")

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil && s.clientCache == cacheKey {
		return s.client, nil
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
			o.BaseEndpoint = &endpoint
		}
		if cfg.ForcePathStyle {
			o.UsePathStyle = true
		}
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})

	s.client = client
	s.clientCache = cacheKey
	return s.client, nil
}
