package storage

import "io"

type UploadInfo struct {
	Size int64
}

type ObjectInfo struct {
	Size        int64
	ContentType string
}

// FileDownload 封装文件流及其元数据，供 handler 流式输出。
type FileDownload struct {
	Reader       io.ReadCloser
	ContentType  string
	Size         int64
	OriginalName string
}
