package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// Client 封装 MinIO 客户端，提供文件的上传、下载、删除操作。
type Client struct {
	mc     *minio.Client
	bucket string
}

// New 初始化 MinIO 客户端。如果存储未启用，返回 nil。
// bucket 不存在时自动创建（开发友好）。
func New(cfg config.StorageConfig, log *zap.Logger) (*Client, error) {
	if !cfg.Enabled {
		log.Info("storage client disabled")
		return nil, nil
	}

	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	ctx := context.Background()
	exists, err := mc.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		log.Info("minio bucket created", zap.String("bucket", cfg.Bucket))
	}

	log.Info("minio client connected",
		zap.String("endpoint", cfg.Endpoint),
		zap.String("bucket", cfg.Bucket),
	)
	return &Client{mc: mc, bucket: cfg.Bucket}, nil
}

// UploadFile 将 multipart.FileHeader 上传到 MinIO，tags 作为对象标签存储在 MinIO 上。
func (c *Client) UploadFile(ctx context.Context, objectKey string, fileHeader *multipart.FileHeader, tags map[string]string) (*minio.UploadInfo, error) {
	if c == nil || c.mc == nil {
		return nil, common.ErrStorageUnavailable
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer file.Close()

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	info, err := c.mc.PutObject(ctx, c.bucket, objectKey, file, fileHeader.Size,
		minio.PutObjectOptions{
			ContentType:  contentType,
			UserMetadata: tags,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to minio: %w", err)
	}
	return &info, nil
}

// GetFile 从 MinIO 获取文件流。调用方必须在读取后关闭 reader。
func (c *Client) GetFile(ctx context.Context, objectKey string) (io.ReadCloser, *minio.ObjectInfo, error) {
	if c == nil || c.mc == nil {
		return nil, nil, common.ErrStorageUnavailable
	}

	obj, err := c.mc.GetObject(ctx, c.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object from minio: %w", err)
	}

	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, fmt.Errorf("failed to stat object: %w", err)
	}

	return obj, &stat, nil
}

// DeleteFile 从 MinIO 删除指定对象。若对象不存在则静默返回 nil。
func (c *Client) DeleteFile(ctx context.Context, objectKey string) error {
	if c == nil || c.mc == nil {
		return common.ErrStorageUnavailable
	}

	err := c.mc.RemoveObject(ctx, c.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return nil
		}
		return fmt.Errorf("failed to delete from minio: %w", err)
	}
	return nil
}

// DeleteByPrefix 按前缀批量删除对象。用于校友删除时清理文件存储。
func (c *Client) DeleteByPrefix(ctx context.Context, prefix string) error {
	if c == nil || c.mc == nil {
		return common.ErrStorageUnavailable
	}

	objectsCh := make(chan minio.ObjectInfo)

	go func() {
		defer close(objectsCh)
		for obj := range c.mc.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: true,
		}) {
			if obj.Err != nil {
				continue
			}
			objectsCh <- obj
		}
	}()

	for err := range c.mc.RemoveObjects(ctx, c.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if err.Err != nil {
			return fmt.Errorf("failed to batch delete from minio: %w", err.Err)
		}
	}
	return nil
}
