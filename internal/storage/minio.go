package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioStore implements Store backed by MinIO / S3.
type MinioStore struct {
	client *minio.Client
	bucket string
}

// MinioConfig holds MinIO connection parameters.
type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// MinioConfigFromEnv reads MinIO config from environment variables,
// falling back to defaults matching docker-compose.yml.
func MinioConfigFromEnv() MinioConfig {
	useSSL, _ := strconv.ParseBool(os.Getenv("MINIO_USE_SSL"))
	return MinioConfig{
		Endpoint:  envOr("MINIO_ENDPOINT", "localhost:9000"),
		AccessKey: envOr("MINIO_ACCESS_KEY", "minioadmin"),
		SecretKey: envOr("MINIO_SECRET_KEY", "minioadmin"),
		Bucket:    envOr("MINIO_BUCKET", "provenn"),
		UseSSL:    useSSL,
	}
}

// NewMinioStore creates a MinIO client and returns it wrapped as a Store.
func NewMinioStore(cfg MinioConfig) (*MinioStore, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return &MinioStore{client: client, bucket: cfg.Bucket}, nil
}

func (s *MinioStore) Put(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, data, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *MinioStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
