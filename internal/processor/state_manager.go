package processor

import (
  "fmt"
  "github.com/gao/Builds/voidtext/internal/database"
)

// UpdateFileStatusWithLog 更新文件状态并记录日志（原子操作）
func UpdateFileStatusWithLog(fileMd5, status, currentStep string, progress int, errorMsg string, logDetails map[string]interface{}) error {
  return database.UpdateFileStatusWithLog(fileMd5, status, currentStep, progress, errorMsg, logDetails)
}

// UpdateFileStatusSimple 简化版状态更新（仅更新状态）
func UpdateFileStatusSimple(fileMd5, status, currentStep string, progress int, errorMsg string) error {
  return database.UpdateFileStatus(fileMd5, status, currentStep, progress, errorMsg)
}

// UpdateFileStatusWithStepProgress 更新文件状态和步骤进度
func UpdateFileStatusWithStepProgress(fileMd5, status, currentStep string, progress int) error {
  return database.UpdateFileStatus(fileMd5, status, currentStep, progress, "")
}

// LogProcessingStep 记录处理步骤日志
func LogProcessingStep(fileMd5, step, action, status string, details map[string]interface{}) error {
  detailsJSON := "{}"
  if details != nil {
    // 在database包中处理JSON序列化
    return database.CreateProcessingLog(&database.ProcessingLogRecord{
      FileMd5: fileMd5,
      Step:    step,
      Action:  action,
      Details: details,
      Status:  status,
    })
  }
  
  return database.CreateProcessingLog(&database.ProcessingLogRecord{
    FileMd5: fileMd5,
    Step:    step,
    Action:  action,
    Details: nil,
    Status:  status,
  })
}

// LogProcessingError 记录处理错误
func LogProcessingError(fileMd5, step string, err error) error {
  return database.CreateProcessingLog(&database.ProcessingLogRecord{
    FileMd5: fileMd5,
    Step:    step,
    Action:  "error",
    Details: map[string]interface{}{
      "error": err.Error(),
    },
    Status: "failed",
  })
}

// LogProcessingSuccess 记录处理成功
func LogProcessingSuccess(fileMd5, step string, details map[string]interface{}) error {
  return database.CreateProcessingLog(&database.ProcessingLogRecord{
    FileMd5: fileMd5,
    Step:    step,
    Action:  "success",
    Details: details,
    Status:  "completed",
  })
}

// UpdateStatusAndLog 更新状态并记录日志（原子操作）
func UpdateStatusAndLog(fileMd5, status, currentStep string, progress int, errorMsg string, logAction string, logDetails map[string]interface{}) error {
  // 使用事务更新状态和记录日志
  return database.UpdateFileStatusWithLog(fileMd5, status, currentStep, progress, errorMsg, map[string]interface{}{
    "action":  logAction,
    "details": logDetails,
  })
}

// StartProcessingStep 开始处理步骤
func StartProcessingStep(fileMd5, step string) error {
  return UpdateStatusAndLog(fileMd5, "processing", step, CalculateProgress(step, 0), "", "step_started", map[string]interface{}{
    "step": step,
  })
}

// CompleteProcessingStep 完成处理步骤
func CompleteProcessingStep(fileMd5, step string, result map[string]interface{}) error {
  nextStep := GetNextStep(step)
  progress := CalculateProgress(nextStep, 0)
  
  return UpdateStatusAndLog(fileMd5, "processing", nextStep, progress, "", "step_completed", map[string]interface{}{
    "step":   step,
    "result": result,
  })
}

// FailProcessingStep 处理步骤失败
func FailProcessingStep(fileMd5, step string, err error) error {
  return UpdateStatusAndLog(fileMd5, "failed", step, 0, err.Error(), "step_failed", map[string]interface{}{
    "step":  step,
    "error": err.Error(),
  })
}

// SkipProcessingStep 跳过处理步骤
func SkipProcessingStep(fileMd5, step, reason string) error {
  nextStep := GetNextStep(step)
  progress := CalculateProgress(nextStep, 0)
  
  return UpdateStatusAndLog(fileMd5, "processing", nextStep, progress, "", "step_skipped", map[string]interface{}{
    "step":   step,
    "reason": reason,
  })
}