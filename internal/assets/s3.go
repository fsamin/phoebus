package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/fsamin/phoebus/internal/config"
)

// S3Store stores assets in an S3-compatible bucket.
type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3Store(cfg config.S3StoreConfig) (*S3Store, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = cfg.ForcePathStyle
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &S3Store{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

func (s *S3Store) key(hash string) string {
	if s.prefix != "" {
		return path.Join(s.prefix, hash)
	}
	return hash
}

func (s *S3Store) metaKey(hash string) string {
	return s.key(hash) + ".meta"
}

func (s *S3Store) Put(ctx context.Context, hash string, contentType string, data io.Reader) error {
	// Read all data into memory for S3 (needs content length)
	buf, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("read asset data: %w", err)
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.key(hash)),
		Body:        bytes.NewReader(buf),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put asset %s to S3: %w", hash, err)
	}

	// Store metadata
	meta, _ := json.Marshal(map[string]string{"content_type": contentType})
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.metaKey(hash)),
		Body:        bytes.NewReader(meta),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("put asset meta %s to S3: %w", hash, err)
	}

	return nil
}

func (s *S3Store) Get(ctx context.Context, hash string) (io.ReadCloser, string, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(hash)),
	})
	if err != nil {
		var nsk *s3types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, "", ErrAssetNotFound
		}
		return nil, "", fmt.Errorf("get asset %s from S3: %w", hash, err)
	}

	ct := "application/octet-stream"
	if result.ContentType != nil {
		ct = *result.ContentType
	}
	return result.Body, ct, nil
}

func (s *S3Store) Delete(ctx context.Context, hash string) error {
	s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.metaKey(hash)),
	})
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(hash)),
	})
	if err != nil {
		return fmt.Errorf("delete asset %s from S3: %w", hash, err)
	}
	return nil
}

func (s *S3Store) Exists(ctx context.Context, hash string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(hash)),
	})
	if err != nil {
		// Check if it's a "not found" error
		return false, nil
	}
	return true, nil
}
