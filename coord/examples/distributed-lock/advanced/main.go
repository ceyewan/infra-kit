package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ceyewan/infra-kit/coord"
	"github.com/ceyewan/infra-kit/coord/lock"
)

func main() {
	fmt.Println("=== 分布式锁 - 高级用法 ===")
	fmt.Println("演示TTL管理、手动续约、过期检查等高级功能")

	// 创建协调器
	cfg := coord.GetDefaultConfig("development")
	provider, err := coord.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("创建协调器失败: %v", err)
	}
	defer provider.Close()

	lockService := provider.Lock()
	ctx := context.Background()

	// 1. 手动续约演示
	manualRenewDemo(ctx, lockService)

	// 2. 过期状态监控
	expirationMonitoringDemo(ctx, lockService)

	// 3. 长时间持有锁的场景
	longLockHoldDemo(ctx, lockService)

	fmt.Println("\n=== 高级用法示例完成 ===")
}

// manualRenewDemo 演示手动续约功能
func manualRenewDemo(ctx context.Context, lockService lock.DistributedLock) {
	fmt.Println("\n--- 手动续约演示 ---")
	const lockKey = "manual-renew-demo"

	// 获取一个短TTL的锁
	lock, err := lockService.Acquire(ctx, lockKey, 5*time.Second)
	if err != nil {
		log.Fatalf("获取锁失败: %v", err)
	}
	defer lock.Unlock(ctx)

	fmt.Printf("✓ 获取锁成功: %s\n", lock.Key())

	// 检查初始TTL
	ttl, err := lock.TTL(ctx)
	if err != nil {
		log.Printf("获取TTL失败: %v", err)
		return
	}
	fmt.Printf("  初始TTL: %v\n", ttl)

	// 等待一段时间后手动续约
	fmt.Println("  等待3秒后手动续约...")
	time.Sleep(3 * time.Second)

	// 检查剩余TTL
	ttl, err = lock.TTL(ctx)
	if err != nil {
		log.Printf("获取TTL失败: %v", err)
		return
	}
	fmt.Printf("  续约前TTL: %v\n", ttl)

	// 手动续约
	success, err := lock.Renew(ctx)
	if err != nil {
		log.Printf("手动续约失败: %v", err)
		return
	}

	if success {
		fmt.Println("  ✓ 手动续约成功")

		// 检查续约后的TTL
		ttl, err = lock.TTL(ctx)
		if err == nil {
			fmt.Printf("  续约后TTL: %v\n", ttl)
		}
	} else {
		fmt.Println("  ✗ 手动续约失败")
	}
}

