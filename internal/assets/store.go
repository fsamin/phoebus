package assets

import (
	"context"
	"fmt"
	"io"

	"github.com/fsamin/phoebus/internal/config"
)

// Store is the interface for asset storage backends.
type Store interface {
	Put(ctx context.Context, hash string, contentType string, data io.Reader) error
	Get(ctx context.Context, hash string) (io.ReadCloser, string, error) // data, contentType, error
	Delete(ctx context.Context, hash string) error
	Exists(ctx context.Context, hash string) (bool, error)
}

// NewStore creates an asset store based on configuration.
func NewStore(cfg config.AssetsConfig) (Store, error) {
	switch cfg.Backend {
	case "filesystem", "":
		return NewFilesystemStore(cfg.Filesystem.DataDir)
	case "s3":
		return NewS3Store(cfg.S3)
	default:
		return nil, fmt.Errorf("unknown asset storage backend: %s", cfg.Backend)
	}
}
