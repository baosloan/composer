package errorx

import (
	"errors"
	"fmt"
	"net/http"
)

// Error 是一个业务错误，包含数字错误码、HTTP 状态码、面向客户端的错误信息以及可选的内层错误（wrapped cause）。
//
// 内层错误仅用于日志记录，绝不会序列化后返回给客户端。
type Error struct {
	// Code 是稳定的业务错误码，客户端可据此进行分支处理。
	Code int
	// Status 是该错误对应的 HTTP 状态码。
	Status int
	// Message 是面向客户端的错误描述，不得包含基础设施相关的细节信息。
	Message string
	// Err 是底层错误原因，可用于 errors.Is/As 进行错误链判断，但绝不会返回给客户端。
	Err error
}

// New 构建一个 *Error。推荐在 code.go 中声明包级别的错误，而非在出错现场调用 New，
// 以便错误码保持集中可发现。
func New(code, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

// Error 实现 error 接口。由于该字符串最终会出现在日志中，因此包含底层错误原因；
// 而响应写入器（response writer）应使用 Message 字段。
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 暴露底层错误原因，供 errors.Is 和 errors.As 使用。
func (e *Error) Unwrap() error { return e.Err }

// Is 使 errors.Is 能够按业务错误码进行匹配，
// 因此即使经过包装，错误副本仍能与原始包级哨兵错误相等。
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// Wrap 返回 e 的一个副本，并附加底层错误原因，而不会修改原始的哨兵错误。
// 请始终使用此方法，而非直接修改包级错误变量。
func (e *Error) Wrap(cause error) *Error {
	return &Error{Code: e.Code, Status: e.Status, Message: e.Message, Err: cause}
}

// WithMessage 返回 e 的一个副本，并使用不同的面向客户端的错误信息。
func (e *Error) WithMessage(message string) *Error {
	return &Error{Code: e.Code, Status: e.Status, Message: message, Err: e.Err}
}

// WithMessagef 是 WithMessage 的格式化版本。
func (e *Error) WithMessagef(format string, args ...any) *Error {
	return e.WithMessage(fmt.Sprintf(format, args...))
}

// From 从 err 中提取 *Error。
//
// 未知错误会被统一规约为 ErrInternal，并将原始错误作为底层原因包装：
// 调用方获得安全的客户端消息，而真实错误仍保留在日志中。
func From(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := errors.AsType[*Error](err); ok {
		return e
	}
	return ErrInternal.Wrap(err)
}

// IsInternal 判断 err 是否表示服务端故障（5xx 状态码），
// 调用方据此决定是否以 error 级别记录日志。
func IsInternal(err error) bool {
	e := From(err)
	return e != nil && e.Status >= http.StatusInternalServerError
}
