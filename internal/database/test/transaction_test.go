package test

import (
  "context"
  "path/filepath"
  "testing"
  "time"
  
  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
  
  "voidtext/internal/database"
)

func TestTransaction(t *testing.T) {
  // 创建临时数据库
  tempDir := t.TempDir()
  dbPath := filepath.Join(tempDir, "test.db")
  
  // 初始化数据库
  err := database.Init(tempDir)
  require.NoError(t, err)
  defer database.Close()
  
  // 测试事务包装器
  t.Run("WithTransaction成功提交", func(t *testing.T) {
    var executed bool
    err := database.WithTransaction(context.Background(), func(ctx context.Context) error {
      executed = true
      return nil
    })
    
    assert.NoError(t, err)
    assert.True(t, executed)
  })
  
  t.Run("WithTransaction回滚", func(t *testing.T) {
    var executed bool
    testErr := assert.AnError
    err := database.WithTransaction(context.Background(), func(ctx context.Context) error {
      executed = true
      return testErr
    })
    
    assert.Error(t, err)
    assert.Equal(t, testErr, err)
    assert.True(t, executed)
  })
  
  t.Run("CreateFileWithVersion", func(t *testing.T) {
    ctx := context.Background()
    
    // 创建文件记录
    fileRecord := &database.FileRecord{
      Md5:      "test_md5_" + time.Now().Format("20060102150405"),
      Author:   "test_author",
      Title:    "test_title",
      FileName: "test.txt",
      FileSize: 1024,
      FilePath: "/tmp/test.txt",
    }
    
    version := &database.FileVersion{
      Step:          "cleaning",
      OriginalMd5:   "original_md5",
      ProcessedMd5:  "processed_md5",
      ProcessedPath: "/tmp/processed.txt",
    }
    
    err := database.CreateFileWithVersion(ctx, fileRecord, version)
    assert.NoError(t, err)
    
    // 验证文件记录
    retrievedFile, err := database.GetFileByMd5(fileRecord.Md5)
    assert.NoError(t, err)
    assert.NotNil(t, retrievedFile)
    assert.Equal(t, fileRecord.Author, retrievedFile.Author)
    assert.Equal(t, fileRecord.Title, retrievedFile.Title)
    
    // 验证版本记录
    versions, err := database.GetFileVersions(fileRecord.Md5)
    assert.NoError(t, err)
    assert.Len(t, versions, 1)
    assert.Equal(t, version.Step, versions[0].Step)
  })
  
  t.Run("UpdateFileStatusWithLog", func(t *testing.T) {
    ctx := context.Background()
    
    // 先创建文件
    fileRecord := &database.FileRecord{
      Md5:      "test_status_md5_" + time.Now().Format("20060102150405"),
      Author:   "test_author",
      Title:    "test_title",
      FileName: "test.txt",
      FileSize: 1024,
      FilePath: "/tmp/test.txt",
      Status:   "pending",
    }
    
    version := &database.FileVersion{
      Step:          "cleaning",
      OriginalMd5:   "original_md5",
      ProcessedMd5:  "processed_md5",
      ProcessedPath: "/tmp/processed.txt",
    }
    
    err := database.CreateFileWithVersion(ctx, fileRecord, version)
    require.NoError(t, err)
    
    // 更新状态
    newStatus := "processing"
    message := "开始处理"
    err = database.UpdateFileStatusWithLog(ctx, fileRecord.Md5, newStatus, message)
    assert.NoError(t, err)
    
    // 验证状态更新
    updatedFile, err := database.GetFileByMd5(fileRecord.Md5)
    assert.NoError(t, err)
    assert.Equal(t, newStatus, updatedFile.Status)
    
    // 验证日志记录
    logs, err := database.GetFileLogs(fileRecord.Md5)
    assert.NoError(t, err)
    assert.NotEmpty(t, logs)
    assert.Contains(t, logs[0].Message, message)
  })
  
  t.Run("DeleteFileWithRelatedData", func(t *testing.T) {
    ctx := context.Background()
    
    // 先创建文件和相关数据
    fileRecord := &database.FileRecord{
      Md5:      "test_delete_md5_" + time.Now().Format("20060102150405"),
      Author:   "test_author",
      Title:    "test_title",
      FileName: "test.txt",
      FileSize: 1024,
      FilePath: "/tmp/test.txt",
    }
    
    version := &database.FileVersion{
      Step:          "cleaning",
      OriginalMd5:   "original_md5",
      ProcessedMd5:  "processed_md5",
      ProcessedPath: "/tmp/processed.txt",
    }
    
    err := database.CreateFileWithVersion(ctx, fileRecord, version)
    require.NoError(t, err)
    
    // 添加日志
    err = database.AddFileLog(fileRecord.Md5, "test_log", "info")
    require.NoError(t, err)
    
    // 删除文件和相关数据
    err = database.DeleteFileWithRelatedData(ctx, fileRecord.Md5)
    assert.NoError(t, err)
    
    // 验证文件已删除
    deletedFile, err := database.GetFileByMd5(fileRecord.Md5)
    assert.NoError(t, err)
    assert.Nil(t, deletedFile)
    
    // 验证版本已删除
    versions, err := database.GetFileVersions(fileRecord.Md5)
    assert.NoError(t, err)
    assert.Empty(t, versions)
    
    // 验证日志已删除
    logs, err := database.GetFileLogs(fileRecord.Md5)
    assert.NoError(t, err)
    assert.Empty(t, logs)
  })
  
  t.Run("嵌套事务", func(t *testing.T) {
    ctx := context.Background()
    
    var outerExecuted bool
    var innerExecuted bool
    
    err := database.WithTransaction(ctx, func(ctx context.Context) error {
      outerExecuted = true
      
      // 内层事务
      return database.WithTransaction(ctx, func(ctx context.Context) error {
        innerExecuted = true
        return nil
      })
    })
    
    assert.NoError(t, err)
    assert.True(t, outerExecuted)
    assert.True(t, innerExecuted)
  })
  
  t.Run("事务中的错误传播", func(t *testing.T) {
    ctx := context.Background()
    
    var executed bool
    testErr := assert.AnError
    
    err := database.WithTransaction(ctx, func(ctx context.Context) error {
      executed = true
      
      // 内层事务返回错误
      return database.WithTransaction(ctx, func(ctx context.Context) error {
        return testErr
      })
    })
    
    assert.Error(t, err)
    assert.Equal(t, testErr, err)
    assert.True(t, executed)
  })
}

