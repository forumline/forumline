package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// R2Storage implements forum.FileStorage using Cloudflare R2.
type R2Storage struct {
	AccountID string
	KeyID     string
	Secret    string
	Bucket    string
	PublicURL string
}

func (s *R2Storage) client() (*minio.Client, error) {
	endpoint := fmt.Sprintf("%s.r2.cloudflarestorage.com", s.AccountID)
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s.KeyID, s.Secret, ""),
		Secure: true,
	})
}

func (s *R2Storage) Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) (string, error) {
	c, err := s.client()
	if err != nil {
		return "", fmt.Errorf("r2 client: %w", err)
	}

	_, err = c.PutObject(ctx, s.Bucket, key, data, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("r2 upload: %w", err)
	}

	return s.URL(key), nil
}

func (s *R2Storage) Delete(ctx context.Context, key string) error {
	c, err := s.client()
	if err != nil {
		return fmt.Errorf("r2 client: %w", err)
	}

	return c.RemoveObject(ctx, s.Bucket, key, minio.RemoveObjectOptions{})
}

func (s *R2Storage) URL(key string) string {
	return fmt.Sprintf("%s/%s", strings.TrimRight(s.PublicURL, "/"), key)
}
