package httpx

import (
	"composer/internal/platform/errorx"
	"composer/internal/platform/logx"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Response 是统一的响应包装结构。
//
// Data 字段没有 omitempty 标签：该键始终存在（错误时为 null），
// 因此客户端可以解构响应，而无需检查字段是否存在。
// 如果在这里使用 omitempty，空载荷时会静默删除该键，从而破坏那些信任此契约的客户端。
type Response struct {
	// Code 成功时为 0，否则为 error.go 中的业务错误码。
	Code int `json:"code"`
	// Message 是人类可读的信息，可以安全地展示给用户。
	Message string `json:"message"`
	// Data 是响应载荷，错误时为 null。
	Data any `json:"data"`
	// RequestID 回显关联 ID，便于用户在提交 bug 报告时引用。
	RequestID string `json:"request_id,omitempty"`
}

const msgSuccess = "success"

func OK(c *gin.Context, data any) { Success(c, data) }

// Success writes 200 with data.
func Success(c *gin.Context, data any) { write(c, http.StatusOK, 0, msgSuccess, data) }

// Created writes 201 with data.
func Created(c *gin.Context, data any) { write(c, http.StatusCreated, 0, msgSuccess, data) }

// write emits a success envelope.
func write(c *gin.Context, status, code int, message string, data any) {
	c.JSON(status, Response{
		Code:      code,
		Message:   message,
		Data:      data,
		RequestID: logx.RequestID(c.Request.Context()),
	})
}

// NoContent writes 204 with no body, for successful deletes.
func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }

// Error writes err as a uniform failure response.
//
// It also decides what gets logged: server-side faults (5xx) are logged at
// error level with the wrapped cause, because that cause is deliberately kept
// out of the response. Client faults (4xx) are logged at debug level — they are
// the caller's problem, not an incident.
func Error(c *gin.Context, err error) {
	e := errorx.From(err)
	if e == nil {
		Success(c, nil)
		return
	}

	ctx := c.Request.Context()
	fields := []zap.Field{
		zap.Int("code", e.Code),
		zap.Int("status", e.Status),
		zap.String("path", c.Request.URL.Path),
	}
	if e.Err != nil {
		fields = append(fields, zap.Error(e.Err))
	}

	if e.Status >= http.StatusInternalServerError {
		logx.From(ctx).Error("request_failed", fields...)
	} else {
		logx.From(ctx).Debug("request_rejected", fields...)
	}

	// AbortWithStatusJSON, unlike the success path, stops the chain so no later
	// handler can append a second body to a failed request.
	c.AbortWithStatusJSON(e.Status, Response{
		Code:      e.Code,
		Message:   e.Message,
		Data:      nil,
		RequestID: logx.RequestID(ctx),
	})
}

// Fail writes a specific business error, overriding its message. Use it when
// the sentinel's generic message would hide useful detail, e.g. which field
// failed validation.
func Fail(c *gin.Context, e *errorx.Error, message string) {
	if message == "" {
		Error(c, e)
		return
	}
	Error(c, e.WithMessage(message))
}
