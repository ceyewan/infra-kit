package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ceyewan/infra-kit/coord"
	"github.com/ceyewan/infra-kit/coord/allocator"
)

func main() {
	fmt.Println("=== ID生成器 - 使用模式 ===")
	fmt.Println("演示ID分配的高级使用模式和最佳实践")

	// 创建协调器
	cfg := coord.GetDefaultConfig("development")
	provider, err := coord.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("创建协调器失败: %v", err)
	}
	defer provider.Close()

	allocatorService, err := provider.InstanceIDAllocator("pattern-service", 5)
	if err != nil {
		log.Fatalf("获取ID分配器失败: %v", err)
	}
	ctx := context.Background()

	// 1. 重试模式演示
	retryPatternDemo(ctx, allocatorService)

	// 2. 连接池模式演示
	poolPatternDemo(ctx, allocatorService)

	// 3. 工作队列模式演示
	workQueuePatternDemo(ctx, allocatorService)

	// 4. 监控和健康检查模式
	monitoringPatternDemo(ctx, allocatorService)

	// 5. 故障恢复模式
	recoveryPatternDemo(ctx, allocatorService)

	fmt.Println("\n=== 使用模式演示完成 ===")
}

// retryPatternDemo 演示重试模式
func retryPatternDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 重试模式 ---")

	// 带重试的ID获取函数
	acquireWithRetry := func(ctx context.Context, maxRetries int) (allocator.AllocatedID, error) {
		var lastErr error

		for i := 0; i < maxRetries; i++ {
			id, err := allocatorService.AcquireID(ctx)
			if err == nil {
				return id, nil
			}
			lastErr = err

			// 指数退避
			backoff := time.Duration(i+1) * 100 * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			fmt.Printf("  第 %d 次重试获取ID: %v\n", i+1, err)
		}

		return nil, fmt.Errorf("重试 %d 次后仍失败: %w", maxRetries, lastErr)
	}

	// 测试重试模式
	id, err := acquireWithRetry(ctx, 3)
	if err != nil {
		log.Printf("重试获取ID失败: %v", err)
		return
	}
	defer id.Close(ctx)

	fmt.Printf("✓ 重试成功，获取到ID: %d\n", id.ID())
}

// poolPatternDemo 演示连接池模式
func poolPatternDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 连接池模式 ---")

	const poolSize = 3
	const numWorkers = 10

	// ID池结构
	type IDPool struct {
		ids     chan allocator.AllocatedID
		service allocator.InstanceIDAllocator
		ctx     context.Context
		cancel  context.CancelFunc
		wg      sync.WaitGroup
	}

	// 创建ID池
	pool := &IDPool{
		ids:     make(chan allocator.AllocatedID, poolSize),
		service: allocatorService,
	}

	pool.ctx, pool.cancel = context.WithCancel(ctx)

	// 预分配ID到池中
	pool.wg.Add(poolSize)
	for i := 0; i < poolSize; i++ {
		go func() {
			defer pool.wg.Done()
			for {
				select {
				case <-pool.ctx.Done():
					return
				default:
					id, err := pool.service.AcquireID(pool.ctx)
					if err != nil {
						time.Sleep(100 * time.Millisecond)
						continue
					}
					select {
					case pool.ids <- id:
					case <-pool.ctx.Done():
						id.Close(pool.ctx)
						return
					}
				}
			}
		}()
	}

	// 使用池的函数
	getIDFromPool := func() (allocator.AllocatedID, error) {
		select {
		case id := <-pool.ids:
			return id, nil
		case <-time.After(2 * time.Second):
			return nil, fmt.Errorf("获取ID超时")
		}
	}

	releaseIDToPool := func(id allocator.AllocatedID) {
		// 在实际应用中，这里可以验证ID的状态
		// 如果ID仍然有效，可以返回池中重复使用
		// 这里直接释放
		id.Close(ctx)
	}

	// 测试池模式
	var wg sync.WaitGroup
	successCount := 0

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			id, err := getIDFromPool()
			if err != nil {
				fmt.Printf("  Worker %d 获取ID失败: %v\n", workerID, err)
				return
			}

			fmt.Printf("  Worker %d 使用ID %d\n", workerID, id.ID())

			// 模拟工作
			time.Sleep(time.Duration(50+workerID*10) * time.Millisecond)

			releaseIDToPool(id)
			successCount++
		}(i)
	}

	wg.Wait()
	fmt.Printf("✓ 池模式完成: %d/%d workers 成功获取ID\n", successCount, numWorkers)

	// 清理池
	pool.cancel()
	pool.wg.Wait()

	// 释放池中剩余的ID
	close(pool.ids)
	for id := range pool.ids {
		id.Close(ctx)
	}
}

// workQueuePatternDemo 演示工作队列模式
func workQueuePatternDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 工作队列模式 ---")

	// 工作任务
	type Task struct {
		ID      int
		Content string
	}

	// 工作队列
	workQueue := make(chan Task, 20)
	resultQueue := make(chan string, 20)

	const numWorkers = 3

	// 启动worker处理任务
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// 每个worker获取一个ID
			workerIDAlloc, err := allocatorService.AcquireID(ctx)
			if err != nil {
				log.Printf("Worker %d 获取ID失败: %v", workerID, err)
				return
			}
			defer workerIDAlloc.Close(ctx)

			fmt.Printf("  Worker %d 启动，分配ID: %d\n", workerID, workerIDAlloc.ID())

			for task := range workQueue {
				// 处理任务
				result := fmt.Sprintf("Worker-%d(ID:%d) 处理任务-%d: %s",
					workerID, workerIDAlloc.ID(), task.ID, task.Content)

				resultQueue <- result

				// 模拟处理时间
				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}

	// 发送任务
	go func() {
		for i := 0; i < 10; i++ {
			workQueue <- Task{
				ID:      i + 1,
				Content: fmt.Sprintf("任务内容 %d", i+1),
			}
		}
		close(workQueue)
	}()

	// 收集结果
	go func() {
		wg.Wait()
		close(resultQueue)
	}()

	// 处理结果
	for result := range resultQueue {
		fmt.Printf("  %s\n", result)
	}

	fmt.Printf("✓ 工作队列模式完成: 处理了 10 个任务\n")
}

