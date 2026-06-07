package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

const (
	driverLocal = "local"
	driverMinIO = "minio"
)

// Client 提供本地硬盘或 MinIO 文件存储。
type Client struct {
	driver    string
	localPath string
	mc        *minio.Client
	bucket    string
}

// New 初始化 MinIO 客户端。如果存储未启用，返回 nil。
// bucket 不存在时自动创建（开发友好）。
func New(cfg config.StorageConfig, log *zap.Logger) (*Client, error) {
	if !cfg.Enabled {
		log.Info("storage client disabled")
		return nil, nil
	}

	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		driver = driverMinIO
	}
	if driver == driverLocal {
		root, err := filepath.Abs(cfg.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("resolve local storage path: %w", err)
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			return nil, fmt.Errorf("create local storage path: %w", err)
		}
		log.Info("local storage ready", zap.String("path", root))
		return &Client{driver: driverLocal, localPath: root}, nil
	}
	if driver != driverMinIO {
		return nil, fmt.Errorf("unsupported storage driver %q", cfg.Driver)
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
	return &Client{driver: driverMinIO, mc: mc, bucket: cfg.Bucket}, nil
}

// UploadFile 将文件上传到当前存储驱动。
func (c *Client) UploadFile(ctx context.Context, objectKey string, fileHeader *multipart.FileHeader, tags map[string]string) (*UploadInfo, error) {
	if c == nil {
		return nil, common.ErrStorageUnavailable
	}

	if c.driver == driverLocal {
		source, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("open uploaded file: %w", err)
		}
		defer source.Close()

		target, err := c.localFilePath(objectKey)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, fmt.Errorf("create local storage directory: %w", err)
		}
		destination, err := os.Create(target)
		if err != nil {
			return nil, fmt.Errorf("create local storage file: %w", err)
		}
		size, copyErr := io.Copy(destination, source)
		closeErr := destination.Close()
		if copyErr != nil {
			_ = os.Remove(target)
			return nil, fmt.Errorf("write local storage file: %w", copyErr)
		}
		if closeErr != nil {
			_ = os.Remove(target)
			return nil, fmt.Errorf("close local storage file: %w", closeErr)
		}
		return &UploadInfo{Size: size}, nil
	}
	if c.mc == nil {
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
	return &UploadInfo{Size: info.Size}, nil
}

// GetFile 获取文件流。调用方必须在读取后关闭 reader。
func (c *Client) GetFile(ctx context.Context, objectKey string) (io.ReadCloser, *ObjectInfo, error) {
	if c == nil {
		return nil, nil, common.ErrStorageUnavailable
	}

	if c.driver == driverLocal {
		target, err := c.localFilePath(objectKey)
		if err != nil {
			return nil, nil, err
		}
		file, err := os.Open(target)
		if err != nil {
			return nil, nil, fmt.Errorf("open local storage file: %w", err)
		}
		stat, err := file.Stat()
		if err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("stat local storage file: %w", err)
		}
		return file, &ObjectInfo{
			Size:        stat.Size(),
			ContentType: mime.TypeByExtension(filepath.Ext(target)),
		}, nil
	}
	if c.mc == nil {
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

	return obj, &ObjectInfo{Size: stat.Size, ContentType: stat.ContentType}, nil
}

// DeleteFile 删除指定对象。若对象不存在则静默返回 nil。
func (c *Client) DeleteFile(ctx context.Context, objectKey string) error {
	if c == nil {
		return common.ErrStorageUnavailable
	}

	if c.driver == driverLocal {
		target, err := c.localFilePath(objectKey)
		if err != nil {
			return err
		}
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete local storage file: %w", err)
		}
		return nil
	}
	if c.mc == nil {
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
	if c == nil {
		return common.ErrStorageUnavailable
	}

	if c.driver == driverLocal {
		target, err := c.localFilePath(prefix)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("delete local storage prefix: %w", err)
		}
		return nil
	}
	if c.mc == nil {
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

func (c *Client) localFilePath(objectKey string) (string, error) {
	cleanKey := filepath.Clean(filepath.FromSlash(objectKey))
	if cleanKey == "." || filepath.IsAbs(cleanKey) || cleanKey == ".." ||
		strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid storage object key")
	}
	target := filepath.Join(c.localPath, cleanKey)
	relative, err := filepath.Rel(c.localPath, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage object key escapes root")
	}
	return target, nil
}
