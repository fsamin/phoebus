package assets

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
)

func TestFilesystemStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemStore(dir)
	if err != nil {
		t.Fatalf("NewFilesystemStore: %v", err)
	}
	ctx := context.Background()

	hash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	data := []byte("hello world image data")
	contentType := "image/png"

	// Exists — should be false initially
	exists, err := store.Exists(ctx, hash)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Fatal("expected asset to not exist")
	}

	// Put
	if err := store.Put(ctx, hash, contentType, bytes.NewReader(data)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Exists — should be true now
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

	// Put same hash again — should overwrite without error (idempotent)
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

	// Get after delete — should error
	_, _, err = store.Get(ctx, hash)
	if err == nil {
		t.Fatal("expected error on Get after Delete")
	}

	// Delete non-existent — should not error
	if err := store.Delete(ctx, hash); err != nil {
		t.Fatalf("Delete non-existent: %v", err)
	}
}

func TestFilesystemStore_EmptyDataDir(t *testing.T) {
	dir := t.TempDir()
	subDir := dir + "/nested/deep"
	store, err := NewFilesystemStore(subDir)
	if err != nil {
		t.Fatalf("NewFilesystemStore with nested dir: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(subDir); os.IsNotExist(err) {
		t.Fatal("expected directory to be created")
	}
	_ = store
}