// monitoringPatternDemo 演示监控和健康检查模式
func monitoringPatternDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 监控和健康检查模式 ---")

	// 监控指标
	metrics := &Metrics{
		TotalAllocated:   0,
		TotalReleased:    0,
		CurrentActive:    0,
		AllocationErrors: 0,
	}

	// 包装分配器以收集指标
	monitoredAllocator := &MonitoredAllocator{
		allocator: allocatorService,
		metrics:   metrics,
	}

	// 模拟一些分配操作
	for i := 0; i < 3; i++ {
		id, err := monitoredAllocator.AcquireID(ctx)
		if err != nil {
			log.Printf("分配ID失败: %v", err)
			continue
		}

		fmt.Printf("  分配ID %d\n", id.ID())

		// 模拟使用
		time.Sleep(50 * time.Millisecond)

		// 释放
		monitoredAllocator.ReleaseID(id)
	}

	// 打印监控信息
	metrics.mu.RLock()
	fmt.Printf("  监控指标:\n")
	fmt.Printf("    总分配数: %d\n", metrics.TotalAllocated)
	fmt.Printf("    总释放数: %d\n", metrics.TotalReleased)
	fmt.Printf("    当前活跃: %d\n", metrics.CurrentActive)
	fmt.Printf("    分配错误: %d\n", metrics.AllocationErrors)
	fmt.Printf("    最后分配: %v\n", metrics.LastAllocation)
	metrics.mu.RUnlock()

	fmt.Printf("✓ 监控模式演示完成\n")
}

// recoveryPatternDemo 演示故障恢复模式
func recoveryPatternDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 故障恢复模式 ---")

	// 模拟故障恢复的分配器
	resilientAllocator := &ResilientAllocator{
		allocator:   allocatorService,
		maxRetries:  3,
		backoffBase: 100 * time.Millisecond,
	}

	// 测试在压力下的表现
	const numOperations = 5
	successCount := 0
	errorCount := 0

	for i := 0; i < numOperations; i++ {
		id, err := resilientAllocator.AcquireID(ctx)
		if err != nil {
			errorCount++
			fmt.Printf("  操作 %d 失败: %v\n", i+1, err)
			continue
		}

		successCount++
		fmt.Printf("  操作 %d 成功，ID: %d\n", i+1, id.ID())

		// 模拟工作
		time.Sleep(20 * time.Millisecond)

		// 确保释放
		if err := resilientAllocator.ReleaseID(id); err != nil {
			fmt.Printf("    释放ID %d 失败: %v\n", id.ID(), err)
		}
	}

	fmt.Printf("✓ 故障恢复模式完成: 成功 %d, 失败 %d\n", successCount, errorCount)
}

// MonitoredAllocator 带监控的分配器包装器
type MonitoredAllocator struct {
	allocator allocator.InstanceIDAllocator
	metrics   *Metrics
}

func (m *MonitoredAllocator) AcquireID(ctx context.Context) (allocator.AllocatedID, error) {
	id, err := m.allocator.AcquireID(ctx)

	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	if err != nil {
		m.metrics.AllocationErrors++
		return nil, err
	}

	m.metrics.TotalAllocated++
	m.metrics.CurrentActive++
	m.metrics.LastAllocation = time.Now()

	return &MonitoredID{
		AllocatedID: id,
		metrics:     m.metrics,
	}, nil
}

func (m *MonitoredAllocator) ReleaseID(id allocator.AllocatedID) {
	if err := id.Close(context.Background()); err != nil {
		// 记录释放错误
	}
}

// MonitoredID 带监控的ID包装器
type MonitoredID struct {
	allocator.AllocatedID
	metrics *Metrics
}

func (m *MonitoredID) Close(ctx context.Context) error {
	err := m.AllocatedID.Close(ctx)

	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	m.metrics.TotalReleased++
	m.metrics.CurrentActive--

	return err
}

// ResilientAllocator 弹性分配器
type ResilientAllocator struct {
	allocator   allocator.InstanceIDAllocator
	maxRetries  int
	backoffBase time.Duration
}

func (r *ResilientAllocator) AcquireID(ctx context.Context) (allocator.AllocatedID, error) {
	var lastErr error

	for i := 0; i < r.maxRetries; i++ {
		id, err := r.allocator.AcquireID(ctx)
		if err == nil {
			return id, nil
		}
		lastErr = err

		// 指数退避
		backoff := r.backoffBase * time.Duration(1<<uint(i))
		if backoff > time.Second {
			backoff = time.Second
		}

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", r.maxRetries, lastErr)
}

func (r *ResilientAllocator) ReleaseID(id allocator.AllocatedID) error {
	return id.Close(context.Background())
}

// Metrics 监控指标
type Metrics struct {
	TotalAllocated   int64
	TotalReleased    int64
	CurrentActive    int64
	AllocationErrors int64
	LastAllocation   time.Time
	mu               sync.RWMutex
}
