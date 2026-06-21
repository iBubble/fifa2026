package scheduler

import (
	"log"
	"sync"
)

// Task 表示排队执行的任务体
type Task struct {
	Name string
	Fn   func() error
}

// TaskScheduler 管理单协程串行执行任务的管道
type TaskScheduler struct {
	taskChan chan Task
	mu       sync.Mutex
	started  bool
}

var (
	globalScheduler *TaskScheduler
	once            sync.Once
)

// GetGlobalScheduler 获取全局唯一的任务调度器单例
func GetGlobalScheduler() *TaskScheduler {
	once.Do(func() {
		globalScheduler = &TaskScheduler{
			taskChan: make(chan Task, 200), // 设置 200 长度的缓冲区
		}
		globalScheduler.Start()
	})
	return globalScheduler
}

// Start 启动后台单线程消费协程
func (s *TaskScheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true

	go func() {
		log.Println("[Scheduler] 后台单线程任务调度服务已启动")
		for t := range s.taskChan {
			s.executeTask(t)
		}
	}()
}

// Submit 提交一个任务到队列中
func (s *TaskScheduler) Submit(name string, fn func() error) {
	task := Task{
		Name: name,
		Fn:   fn,
	}
	select {
	case s.taskChan <- task:
		log.Printf("[Scheduler] 成功向调度器提交任务: %s (当前队列堆积数: %d)", name, len(s.taskChan))
	default:
		log.Printf("[Scheduler] ⚠️ 任务队列已满，抛弃任务: %s", name)
	}
}

// executeTask 执行单个任务并进行 panic 恢复保护
func (s *TaskScheduler) executeTask(t Task) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Scheduler] ❌ 任务 %s 执行时崩溃 (Panic): %v", t.Name, r)
		}
	}()

	log.Printf("[Scheduler] ▶️ 开始串行执行任务: %s", t.Name)
	if err := t.Fn(); err != nil {
		log.Printf("[Scheduler] ❌ 任务 %s 执行出错: %v", t.Name, err)
	} else {
		log.Printf("[Scheduler] ✅ 任务 %s 执行成功", t.Name)
	}
}
