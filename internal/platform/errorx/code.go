package errorx

import "net/http"

// 业务错误码按领域分组，以便仅凭错误码即可识别其所属模块：
//
//	10xxx  通用 / 传输层（transport）
//	11xxx  基础设施（数据库、缓存、消息队列）
//	20xxx  认证（auth）
//	21xxx  用户（user）
//
// 每个错误自包含其对应的 HTTP 状态码；参见 Error.Status。
var (
	// 通用
	ErrInternal        = New(10000, http.StatusInternalServerError, "internal server error")
	ErrBadRequest      = New(10001, http.StatusBadRequest, "bad request")
	ErrValidation      = New(10002, http.StatusBadRequest, "validation failed")
	ErrUnauthorized    = New(10003, http.StatusUnauthorized, "unauthorized")
	ErrForbidden       = New(10004, http.StatusForbidden, "forbidden")
	ErrNotFound        = New(10005, http.StatusNotFound, "resource not found")
	ErrConflict        = New(10006, http.StatusConflict, "resource conflict")
	ErrTooManyReqs     = New(10007, http.StatusTooManyRequests, "too many requests")
	ErrTimeout         = New(10008, http.StatusGatewayTimeout, "request timeout")
	ErrPayloadTooLarge = New(10009, http.StatusRequestEntityTooLarge, "payload too large")
	ErrUnavailable     = New(10010, http.StatusServiceUnavailable, "service unavailable")

	// 基础设施错误。这些错误由平台包返回，
	// 以便在底层服务出现故障时，处理器仍能生成统一的响应。
	ErrDatabase = New(11001, http.StatusInternalServerError, "database error")
	ErrCache    = New(11002, http.StatusInternalServerError, "cache error")
	ErrMQ       = New(11003, http.StatusInternalServerError, "message queue error")

	// 认证
	ErrInvalidCredential = New(20001, http.StatusUnauthorized, "invalid username or password")
	ErrTokenExpired      = New(20002, http.StatusUnauthorized, "token expired")
	ErrTokenInvalid      = New(20003, http.StatusUnauthorized, "token invalid")
	ErrTokenRevoked      = New(20004, http.StatusUnauthorized, "token revoked")
	ErrAccountDisabled   = New(20005, http.StatusForbidden, "account disabled")

	// 用户
	ErrUserNotFound      = New(21001, http.StatusNotFound, "user not found")
	ErrUserAlreadyExists = New(21002, http.StatusConflict, "user already exists")
	ErrEmailAlreadyUsed  = New(21003, http.StatusConflict, "email already in use")
)
