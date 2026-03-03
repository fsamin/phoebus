package assets

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/fsamin/phoebus/internal/config"
)

// These tests require a running MinIO/S3-compatible endpoint.
// Set PHOEBUS_TEST_S3_ENDPOINT to enable them (e.g. http://localhost:9000).
// Default credentials: minioadmin/minioadmin, bucket: phoebus-test-assets

func s3TestConfig(t *testing.T) config.S3StoreConfig {
	endpoint := os.Getenv("PHOEBUS_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("PHOEBUS_TEST_S3_ENDPOINT not set — skipping S3 tests")
	}
	return config.S3StoreConfig{
		Bucket:         "phoebus-test-assets",
		Region:         "us-east-1",
		Endpoint:       endpoint,
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
		ForcePathStyle: true,
	}
}

func TestS3Store(t *testing.T) {
	cfg := s3TestConfig(t)
	store, err := NewS3Store(cfg)
	if err != nil {
		t.Fatalf("NewS3Store: %v", err)
	}
	ctx := context.Background()

	hash := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	data := []byte("hello S3 asset data")
	contentType := "image/png"

	// Exists — should be false initially
	exists, err := store.Exists(ctx, hash)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Fatal("expected asset to not exist initially")
	}

	// Put
	if err := store.Put(ctx, hash, contentType, bytes.NewReader(data)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	t.Cleanup(func() { store.Delete(ctx, hash) })

	// Exists — should be true
	exists, err = store.Exists(ctx, hash)
	if err != nil {
		t.Fatalf("Exists after Put: %v", err)
	}
	if !exists {
		t.Fatal("expected asset to exist after Put")
	}

	// Get
	reader, ct, err := store.Get(ctx, hash)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer reader.Close()

	if ct != contentType {
		t.Fatalf("content type mismatch: got %q, want %q", ct, contentType)
	}

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("data mismatch: got %q, want %q", got, data)
	}

	// Put same hash again (idempotent)
	if err := store.Put(ctx, hash, contentType, bytes.NewReader(data)); err != nil {
		t.Fatalf("Put duplicate: %v", err)
	}

	// Delete
	if err := store.Delete(ctx, hash); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Exists — should be false after delete
	exists, err = store.Exists(ctx, hash)
	if err != nil {
		t.Fatalf("Exists after Delete: %v", err)
	}
	if exists {
		t.Fatal("expected asset to not exist after Delete")
	}
}

func TestS3Store_MultipleAssets(t *testing.T) {
	cfg := s3TestConfig(t)
	store, err := NewS3Store(cfg)
	if err != nil {
		t.Fatalf("NewS3Store: %v", err)
	}
	ctx := context.Background()

	assets := []struct {
		hash        string
		data        []byte
		contentType string
	}{
		{"aaaa000000000000000000000000000000000000000000000000000000000001", []byte("PNG image data"), "image/png"},
		{"aaaa000000000000000000000000000000000000000000000000000000000002", []byte("JPEG image data"), "image/jpeg"},
		{"aaaa000000000000000000000000000000000000000000000000000000000003", []byte("MP4 video data"), "video/mp4"},
	}

	for _, a := range assets {
		if err := store.Put(ctx, a.hash, a.contentType, bytes.NewReader(a.data)); err != nil {
			t.Fatalf("Put %s: %v", a.hash, err)
		}
		t.Cleanup(func() { store.Delete(ctx, a.hash) })
	}

	// Verify all exist and have correct content
	for _, a := range assets {
		exists, err := store.Exists(ctx, a.hash)
		if err != nil {
			t.Fatalf("Exists %s: %v", a.hash, err)
		}
		if !exists {
			t.Fatalf("expected %s to exist", a.hash)
		}

		reader, ct, err := store.Get(ctx, a.hash)
		if err != nil {
			t.Fatalf("Get %s: %v", a.hash, err)
		}
		got, _ := io.ReadAll(reader)
		reader.Close()

		if ct != a.contentType {
			t.Errorf("content type for %s: got %q, want %q", a.hash, ct, a.contentType)
		}
		if !bytes.Equal(got, a.data) {
			t.Errorf("data for %s: got %q, want %q", a.hash, got, a.data)
		}
	}
}

func TestS3Store_WithPrefix(t *testing.T) {
	cfg := s3TestConfig(t)
	cfg.Prefix = "test-prefix"
	store, err := NewS3Store(cfg)
	if err != nil {
		t.Fatalf("NewS3Store with prefix: %v", err)
	}
	ctx := context.Background()

	hash := "bbbb000000000000000000000000000000000000000000000000000000000001"
	data := []byte("prefixed asset data")

	if err := store.Put(ctx, hash, "text/plain", bytes.NewReader(data)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	t.Cleanup(func() { store.Delete(ctx, hash) })

	exists, err := store.Exists(ctx, hash)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("expected asset with prefix to exist")
	}

	reader, _, err := store.Get(ctx, hash)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(reader)
	reader.Close()
	if !bytes.Equal(got, data) {
		t.Fatalf("data mismatch with prefix")
	}
}
