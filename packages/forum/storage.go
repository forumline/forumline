package forum

import (
	"context"
	"io"
)

// FileStorage abstracts file uploads so the forum engine works with any
// storage backend. Hosted forums use Cloudflare R2; standalone forums
// can use local disk, S3-compatible storage, or disable uploads entirely.
type FileStorage interface {
	// Upload stores a file and returns its public URL.
	// key is the storage path (e.g. "user/abc123/avatar.png").
	// contentType is the MIME type (e.g. "image/png").
	Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) (url string, err error)

	// Delete removes a file by its storage key.
	Delete(ctx context.Context, key string) error

	// URL returns the public URL for a given storage key.
	// This is used to construct avatar URLs without re-uploading.
	URL(key string) string
}

// NoopStorage is a FileStorage that rejects all uploads.
// Use this when file uploads are disabled (e.g. a minimal standalone forum).
type NoopStorage struct{}

func (NoopStorage) Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) (string, error) {
	return "", &StorageDisabledError{}
}

func (NoopStorage) Delete(ctx context.Context, key string) error {
	return nil
}

func (NoopStorage) URL(key string) string {
	return ""
}

// StorageDisabledError is returned when upload is attempted on a NoopStorage.
type StorageDisabledError struct{}

func (e *StorageDisabledError) Error() string {
	return "file uploads are not configured"
}
