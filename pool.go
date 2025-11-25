package main

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrWorkerPoolFreed = errors.New("workerpool freed")
)

type Task func()

type Pool struct {
	capacity int // workerpool大小

	active chan struct{} // 对应上图中的active channel
	tasks  chan Task     // 对应上图中的task channel

	wg   sync.WaitGroup // 用于在pool销毁时等待所有worker退出
	quit chan struct{}  // 用于通知各个worker退出的信号channel
}

const (
	defaultCapacity = 100
	maxCapacity     = 1000
)

func New(capacity int) *Pool {
	if capacity < 0 {
		capacity = defaultCapacity
	}

	if capacity > maxCapacity {
		capacity = defaultCapacity
	}

	pool := &Pool{
		capacity: defaultCapacity,
		active:   make(chan struct{}, capacity),
		tasks:    make(chan Task),
		wg:       sync.WaitGroup{},
		quit:     make(chan struct{}),
	}

	fmt.Println("workerpool start")

	go pool.run()

	return pool

}

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

func (p *Pool) newWorker(id int) {
	p.wg.Add(1)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("worker[%d] recover panic[%s] and exit\n", id, err)
				<-p.active
			}
			p.wg.Done()
		}()

		fmt.Printf("worker[%d] start\n", id)

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

func (p *Pool) Free() {
	close(p.quit)
	p.wg.Wait()
	fmt.Printf("workerpool free\n")
}

func (p *Pool) Schedule(t Task) error {
	select {
	case <-p.quit:
		return ErrWorkerPoolFreed
	case p.tasks <- t:
		return nil
	}
}
