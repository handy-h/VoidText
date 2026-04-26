package test

import (
  "encoding/json"
  "errors"
  "net/http"
  "testing"
  
  "github.com/gin-gonic/gin"
  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
  
  "voidtext/internal/errors"
)

func TestAppError(t *testing.T) {
  t.Run("创建AppError", func(t *testing.T) {
    err := errors.NewAppError("测试错误", errors.ErrCodeInternal, http.StatusInternalServerError)
    
    assert.Error(t, err)
    assert.Equal(t, "测试错误", err.Message)
    assert.Equal(t, errors.ErrCodeInternal, err.Code)
    assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
    assert.NotEmpty(t, err.Timestamp)
  })
  
  t.Run("AppError实现error接口", func(t *testing.T) {
    err := errors.NewAppError("测试错误", errors.ErrCodeInternal, http.StatusInternalServerError)
    
    errorStr := err.Error()
    assert.Contains(t, errorStr, "测试错误")
    assert.Contains(t, errorStr, string(errors.ErrCodeInternal))
    assert.Contains(t, errorStr, "500")
  })
  
  t.Run("错误代码常量", func(t *testing.T) {
    testCases := []struct {
      name     string
      code     errors.ErrorCode
      expected string
    }{
      {"内部错误", errors.ErrCodeInternal, "INTERNAL_ERROR"},
      {"未找到", errors.ErrCodeNotFound, "NOT_FOUND"},
      {"无效输入", errors.ErrCodeInvalidInput, "INVALID_INPUT"},
      {"未授权", errors.ErrCodeUnauthorized, "UNAUTHORIZED"},
      {"禁止访问", errors.ErrCodeForbidden, "FORBIDDEN"},
      {"冲突", errors.ErrCodeConflict, "CONFLICT"},
      {"服务不可用", errors.ErrCodeServiceUnavailable, "SERVICE_UNAVAILABLE"},
      {"超时", errors.ErrCodeTimeout, "TIMEOUT"},
      {"数据库错误", errors.ErrCodeDatabase, "DATABASE_ERROR"},
      {"文件错误", errors.ErrCodeFile, "FILE_ERROR"},
      {"网络错误", errors.ErrCodeNetwork, "NETWORK_ERROR"},
      {"配置错误", errors.ErrCodeConfig, "CONFIG_ERROR"},
      {"验证错误", errors.ErrCodeValidation, "VALIDATION_ERROR"},
      {"处理错误", errors.ErrCodeProcessing, "PROCESSING_ERROR"},
    }
    
    for _, tc := range testCases {
      t.Run(tc.name, func(t *testing.T) {
        assert.Equal(t, tc.expected, string(tc.code))
      })
    }
  })
  
  t.Run("ToAppError转换", func(t *testing.T) {
    t.Run("标准错误", func(t *testing.T) {
      stdErr := errors.New("标准错误")
      appErr := errors.ToAppError(stdErr)
      
      assert.Equal(t, "标准错误", appErr.Message)
      assert.Equal(t, errors.ErrCodeInternal, appErr.Code)
      assert.Equal(t, http.StatusInternalServerError, appErr.StatusCode)
    })
    
    t.Run("AppError保持不变", func(t *testing.T) {
      originalErr := errors.NewAppError("原始错误", errors.ErrCodeNotFound, http.StatusNotFound)
      convertedErr := errors.ToAppError(originalErr)
      
      assert.Equal(t, originalErr, convertedErr)
    })
    
    t.Run("nil错误", func(t *testing.T) {
      appErr := errors.ToAppError(nil)
      assert.Nil(t, appErr)
    })
  })
  
  t.Run("错误响应", func(t *testing.T) {
    err := errors.NewAppError("测试错误", errors.ErrCodeInvalidInput, http.StatusBadRequest)
    response := errors.ErrorResponse(err)
    
    assert.Equal(t, false, response["success"])
    assert.Equal(t, "测试错误", response["error"])
    assert.Equal(t, string(errors.ErrCodeInvalidInput), response["code"])
    assert.Equal(t, http.StatusBadRequest, response["status"])
    assert.Contains(t, response, "timestamp")
  })
  
  t.Run("错误包装", func(t *testing.T) {
    t.Run("包装标准错误", func(t *testing.T) {
      stdErr := errors.New("底层错误")
      appErr := errors.Wrap(stdErr, "包装错误", errors.ErrCodeDatabase, http.StatusInternalServerError)
      
      assert.Error(t, appErr)
      assert.Equal(t, "包装错误: 底层错误", appErr.Message)
      assert.Equal(t, errors.ErrCodeDatabase, appErr.Code)
      assert.Equal(t, http.StatusInternalServerError, appErr.StatusCode)
    })
    
    t.Run("包装AppError", func(t *testing.T) {
      originalErr := errors.NewAppError("原始错误", errors.ErrCodeNotFound, http.StatusNotFound)
      wrappedErr := errors.Wrap(originalErr, "包装错误", errors.ErrCodeInternal, http.StatusInternalServerError)
      
      assert.Error(t, wrappedErr)
      assert.Equal(t, "包装错误: 原始错误", wrappedErr.Message)
      // 包装会覆盖原始错误的代码和状态码
      assert.Equal(t, errors.ErrCodeInternal, wrappedErr.Code)
      assert.Equal(t, http.StatusInternalServerError, wrappedErr.StatusCode)
    })
    
    t.Run("包装nil错误", func(t *testing.T) {
      wrappedErr := errors.Wrap(nil, "包装错误", errors.ErrCodeInternal, http.StatusInternalServerError)
      assert.Nil(t, wrappedErr)
    })
  })
  
  t.Run("错误链", func(t *testing.T) {
    err1 := errors.New("错误1")
    err2 := errors.Wrap(err1, "错误2", errors.ErrCodeDatabase, http.StatusInternalServerError)
    err3 := errors.Wrap(err2, "错误3", errors.ErrCodeProcessing, http.StatusBadRequest)
    
    assert.Error(t, err3)
    assert.Equal(t, "错误3: 错误2: 错误1", err3.Message)
    assert.Equal(t, errors.ErrCodeProcessing, err3.Code)
    assert.Equal(t, http.StatusBadRequest, err3.StatusCode)
  })
  
  t.Run("错误比较", func(t *testing.T) {
    err1 := errors.NewAppError("错误", errors.ErrCodeInternal, http.StatusInternalServerError)
    err2 := errors.NewAppError("错误", errors.ErrCodeInternal, http.StatusInternalServerError)
    err3 := errors.NewAppError("不同错误", errors.ErrCodeNotFound, http.StatusNotFound)
    
    // 由于时间戳不同，相同的错误内容也不相等
    assert.NotEqual(t, err1, err2)
    assert.NotEqual(t, err1, err3)
    
    // 但错误消息应该相同
    assert.Equal(t, err1.Message, err2.Message)
    assert.NotEqual(t, err1.Message, err3.Message)
  })
  
  t.Run("错误工厂函数", func(t *testing.T) {
    t.Run("NotFound", func(t *testing.T) {
      err := errors.NotFound("资源未找到")
      assert.Equal(t, "资源未找到", err.Message)
      assert.Equal(t, errors.ErrCodeNotFound, err.Code)
      assert.Equal(t, http.StatusNotFound, err.StatusCode)
    })
    
    t.Run("BadRequest", func(t *testing.T) {
      err := errors.BadRequest("无效请求")
      assert.Equal(t, "无效请求", err.Message)
      assert.Equal(t, errors.ErrCodeInvalidInput, err.Code)
      assert.Equal(t, http.StatusBadRequest, err.StatusCode)
    })
    
    t.Run("Unauthorized", func(t *testing.T) {
      err := errors.Unauthorized("未授权")
      assert.Equal(t, "未授权", err.Message)
      assert.Equal(t, errors.ErrCodeUnauthorized, err.Code)
      assert.Equal(t, http.StatusUnauthorized, err.StatusCode)
    })
    
    t.Run("Forbidden", func(t *testing.T) {
      err := errors.Forbidden("禁止访问")
      assert.Equal(t, "禁止访问", err.Message)
      assert.Equal(t, errors.ErrCodeForbidden, err.Code)
      assert.Equal(t, http.StatusForbidden, err.StatusCode)
    })
    
    t.Run("Internal", func(t *testing.T) {
      err := errors.Internal("内部错误")
      assert.Equal(t, "内部错误", err.Message)
      assert.Equal(t, errors.ErrCodeInternal, err.Code)
      assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
    })
    
    t.Run("Conflict", func(t *testing.T) {
      err := errors.Conflict("资源冲突")
      assert.Equal(t, "资源冲突", err.Message)
      assert.Equal(t, errors.ErrCodeConflict, err.Code)
      assert.Equal(t, http.StatusConflict, err.StatusCode)
    })
  })
  
  t.Run("错误上下文", func(t *testing.T) {
    err := errors.NewAppError("测试错误", errors.ErrCodeInternal, http.StatusInternalServerError)
    
    // 添加上下文
    err.AddContext("user_id", "123")
    err.AddContext("action", "create")
    err.AddContext("resource", "file")
    
    assert.Equal(t, "123", err.Context["user_id"])
    assert.Equal(t, "create", err.Context["action"])
    assert.Equal(t, "file", err.Context["resource"])
    
    // 错误消息包含上下文
    errorStr := err.Error()
    assert.Contains(t, errorStr, "user_id=123")
    assert.Contains(t, errorStr, "action=create")
    assert.Contains(t, errorStr, "resource=file")
  })
  
  t.Run("错误堆栈", func(t *testing.T) {
    err := errors.NewAppError("测试错误", errors.ErrCodeInternal, http.StatusInternalServerError)
    
    // 添加堆栈跟踪
    err.WithStack()
    
    assert.NotEmpty(t, err.Stack)
    // 堆栈应该包含文件名和行号
    assert.Contains(t, err.Stack, "errors_test.go")
  })
  
  t.Run("错误链解包", func(t *testing.T) {
    originalErr := errors.New("原始错误")
    wrappedErr := errors.Wrap(originalErr, "包装错误", errors.ErrCodeInternal, http.StatusInternalServerError)
    
    // 解包应该得到原始错误
    unwrapped := errors.Unwrap(wrappedErr)
    assert.Equal(t, originalErr, unwrapped)
    
    // 多层包装
    doubleWrapped := errors.Wrap(wrappedErr, "再次包装", errors.ErrCodeDatabase, http.StatusInternalServerError)
    unwrapped = errors.Unwrap(doubleWrapped)
    assert.Equal(t, wrappedErr, unwrapped)
    
    // 解包标准错误返回nil
    unwrapped = errors.Unwrap(originalErr)
    assert.Nil(t, unwrapped)
  })
  
  t.Run("错误类型判断", func(t *testing.T) {
    t.Run("IsNotFound", func(t *testing.T) {
      err := errors.NotFound("未找到")
      assert.True(t, errors.IsNotFound(err))
      
      otherErr := errors.BadRequest("无效请求")
      assert.False(t, errors.IsNotFound(otherErr))
      
      stdErr := errors.New("标准错误")
      assert.False(t, errors.IsNotFound(stdErr))
    })
    
    t.Run("IsBadRequest", func(t *testing.T) {
      err := errors.BadRequest("无效请求")
      assert.True(t, errors.IsBadRequest(err))
    })
    
    t.Run("IsUnauthorized", func(t *testing.T) {
      err := errors.Unauthorized("未授权")
      assert.True(t, errors.IsUnauthorized(err))
    })
    
    t.Run("IsForbidden", func(t *testing.T) {
      err := errors.Forbidden("禁止访问")
      assert.True(t, errors.IsForbidden(err))
    })
    
    t.Run("IsInternal", func(t *testing.T) {
      err := errors.Internal("内部错误")
      assert.True(t, errors.IsInternal(err))
    })
    
    t.Run("IsConflict", func(t *testing.T) {
      err := errors.Conflict("冲突")
      assert.True(t, errors.IsConflict(err))
    })
  })
  
  t.Run("错误响应序列化", func(t *testing.T) {
    err := errors.NewAppError("测试错误", errors.ErrCodeInvalidInput, http.StatusBadRequest)
    err.AddContext("field", "username")
    err.AddContext("value", "test")
    
    response := errors.ErrorResponse(err)
    
    // 转换为JSON并验证
    jsonData, marshalErr := json.Marshal(response)
    require.NoError(t, marshalErr)
    
    var decoded map[string]interface{}
    unmarshalErr := json.Unmarshal(jsonData, &decoded)
    require.NoError(t, unmarshalErr)
    
    assert.Equal(t, false, decoded["success"])
    assert.Equal(t, "测试错误", decoded["error"])
    assert.Equal(t, "INVALID_INPUT", decoded["code"])
    assert.Equal(t, float64(400), decoded["status"])
    assert.Contains(t, decoded, "timestamp")
    
    contextObj, ok := decoded["context"].(map[string]interface{})
    assert.True(t, ok)
    assert.Equal(t, "username", contextObj["field"])
    assert.Equal(t, "test", contextObj["value"])
  })
}