func TestTransactionConcurrency(t *testing.T) {
  // 创建临时数据库
  tempDir := t.TempDir()
  
  // 初始化数据库
  err := database.Init(tempDir)
  require.NoError(t, err)
  defer database.Close()
  
  ctx := context.Background()
  
  // 并发测试
  t.Run("并发事务", func(t *testing.T) {
    const numGoroutines = 10
    const numOperations = 100
    
    errors := make(chan error, numGoroutines)
    
    for i := 0; i < numGoroutines; i++ {
      go func(id int) {
        for j := 0; j < numOperations; j++ {
          md5 := "concurrent_md5_" + time.Now().Format("20060102150405") + "_" + string(rune(id)) + "_" + string(rune(j))
          
          err := database.WithTransaction(ctx, func(ctx context.Context) error {
            fileRecord := &database.FileRecord{
              Md5:      md5,
              Author:   "author_" + string(rune(id)),
              Title:    "title_" + string(rune(j)),
              FileName: "test.txt",
              FileSize: 1024,
              FilePath: "/tmp/test.txt",
            }
            
            version := &database.FileVersion{
              Step:          "cleaning",
              OriginalMd5:   "original_md5",
              ProcessedMd5:  "processed_md5",
              ProcessedPath: "/tmp/processed.txt",
            }
            
            return database.CreateFileWithVersion(ctx, fileRecord, version)
          })
          
          if err != nil {
            errors <- err
            return
          }
        }
        errors <- nil
      }(i)
    }
    
    // 收集错误
    for i := 0; i < numGoroutines; i++ {
      err := <-errors
      assert.NoError(t, err)
    }
  })
}

