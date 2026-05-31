package processor

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// WorkerPool 工作池
type WorkerPool struct {
	maxWorkers    int
	activeWorkers int
	mu            sync.RWMutex
	jobQueue      chan Job
	workerWg      sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	closed        bool
}

// Job 任务接口
type Job interface {
	Execute() error
	ID() string
}

// FileProcessingJob 文件处理任务
type FileProcessingJob struct {
	fileMd5    string
	steps      []string
	onComplete func(error)
}

// Execute 执行任务
func (j *FileProcessingJob) Execute() error {
	for _, step := range j.steps {
		_, err := ProcessStep(j.fileMd5, step)
		if err != nil {
			return fmt.Errorf("步骤 %s 处理失败: %w", step, err)
		}
	}

	// 审核步骤仅在调用方未显式包含 StepReview 时自动执行，避免双重执行
	hasReview := false
	for _, step := range j.steps {
		if step == StepReview {
			hasReview = true
			break
		}
	}
	if !hasReview {
		_, err := ProcessStep(j.fileMd5, StepReview)
		if err != nil {
			return fmt.Errorf("审核步骤处理失败: %w", err)
		}
	}

	return nil
}

// ID 返回任务ID
func (j *FileProcessingJob) ID() string {
	return j.fileMd5
}

// NewWorkerPool 创建工作池
func NewWorkerPool(maxWorkers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &WorkerPool{
		maxWorkers: maxWorkers,
		jobQueue:   make(chan Job, 100), // 缓冲队列
		ctx:        ctx,
		cancel:     cancel,
	}

	// 启动工作协程
	for i := 0; i < maxWorkers; i++ {
		pool.workerWg.Add(1)
		go pool.worker(i)
	}

	return pool
}

// worker 工作协程
func (p *WorkerPool) worker(id int) {
	defer p.workerWg.Done()

	for job := range p.jobQueue {
		p.mu.Lock()
		p.activeWorkers++
		p.mu.Unlock()

		log.Printf("Worker %d 开始处理任务: %s", id, job.ID())
		startTime := time.Now()

		err := job.Execute()

		p.mu.Lock()
		p.activeWorkers--
		p.mu.Unlock()

		duration := time.Since(startTime)
		if err != nil {
			log.Printf("Worker %d 任务 %s 处理失败 (耗时: %v): %v", id, job.ID(), duration, err)
		} else {
			log.Printf("Worker %d 任务 %s 处理完成 (耗时: %v)", id, job.ID(), duration)
		}

		// 调用完成回调
		if processingJob, ok := job.(*FileProcessingJob); ok && processingJob.onComplete != nil {
			processingJob.onComplete(err)
		}
	}
}

// Submit 提交任务
func (p *WorkerPool) Submit(job Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("工作池已关闭")
		}
	}()

	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return fmt.Errorf("工作池已关闭")
	}
	p.mu.RUnlock()

	select {
	case p.jobQueue <- job:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("任务队列已满，提交超时")
	}
}

// ActiveWorkers 获取活跃工作协程数
func (p *WorkerPool) ActiveWorkers() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.activeWorkers
}

// QueueSize 获取队列大小
func (p *WorkerPool) QueueSize() int {
	return len(p.jobQueue)
}

// Shutdown 关闭工作池（支持重复调用）
// 先关闭任务接收通道，等待所有 worker 处理完当前任务后退出
func (p *WorkerPool) Shutdown() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()

	// 先取消 context，让正在阻塞的 Submit 及时返回
	p.cancel()
	// 关闭任务队列，worker 处理完当前任务后退出
	close(p.jobQueue)
	// 等待所有 worker 退出
	p.workerWg.Wait()
	log.Println("工作池已关闭")
}

// DefaultWorkerPool 默认工作池
var (
	defaultWorkerPool *WorkerPool
	once              sync.Once
)

// GetWorkerPool 获取默认工作池（单例）
func GetWorkerPool() *WorkerPool {
	once.Do(func() {
		// 默认最大并发数为CPU核心数，至少为1
		maxWorkers := runtime.NumCPU()
		if maxWorkers < 1 {
			maxWorkers = 1
		}
		defaultWorkerPool = NewWorkerPool(maxWorkers)
		log.Printf("初始化工作池，最大工作协程数: %d", maxWorkers)
	})
	return defaultWorkerPool
}

// SubmitFileProcessing 提交文件处理任务
func SubmitFileProcessing(fileMd5 string, steps []string, onComplete func(error)) error {
	job := &FileProcessingJob{
		fileMd5:    fileMd5,
		steps:      steps,
		onComplete: onComplete,
	}

	pool := GetWorkerPool()
	return pool.Submit(job)
}
