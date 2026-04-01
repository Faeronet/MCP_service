package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	client *minio.Client
	bucket string
}

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio new: %w", err)
	}
	// Ensure bucket exists
	_ = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{})
	return &Client{client: client, bucket: cfg.Bucket}, nil
}

func (c *Client) Put(ctx context.Context, objectName string, reader io.Reader, contentType string, size int64) (string, error) {
	info, err := c.client.PutObject(ctx, c.bucket, objectName, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", err
	}
	return info.ETag, nil
}

func (c *Client) Get(ctx context.Context, objectName string) (*minio.Object, error) {
	return c.client.GetObject(ctx, c.bucket, objectName, minio.GetObjectOptions{})
}

func (c *Client) Bucket() string { return c.bucket }

func (c *Client) Client() *minio.Client { return c.client }

// Remove удаляет объект из бакета. Ошибки (в т.ч. объект не найден) можно игнорировать при best-effort удалении.
func (c *Client) Remove(ctx context.Context, objectName string) error {
	return c.client.RemoveObject(ctx, c.bucket, objectName, minio.RemoveObjectOptions{})
}