func TestTransactionIsolation(t *testing.T) {
  // 创建临时数据库
  tempDir := t.TempDir()
  
  // 初始化数据库
  err := database.Init(tempDir)
  require.NoError(t, err)
  defer database.Close()
  
  ctx := context.Background()
  
  t.Run("事务隔离级别", func(t *testing.T) {
    // 测试读已提交隔离级别
    md5 := "isolation_md5_" + time.Now().Format("20060102150405")
    
    // 事务1：创建文件
    err := database.WithTransaction(ctx, func(ctx context.Context) error {
      fileRecord := &database.FileRecord{
        Md5:      md5,
        Author:   "author",
        Title:    "title",
        FileName: "test.txt",
        FileSize: 1024,
        FilePath: "/tmp/test.txt",
      }
      
      version := &database.FileVersion{
        Step:          "cleaning",
        OriginalMd5:   "original_md5",
        ProcessedMd5:  "processed_md5",
        ProcessedPath: "/tmp/processed.txt",
      }
      
      return database.CreateFileWithVersion(ctx, fileRecord, version)
    })
    require.NoError(t, err)
    
    // 事务2：读取文件（应该能看到提交的数据）
    var foundFile *database.FileRecord
    err = database.WithTransaction(ctx, func(ctx context.Context) error {
      file, err := database.GetFileByMd5(md5)
      if err != nil {
        return err
      }
      foundFile = file
      return nil
    })
    
    assert.NoError(t, err)
    assert.NotNil(t, foundFile)
    assert.Equal(t, md5, foundFile.Md5)
  })
  
  t.Run("事务回滚不影响其他事务", func(t *testing.T) {
    md51 := "rollback_md5_1_" + time.Now().Format("20060102150405")
    md52 := "rollback_md5_2_" + time.Now().Format("20060102150405")
    
    // 事务1：成功提交
    err := database.WithTransaction(ctx, func(ctx context.Context) error {
      fileRecord := &database.FileRecord{
        Md5:      md51,
        Author:   "author1",
        Title:    "title1",
        FileName: "test1.txt",
        FileSize: 1024,
        FilePath: "/tmp/test1.txt",
      }
      
      version := &database.FileVersion{
        Step:          "cleaning",
        OriginalMd5:   "original_md51",
        ProcessedMd5:  "processed_md51",
        ProcessedPath: "/tmp/processed1.txt",
      }
      
      return database.CreateFileWithVersion(ctx, fileRecord, version)
    })
    require.NoError(t, err)
    
    // 事务2：回滚
    err = database.WithTransaction(ctx, func(ctx context.Context) error {
      fileRecord := &database.FileRecord{
        Md5:      md52,
        Author:   "author2",
        Title:    "title2",
        FileName: "test2.txt",
        FileSize: 1024,
        FilePath: "/tmp/test2.txt",
      }
      
      version := &database.FileVersion{
        Step:          "cleaning",
        OriginalMd5:   "original_md52",
        ProcessedMd5:  "processed_md52",
        ProcessedPath: "/tmp/processed2.txt",
      }
      
      if err := database.CreateFileWithVersion(ctx, fileRecord, version); err != nil {
        return err
      }
      
      // 故意返回错误导致回滚
      return assert.AnError
    })
    assert.Error(t, err)
    
    // 验证事务1的数据存在
    file1, err := database.GetFileByMd5(md51)
    assert.NoError(t, err)
    assert.NotNil(t, file1)
    
    // 验证事务2的数据不存在（已回滚）
    file2, err := database.GetFileByMd5(md52)
    assert.NoError(t, err)
    assert.Nil(t, file2)
  })
}