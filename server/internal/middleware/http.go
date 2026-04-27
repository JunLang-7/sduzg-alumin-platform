package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const RequestIDHeader = "X-Request-ID"

// RequestID 从请求头中提取请求 ID，如果不存在则生成一个新的请求 ID，并将其设置到上下文和响应头中
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}

		c.Set(RequestIDHeader, requestID)
		c.Header(RequestIDHeader, requestID)
		c.Next()
	}
}

// RequestLogger 记录每个 HTTP 请求的详细信息，包括方法、路径、状态码、响应时间、客户端 IP 和请求 ID
func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
			zap.String("request_id", requestIDFromContext(c)),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		log.Info("http request", fields...)
	}
}

// Recovery 捕获处理过程中发生的 panic，记录错误信息并返回 500 内部服务器错误响应
func Recovery(log *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Error(
			"panic recovered",
			zap.Any("panic", recovered),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("request_id", requestIDFromContext(c)),
		)

		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "internal server error")
		c.Abort()
	})
}

func requestIDFromContext(c *gin.Context) string {
	value, exists := c.Get(RequestIDHeader)
	if !exists {
		return ""
	}

	requestID, ok := value.(string)
	if !ok {
		return ""
	}
	return requestID
}