func TestErrorMiddleware(t *testing.T) {
  // 注意：这里我们只测试错误处理逻辑，不测试完整的中间件
  // 完整的中间件测试需要在handler测试中完成
  
  t.Run("错误响应格式", func(t *testing.T) {
    err := errors.NewAppError("测试错误", errors.ErrCodeInternal, http.StatusInternalServerError)
    response := errors.ErrorResponse(err)
    
    // 验证响应格式
    assert.IsType(t, gin.H{}, response)
    assert.Equal(t, false, response["success"])
    assert.Equal(t, "测试错误", response["error"])
    assert.Equal(t, "INTERNAL_ERROR", response["code"])
    assert.Equal(t, http.StatusInternalServerError, response["status"])
    assert.Contains(t, response, "timestamp")
  })
  
  t.Run("标准错误转换", func(t *testing.T) {
    stdErr := errors.New("标准错误")
    appErr := errors.ToAppError(stdErr)
    response := errors.ErrorResponse(appErr)
    
    assert.Equal(t, false, response["success"])
    assert.Equal(t, "标准错误", response["error"])
    assert.Equal(t, "INTERNAL_ERROR", response["code"])
    assert.Equal(t, http.StatusInternalServerError, response["status"])
  })
  
  t.Run("nil错误处理", func(t *testing.T) {
    response := errors.ErrorResponse(nil)
    assert.Nil(t, response)
  })
}

// 辅助函数：模拟JSON序列化
func jsonMarshal(v interface{}) ([]byte, error) {
  // 这里我们只是模拟，实际测试中应该使用encoding/json
  return []byte("{}"), nil
}

func jsonUnmarshal(data []byte, v interface{}) error {
  // 这里我们只是模拟，实际测试中应该使用encoding/json
  return nil
}