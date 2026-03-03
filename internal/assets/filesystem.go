package assets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FilesystemStore stores assets on the local filesystem.
// Files are organized as {dataDir}/{hash[0:2]}/{hash} to avoid too many files in one directory.
// A companion .meta file stores the content type.
type FilesystemStore struct {
	dataDir string
}

func NewFilesystemStore(dataDir string) (*FilesystemStore, error) {
	if dataDir == "" {
		dataDir = "./data/assets"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create asset directory %s: %w", dataDir, err)
	}
	return &FilesystemStore{dataDir: dataDir}, nil
}

var ErrInvalidHash = fmt.Errorf("invalid asset hash")

func (fs *FilesystemStore) path(hash string) (string, error) {
	if len(hash) < 2 {
		return "", ErrInvalidHash
	}
	return filepath.Join(fs.dataDir, hash[:2], hash), nil
}

func (fs *FilesystemStore) metaPath(hash string) (string, error) {
	p, err := fs.path(hash)
	if err != nil {
		return "", err
	}
	return p + ".meta", nil
}

type fileMeta struct {
	ContentType string `json:"content_type"`
}

func (fs *FilesystemStore) Put(_ context.Context, hash string, contentType string, data io.Reader) error {
	p, err := fs.path(hash)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir for asset %s: %w", hash, err)
	}

	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create asset file %s: %w", hash, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, data); err != nil {
		os.Remove(p)
		return fmt.Errorf("write asset %s: %w", hash, err)
	}

	// Write metadata
	mp, _ := fs.metaPath(hash)
	meta, _ := json.Marshal(fileMeta{ContentType: contentType})
	if err := os.WriteFile(mp, meta, 0o644); err != nil {
		os.Remove(p)
		return fmt.Errorf("write asset meta %s: %w", hash, err)
	}

	return nil
}

func (fs *FilesystemStore) Get(_ context.Context, hash string) (io.ReadCloser, string, error) {
	p, err := fs.path(hash)
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", ErrAssetNotFound
		}
		return nil, "", fmt.Errorf("open asset %s: %w", hash, err)
	}

	mp, _ := fs.metaPath(hash)
	metaBytes, err := os.ReadFile(mp)
	if err != nil {
		f.Close()
		return nil, "", fmt.Errorf("read asset meta %s: %w", hash, err)
	}

	var m fileMeta
	if err := json.Unmarshal(metaBytes, &m); err != nil {
		f.Close()
		return nil, "", fmt.Errorf("parse asset meta %s: %w", hash, err)
	}

	return f, m.ContentType, nil
}

func (fs *FilesystemStore) Delete(_ context.Context, hash string) error {
	mp, err := fs.metaPath(hash)
	if err != nil {
		return err
	}
	os.Remove(mp)
	p, _ := fs.path(hash)
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete asset %s: %w", hash, err)
	}
	return nil
}

func (fs *FilesystemStore) Exists(_ context.Context, hash string) (bool, error) {
	p, err := fs.path(hash)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
