package storage

import "io"

// FileDownload 封装了从 MinIO 获取的文件流及其元数据，供 handler 流式输出。
type FileDownload struct {
	Reader       io.ReadCloser
	ContentType  string
	Size         int64
	OriginalName string
}
