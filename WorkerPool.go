package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

type WorkerPool struct {
	tasks       chan string
	mu          sync.Mutex
	cancelFuncs map[int]context.CancelFunc
	nextID      int
}

func NewWorkerPool(buffer int) *WorkerPool {
	return &WorkerPool{
		tasks:       make(chan string, buffer),
		cancelFuncs: make(map[int]context.CancelFunc),
		nextID:      1,
	}
}

func (wp *WorkerPool) AddWorker() int {
	wp.mu.Lock()
	id := wp.nextID
	wp.nextID++
	wp.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	go func(workerID int, ctx context.Context) {
		fmt.Printf("[Worker %d] started\n", workerID)
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("[Worker %d] stopped\n", workerID)
				return
			case task, ok := <-wp.tasks:
				if !ok {
					fmt.Printf("[Worker %d] tasks channel closed, exiting\n", workerID)
					return
				}
				fmt.Printf("[Worker %d] processing task: %s\n", workerID, task)

			}
		}
	}(id, ctx)

	wp.mu.Lock()
	wp.cancelFuncs[id] = cancel
	wp.mu.Unlock()

	return id
}

func (wp *WorkerPool) RemoveWorker(id int) error {
	wp.mu.Lock()
	cancel, exists := wp.cancelFuncs[id]
	if !exists {
		wp.mu.Unlock()
		return fmt.Errorf("worker %d not found", id)
	}
	delete(wp.cancelFuncs, id)
	wp.mu.Unlock()

	cancel()
	return nil
}

func (wp *WorkerPool) Submit(task string) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if len(wp.cancelFuncs) == 0 {
		return fmt.Errorf("no workers available to process tasks")
	}

	select {
	case wp.tasks <- task:
		return nil
	default:
		return fmt.Errorf("task queue is full, cannot submit task")
	}
}

func (wp *WorkerPool) Shutdown() {
	wp.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(wp.cancelFuncs))
	for _, cancel := range wp.cancelFuncs {
		cancels = append(cancels, cancel)
	}
	wp.cancelFuncs = nil
	wp.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	close(wp.tasks)
}

func main() {
	pool := NewWorkerPool(10)
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println("stdin closed, exiting")
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		cmd := parts[0]

		switch cmd {
		case "add":
			id := pool.AddWorker()
			fmt.Printf("Added worker with id=%d\n", id)
		case "remove":
			if len(parts) < 2 {
				fmt.Println("Usage: remove <id>")
				continue
			}
			idStr := parts[1]
			id, err := strconv.Atoi(idStr)
			if err != nil {
				fmt.Printf("Invalid id: %s\n", idStr)
				continue
			}
			if err := pool.RemoveWorker(id); err != nil {
				fmt.Printf("Error removing worker: %v\n", err)
			} else {
				fmt.Printf("Removed worker id=%d\n", id)
			}
		case "exit":
			fmt.Println("Shutting down pool...")
			pool.Shutdown()
			fmt.Println("Pool shut down. Exiting.")
			return
		default:
			if err := pool.Submit(line); err != nil {
				fmt.Printf("Failed to submit task: %v\n", err)
			} else {
				fmt.Println("Task submitted")
			}
		}
	}

	pool.Shutdown()
	fmt.Println("Exited loop; pool shut down.")
}
