package storage

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// Client 封装 MinIO 客户端，提供文件的上传、下载、删除操作。
type Client struct {
	mc             *minio.Client
	bucket         string
	publicEndpoint string // 公开访问地址，生成预签名 URL 时替换 host；空则使用内部 endpoint
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
	return &Client{mc: mc, bucket: cfg.Bucket, publicEndpoint: cfg.PublicEndpoint}, nil
}

// PresignedPutURL 生成预签名上传 URL，客户端可用该 URL 直传文件到 MinIO。
// expiry 为签名有效期，建议 5-15 分钟。
// 若配置了 PublicEndpoint，URL 中的 host 会被替换为公开地址。
func (c *Client) PresignedPutURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	if c == nil || c.mc == nil {
		return "", common.ErrStorageUnavailable
	}
	u, err := c.mc.PresignedPutObject(ctx, c.bucket, objectKey, expiry)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned put url: %w", err)
	}
	return c.replaceEndpoint(u.String()), nil
}

// PresignedGetURL 生成预签名下载 URL，客户端可用该 URL 直连 MinIO 下载文件。
// expiry 为签名有效期，建议 5-15 分钟。
// 若配置了 PublicEndpoint，URL 中的 host 会被替换为公开地址。
func (c *Client) PresignedGetURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	if c == nil || c.mc == nil {
		return "", common.ErrStorageUnavailable
	}
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, objectKey, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned get url: %w", err)
	}
	return c.replaceEndpoint(u.String()), nil
}

// StatObject 获取 MinIO 对象元信息，用于确认上传确实完成。
func (c *Client) StatObject(ctx context.Context, objectKey string) (*minio.ObjectInfo, error) {
	if c == nil || c.mc == nil {
		return nil, common.ErrStorageUnavailable
	}
	info, err := c.mc.StatObject(ctx, c.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}
	return &info, nil
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

// replaceEndpoint 将 URL 中的 host 替换为公开地址。
// publicEndpoint 格式为 "host" 或 "host:port"；若不含端口则保留原 URL 的端口。
// 如果 publicEndpoint 为空或 rawURL 解析失败，返回原 URL。
func (c *Client) replaceEndpoint(rawURL string) string {
	if c.publicEndpoint == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	// 替换 host，保留原 URL 的端口（若 publicEndpoint 未指定端口）
	host := c.publicEndpoint
	if _, _, err := net.SplitHostPort(host); err != nil {
		// publicEndpoint 不含端口，保留原 URL 的端口
		if _, _, origErr := net.SplitHostPort(parsed.Host); origErr == nil {
			origHost, origPort, _ := net.SplitHostPort(parsed.Host)
			if origHost != "" {
				host = net.JoinHostPort(host, origPort)
			}
		}
	}
	parsed.Host = host
	return parsed.String()
}