// expirationMonitoringDemo 演示过期状态监控
func expirationMonitoringDemo(ctx context.Context, lockService lock.DistributedLock) {
	fmt.Println("\n--- 过期状态监控 ---")
	const lockKey = "expiration-monitoring-demo"

	// 获取一个极短TTL的锁用于演示
	lock, err := lockService.Acquire(ctx, lockKey, 3*time.Second)
	if err != nil {
		log.Fatalf("获取锁失败: %v", err)
	}

	fmt.Printf("✓ 获取锁成功: %s\n", lock.Key())

	// 监控锁状态
	for i := 0; i < 10; i++ {
		ttl, err := lock.TTL(ctx)
		if err != nil {
			log.Printf("获取TTL失败: %v", err)
			break
		}

		expired, err := lock.IsExpired(ctx)
		if err != nil {
			log.Printf("检查过期状态失败: %v", err)
			break
		}

		fmt.Printf("  监控点 %d: TTL=%v, Expired=%v\n", i+1, ttl, expired)

		if expired {
			fmt.Println("  ✓ 检测到锁已过期")
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	// 尝试释放已过期的锁（可能失败，这是正常的）
	if err := lock.Unlock(ctx); err != nil {
		fmt.Printf("  释放过期锁失败（符合预期）: %v\n", err)
	} else {
		fmt.Println("  ✓ 过期锁释放成功")
	}
}

// longLockHoldDemo 演示长时间持有锁的场景
func longLockHoldDemo(ctx context.Context, lockService lock.DistributedLock) {
	fmt.Println("\n--- 长时间持有锁场景 ---")
	const lockKey = "long-hold-demo"

	// 获取一个较长时间TTL的锁
	lock, err := lockService.Acquire(ctx, lockKey, 30*time.Second)
	if err != nil {
		log.Fatalf("获取锁失败: %v", err)
	}
	defer lock.Unlock(ctx)

	fmt.Printf("✓ 获取锁成功: %s\n", lock.Key())

	// 模拟长时间工作，定期检查和续约
	for i := 0; i < 5; i++ {
		// 检查TTL
		ttl, err := lock.TTL(ctx)
		if err != nil {
			log.Printf("获取TTL失败: %v", err)
			break
		}

		fmt.Printf("  工作阶段 %d: 剩余TTL=%v\n", i+1, ttl)

		// 如果TTL小于5秒，进行续约
		if ttl < 5*time.Second {
			fmt.Println("    TTL较低，进行续约...")
			success, err := lock.Renew(ctx)
			if err != nil {
				log.Printf("续约失败: %v", err)
				break
			}
			if success {
				fmt.Println("    ✓ 续约成功")
			} else {
				fmt.Println("    ✗ 续约失败")
				break
			}
		}

		// 模拟工作
		time.Sleep(2 * time.Second)
	}

	fmt.Println("  ✓ 长时间工作完成")
}

// contextCancellationDemo 演示上下文取消的处理
func contextCancellationDemo(ctx context.Context, lockService lock.DistributedLock) {
	fmt.Println("\n--- 上下文取消处理 ---")
	const lockKey = "context-cancellation-demo"

	// 首先获取一个锁
	lock1, err := lockService.Acquire(ctx, lockKey, 10*time.Second)
	if err != nil {
		log.Fatalf("首次获取锁失败: %v", err)
	}
	defer lock1.Unlock(ctx)

	fmt.Println("  已持有锁，创建可取消上下文尝试获取锁...")

	// 创建可取消的上下文
	cancelCtx, cancel := context.WithCancel(ctx)

	// 启动goroutine尝试获取锁
	done := make(chan struct{})
	go func() {
		lock2, err := lockService.Acquire(cancelCtx, lockKey, 10*time.Second)
		if err != nil {
			fmt.Printf("  ✓ 上下文取消生效: %v\n", err)
		} else {
			fmt.Println("  ✗ 上下文取消未生效")
			lock2.Unlock(ctx)
		}
		close(done)
	}()

	// 等待一下然后取消上下文
	time.Sleep(100 * time.Millisecond)
	cancel()

	// 等待操作完成
	select {
	case <-done:
		fmt.Println("  ✓ 上下文取消测试完成")
	case <-time.After(2 * time.Second):
		fmt.Println("  ✗ 上下文取消测试超时")
	}
}

// errorHandlingDemo 演示错误处理和恢复
func errorHandlingDemo(ctx context.Context, lockService lock.DistributedLock) {
	fmt.Println("\n--- 错误处理和恢复 ---")

	// 测试各种错误情况
	testCases := []struct {
		name    string
		key     string
		ttl     time.Duration
		wantErr bool
	}{
		{"空键", "", 5 * time.Second, true},
		{"零TTL", "zero-ttl", 0, true},
		{"负TTL", "negative-ttl", -time.Second, true},
		{"正常情况", "normal-lock", 5 * time.Second, false},
	}

	for _, tc := range testCases {
		fmt.Printf("  测试 %s: ", tc.name)

		_, err := lockService.Acquire(ctx, tc.key, tc.ttl)

		if tc.wantErr {
			if err != nil {
				fmt.Printf("✓ 预期错误: %v\n", err)
			} else {
				fmt.Println("✗ 应该出错但没有")
			}
		} else {
			if err != nil {
				fmt.Printf("✗ 意外错误: %v\n", err)
			} else {
				fmt.Println("✓ 成功获取锁")
			}
		}
	}
}
