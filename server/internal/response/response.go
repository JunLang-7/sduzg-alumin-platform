package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CodeSuccess            = 0
	CodeBadRequest         = 40000
	CodeUnauthorized       = 40100
	CodeForbidden          = 40300
	CodeNotFound           = 40400
	CodeInternalError      = 50000
	CodeServiceUnavailable = 50300
)

type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Success 发送成功响应
// data 参数可以是任何类型，通常是一个结构体或 map，表示响应的数据内容
func Success(c *gin.Context, data any) {
	JSON(c, http.StatusOK, CodeSuccess, "success", data)
}

// Fail 发送失败响应
// httpStatus 参数表示 HTTP 状态码，
// code 参数表示业务错误码，
// message 参数表示错误信息
func Fail(c *gin.Context, httpStatus int, code int, message string) {
	JSON(c, httpStatus, code, message, nil)
}

// JSON 发送 JSON 响应
// httpStatus 参数表示 HTTP 状态码，
// code 参数表示业务错误码，
// message 参数表示响应信息，
// data 参数表示响应的数据内容，可以是任何类型
func JSON(c *gin.Context, httpStatus int, code int, message string, data any) {
	c.JSON(httpStatus, Body{
		Code:    code,
		Message: message,
		Data:    data,
	})
}
