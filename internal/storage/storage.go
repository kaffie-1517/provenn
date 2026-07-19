// Package storage provides an interface and MinIO/S3 implementation for file storage.
package storage

import (
	"context"
	"io"
)

// Store abstracts object storage operations (MinIO/S3).
type Store interface {
	// Put uploads data under the given key.
	Put(ctx context.Context, key string, data io.Reader, size int64, contentType string) error
	// Get retrieves the object at key. Caller must close the returned reader.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
}
