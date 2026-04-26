package errors

import (
  "fmt"
  "net/http"
  "strings"
)

// ErrorCode 错误码
type ErrorCode string

const (
  // 通用错误
  ErrInternalServer   ErrorCode = "INTERNAL_SERVER_ERROR"
  ErrBadRequest       ErrorCode = "BAD_REQUEST"
  ErrNotFound         ErrorCode = "NOT_FOUND"
  ErrUnauthorized     ErrorCode = "UNAUTHORIZED"
  ErrForbidden        ErrorCode = "FORBIDDEN"
  ErrValidationFailed ErrorCode = "VALIDATION_FAILED"
  
  // 文件相关错误
  ErrFileNotFound     ErrorCode = "FILE_NOT_FOUND"
  ErrFileUploadFailed ErrorCode = "FILE_UPLOAD_FAILED"
  ErrFileProcessing   ErrorCode = "FILE_PROCESSING_ERROR"
  ErrFileDeleteFailed ErrorCode = "FILE_DELETE_FAILED"
  
  // 数据库相关错误
  ErrDatabase         ErrorCode = "DATABASE_ERROR"
  ErrRecordNotFound   ErrorCode = "RECORD_NOT_FOUND"
  ErrDuplicateRecord  ErrorCode = "DUPLICATE_RECORD"
  
  // 处理相关错误
  ErrProcessingFailed ErrorCode = "PROCESSING_FAILED"
  ErrStepFailed       ErrorCode = "STEP_FAILED"
  ErrInvalidStep      ErrorCode = "INVALID_STEP"
  
  // 认证相关错误
  ErrInvalidToken     ErrorCode = "INVALID_TOKEN"
  ErrTokenExpired     ErrorCode = "TOKEN_EXPIRED"
  ErrMissingAuth      ErrorCode = "MISSING_AUTH"
)

// AppError 应用错误
type AppError struct {
  Code       ErrorCode `json:"code"`
  Message    string    `json:"message"`
  Details    string    `json:"details,omitempty"`
  StatusCode int       `json:"-"`
  Cause      error     `json:"-"`
}

// Error 实现error接口
func (e *AppError) Error() string {
  if e.Details != "" {
    return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
  }
  return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 实现错误链
func (e *AppError) Unwrap() error {
  return e.Cause
}

// New 创建新的应用错误
func New(code ErrorCode, message string) *AppError {
  return &AppError{
    Code:       code,
    Message:    message,
    StatusCode: getStatusCode(code),
  }
}

// NewWithDetails 创建带详细信息的应用错误
func NewWithDetails(code ErrorCode, message, details string) *AppError {
  return &AppError{
    Code:       code,
    Message:    message,
    Details:    details,
    StatusCode: getStatusCode(code),
  }
}

// Wrap 包装错误
func Wrap(err error, code ErrorCode, message string) *AppError {
  return &AppError{
    Code:       code,
    Message:    message,
    StatusCode: getStatusCode(code),
    Cause:      err,
  }
}

// WrapWithDetails 包装错误并添加详细信息
func WrapWithDetails(err error, code ErrorCode, message, details string) *AppError {
  return &AppError{
    Code:       code,
    Message:    message,
    Details:    details,
    StatusCode: getStatusCode(code),
    Cause:      err,
  }
}

// IsAppError 检查是否为应用错误
func IsAppError(err error) bool {
  _, ok := err.(*AppError)
  return ok
}

// ToAppError 转换为应用错误
func ToAppError(err error) *AppError {
  if appErr, ok := err.(*AppError); ok {
    return appErr
  }
  
  // 根据错误类型创建相应的应用错误
  if strings.Contains(err.Error(), "not found") {
    return Wrap(err, ErrNotFound, "资源未找到")
  }
  
  if strings.Contains(err.Error(), "duplicate") {
    return Wrap(err, ErrDuplicateRecord, "记录已存在")
  }
  
  if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "unauthorized") {
    return Wrap(err, ErrUnauthorized, "未授权访问")
  }
  
  if strings.Contains(err.Error(), "validation") {
    return Wrap(err, ErrValidationFailed, "验证失败")
  }
  
  return Wrap(err, ErrInternalServer, "内部服务器错误")
}

// getStatusCode 根据错误码获取HTTP状态码
func getStatusCode(code ErrorCode) int {
  switch code {
  case ErrBadRequest, ErrValidationFailed:
    return http.StatusBadRequest
  case ErrUnauthorized:
    return http.StatusUnauthorized
  case ErrForbidden:
    return http.StatusForbidden
  case ErrNotFound, ErrFileNotFound, ErrRecordNotFound:
    return http.StatusNotFound
  case ErrDuplicateRecord:
    return http.StatusConflict
  default:
    return http.StatusInternalServerError
  }
}

// ErrorResponse 错误响应
type ErrorResponse struct {
  Success bool       `json:"success"`
  Error   *AppError  `json:"error"`
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(err error) *ErrorResponse {
  appErr := ToAppError(err)
  return &ErrorResponse{
    Success: false,
    Error:   appErr,
  }
}

// ValidationError 验证错误
type ValidationError struct {
  Field   string `json:"field"`
  Message string `json:"message"`
}

// ValidationErrors 验证错误集合
type ValidationErrors []ValidationError

// Error 实现error接口
func (v ValidationErrors) Error() string {
  var messages []string
  for _, err := range v {
    messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
  }
  return strings.Join(messages, "; ")
}

// ToAppError 转换为应用错误
func (v ValidationErrors) ToAppError() *AppError {
  return NewWithDetails(ErrValidationFailed, "验证失败", v.Error())
}

// Common errors 常用错误
var (
  ErrFileNotFoundError     = New(ErrFileNotFound, "文件未找到")
  ErrInvalidTokenError     = New(ErrInvalidToken, "无效的认证令牌")
  ErrMissingAuthError      = New(ErrMissingAuth, "缺少认证信息")
  ErrUnauthorizedError     = New(ErrUnauthorized, "未授权访问")
  ErrInternalServerError   = New(ErrInternalServer, "内部服务器错误")
  ErrBadRequestError       = New(ErrBadRequest, "请求参数错误")
  ErrDatabaseError         = New(ErrDatabase, "数据库操作失败")
  ErrProcessingFailedError = New(ErrProcessingFailed, "文件处理失败")
)