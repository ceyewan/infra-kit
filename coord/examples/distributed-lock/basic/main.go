package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ceyewan/infra-kit/coord"
	"github.com/ceyewan/infra-kit/coord/lock"
)

func main() {
	fmt.Println("=== 分布式锁 - 基础用法 ===")
	fmt.Println("演示分布式锁的基本获取、使用和释放流程")

	// 创建协调器
	cfg := coord.GetDefaultConfig("development")
	provider, err := coord.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("创建协调器失败: %v", err)
	}
	defer provider.Close()

	lockService := provider.Lock()
	ctx := context.Background()

	// 1. 基础锁获取和释放
	basicLockDemo(ctx, lockService)

	// 2. 阻塞vs非阻塞获取
	blockingVsNonBlockingDemo(ctx, lockService)

	// 3. 锁的TTL管理
	ttlManagementDemo(ctx, lockService)

	fmt.Println("\n=== 基础用法示例完成 ===")
}

// basicLockDemo 演示最基本的锁获取和释放
func basicLockDemo(ctx context.Context, lockService lock.DistributedLock) {
	fmt.Println("\n--- 基础锁获取和释放 ---")
	const lockKey = "basic-demo-lock"

	// 获取锁
	lock, err := lockService.Acquire(ctx, lockKey, 10*time.Second)
	if err != nil {
		log.Fatalf("获取锁失败: %v", err)
	}

	fmt.Printf("✓ 获取锁成功: %s\n", lock.Key())

	// 模拟受保护的工作
	fmt.Println("  执行受保护的工作...")
	time.Sleep(1 * time.Second)

	// 释放锁
	if err := lock.Unlock(ctx); err != nil {
		log.Printf("释放锁失败: %v", err)
	} else {
		fmt.Println("  ✓ 锁已释放")
	}
}

// blockingVsNonBlockingDemo 演示阻塞和非阻塞获取锁的区别
func blockingVsNonBlockingDemo(ctx context.Context, lockService lock.DistributedLock) {
	fmt.Println("\n--- 阻塞 vs 非阻塞获取锁 ---")
	const lockKey = "blocking-demo-lock"

	// 首先获取一个锁并持有
	lock1, err := lockService.Acquire(ctx, lockKey, 5*time.Second)
	if err != nil {
		log.Fatalf("首次获取锁失败: %v", err)
	}
	fmt.Println("  已持有锁，尝试第二次获取...")

	// 非阻塞尝试获取（应该失败）
	_, err = lockService.TryAcquire(ctx, lockKey, 5*time.Second)
	if err != nil {
		fmt.Printf("  ✓ 非阻塞获取失败（符合预期）: %v\n", err)
	} else {
		fmt.Println("  ✗ 非阻塞获取意外成功")
	}

	// 释放第一个锁
	if err := lock1.Unlock(ctx); err != nil {
		log.Printf("释放锁失败: %v", err)
		return
	}

	// 现在非阻塞获取应该成功
	lock2, err := lockService.TryAcquire(ctx, lockKey, 5*time.Second)
	if err != nil {
		log.Printf("非阻塞获取失败: %v", err)
	} else {
		fmt.Println("  ✓ 锁释放后，非阻塞获取成功")
		lock2.Unlock(ctx)
	}
}

// ttlManagementDemo 演示锁的TTL管理
func ttlManagementDemo(ctx context.Context, lockService lock.DistributedLock) {
	fmt.Println("\n--- 锁的TTL管理 ---")
	const lockKey = "ttl-demo-lock"

	// 获取一个短TTL的锁
	lock, err := lockService.Acquire(ctx, lockKey, 3*time.Second)
	if err != nil {
		log.Fatalf("获取锁失败: %v", err)
	}
	defer lock.Unlock(ctx)

	// 检查初始TTL
	ttl, err := lock.TTL(ctx)
	if err != nil {
		log.Printf("获取TTL失败: %v", err)
		return
	}
	fmt.Printf("  初始TTL: %v\n", ttl)

	// 等待一段时间后检查TTL
	time.Sleep(1 * time.Second)
	ttl, err = lock.TTL(ctx)
	if err == nil {
		fmt.Printf("  1秒后TTL: %v\n", ttl)
	}

	// 检查锁是否过期
	expired, err := lock.IsExpired(ctx)
	if err == nil {
		fmt.Printf("  锁是否过期: %v\n", expired)
	}

	fmt.Println("  ✓ TTL管理演示完成")
}

// concurrencyDemo 演示并发环境下的锁使用
func concurrencyDemo(ctx context.Context, lockService lock.DistributedLock) {
	fmt.Println("\n--- 并发环境下的锁使用 ---")
	const lockKey = "concurrency-demo-lock"
	const numWorkers = 5

	var wg sync.WaitGroup
	results := make(chan string, numWorkers)

	// 启动多个worker并发尝试获取锁
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			startTime := time.Now()

			// 阻塞获取锁
			lock, err := lockService.Acquire(ctx, lockKey, 2*time.Second)
			if err != nil {
				results <- fmt.Sprintf("Worker %d: 获取锁失败 (%v)", workerID, err)
				return
			}

			// 成功获取锁
			acquireTime := time.Since(startTime)
			results <- fmt.Sprintf("Worker %d: 获取锁成功 (耗时: %v)", workerID, acquireTime)

			// 模拟工作
			time.Sleep(200 * time.Millisecond)

			// 释放锁
			lock.Unlock(ctx)
		}(i)
	}

	// 收集结果
	go func() {
		wg.Wait()
		close(results)
	}()

	// 显示结果
	successCount := 0
	for result := range results {
		fmt.Printf("  %s\n", result)
		if contains(result, "获取锁成功") {
			successCount++
		}
	}

	fmt.Printf("  并发测试结果: %d/%d 成功获取锁\n", successCount, numWorkers)
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
