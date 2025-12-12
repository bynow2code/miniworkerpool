package main

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrWorkerPoolFreed 表示工作池已被释放的错误
	ErrWorkerPoolFreed = errors.New("workerpool freed")
)

// Task 定义任务类型，是一个无参数无返回值的函数
type Task func()

// Option 定义选项模式函数类型，用于配置工作池
type Option func(pool *Pool)

// Pool 定义工作池结构体
type Pool struct {
	preAlloc bool // 是否在创建pool的时候就预创建workers，默认值为：false
	capacity int  // workerpool大小

	active chan struct{} // 控制并发worker数量的信号通道
	tasks  chan Task     // 任务队列通道

	wg   sync.WaitGroup // 用于在pool销毁时等待所有worker退出
	quit chan struct{}  // 用于通知各个worker退出的信号channel
}

const (
	defaultCapacity = 100  // 默认容量
	maxCapacity     = 1000 // 最大容量
)

// New 创建一个新的工作池实例
func New(capacity int, opts ...Option) *Pool {
	if capacity < 0 {
		capacity = defaultCapacity
	}

	if capacity > maxCapacity {
		capacity = defaultCapacity
	}

	p := &Pool{
		capacity: defaultCapacity,
		active:   make(chan struct{}, capacity),
		tasks:    make(chan Task),
		wg:       sync.WaitGroup{},
		quit:     make(chan struct{}),
	}

	// 应用可选配置
	for _, opt := range opts {
		opt(p)
	}

	fmt.Println("workerpool start")

	// 如果启用预分配，则预先创建指定数量的worker
	if p.preAlloc {
		for i := 1; i <= p.capacity; i++ {
			p.newWorker(i)
			p.active <- struct{}{}
		}
	}

	// 启动工作池运行协程
	go p.run()

	return p
}

// WithPreAlloc 设置是否预分配worker的选项
func WithPreAlloc(b bool) Option {
	return func(p *Pool) {
		p.preAlloc = b
	}
}

// run 运行工作池主循环，动态创建worker
func (p *Pool) run() {
	id := 0
	for {
		select {
		case <-p.quit:
			return
		case p.active <- struct{}{}:
			id++
			p.newWorker(id)
		}
	}
}

// newWorker 创建一个新的工作协程
func (p *Pool) newWorker(id int) {
	p.wg.Add(1)
	go func() {
		defer func() {
			// 捕获可能的panic并优雅地退出
			if err := recover(); err != nil {
				fmt.Printf("worker[%d] recover panic[%s] and exit\n", id, err)
				<-p.active
			}
			p.wg.Done()
		}()

		fmt.Printf("worker[%d] start\n", id)

		// 工作协程主循环
		for {
			select {
			case <-p.quit:
				fmt.Printf("worker[%d] exit\n", id)
				<-p.active
				return
			case t := <-p.tasks:
				fmt.Printf("worker[%d] receive a task\n", id)
				t()
			}
		}
	}()
}

// Free 释放工作池资源，等待所有worker退出
func (p *Pool) Free() {
	close(p.quit)
	p.wg.Wait()
	fmt.Printf("workerpool free\n")
}

// Schedule 提交一个任务到工作池执行
func (p *Pool) Schedule(t Task) error {
	select {
	case <-p.quit:
		return ErrWorkerPoolFreed
	case p.tasks <- t:
		return nil
	}
}
