package coord

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/coord/lock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLockInterfaceIntegration 测试 DistributedLock 接口的集成使用
func TestLockInterfaceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 创建协调器
	cfg := GetDefaultConfig("test")
	provider, err := New(context.Background(), cfg)
	require.NoError(t, err)
	defer provider.Close()

	lockService := provider.Lock()
	ctx := context.Background()

	t.Run("interface compliance", func(t *testing.T) {
		// 验证接口实现
		var _ lock.DistributedLock = lockService

		// 获取锁
		acquiredLock, err := lockService.Acquire(ctx, "interface-test", time.Second*5)
		require.NoError(t, err)

		// 验证 Lock 接口
		var _ lock.Lock = acquiredLock

		// 测试所有接口方法
		assert.NotEmpty(t, acquiredLock.Key())

		ttl, err := acquiredLock.TTL(ctx)
		assert.NoError(t, err)
		assert.True(t, ttl > 0)

		// 测试新增方法
		expired, err := acquiredLock.IsExpired(ctx)
		assert.NoError(t, err)
		assert.False(t, expired)

		success, err := acquiredLock.Renew(ctx)
		assert.NoError(t, err)
		assert.True(t, success)

		// 释放锁
		err = acquiredLock.Unlock(ctx)
		assert.NoError(t, err)
	})

	t.Run("blocking vs non-blocking", func(t *testing.T) {
		lockKey := "blocking-test"

		// 首先阻塞获取锁
		lock1, err := lockService.Acquire(ctx, lockKey, time.Second*3)
		require.NoError(t, err)
		defer lock1.Unlock(ctx)

		// 尝试非阻塞获取，应该失败
		_, err = lockService.TryAcquire(ctx, lockKey, time.Second*3)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already held")

		// 释放第一个锁
		err = lock1.Unlock(ctx)
		assert.NoError(t, err)

		// 现在应该可以成功获取
		lock2, err := lockService.TryAcquire(ctx, lockKey, time.Second*3)
		require.NoError(t, err)
		lock2.Unlock(ctx)
	})

	t.Run("error handling", func(t *testing.T) {
		// 测试空键
		_, err := lockService.Acquire(ctx, "", time.Second*5)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")

		// 测试零 TTL
		_, err = lockService.Acquire(ctx, "zero-ttl-test", 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")

		// 测试负 TTL
		_, err = lockService.Acquire(ctx, "negative-ttl-test", -time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")
	})

	t.Run("concurrent access", func(t *testing.T) {
		const numWorkers = 5
		const lockKey = "concurrent-interface-test"

		var wg sync.WaitGroup
		results := make(chan bool, numWorkers)

		// 启动多个 worker 并发尝试获取锁
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				// 尝试获取锁
				l, err := lockService.TryAcquire(ctx, lockKey, time.Second*2)
				if err != nil {
					results <- false
					return
				}

				// 成功获取锁，工作后释放
				time.Sleep(time.Millisecond * 100)
				l.Unlock(ctx)
				results <- true
			}(i)
		}

		// 等待所有 worker 完成
		go func() {
			wg.Wait()
			close(results)
		}()

		// 统计结果
		successCount := 0
		for success := range results {
			if success {
				successCount++
			}
		}

		// 应该只有一个 worker 成功获取锁
		assert.Equal(t, 1, successCount, "exactly one worker should acquire the lock")
	})

	t.Run("TTL and expiration", func(t *testing.T) {
		lockKey := "ttl-test"

		// 获取一个短 TTL 的锁
		l, err := lockService.Acquire(ctx, lockKey, time.Second*2)
		require.NoError(t, err)

		// 检查初始 TTL
		initialTTL, err := l.TTL(ctx)
		require.NoError(t, err)
		assert.True(t, initialTTL > 0, "initial TTL should be positive")
		assert.True(t, initialTTL <= time.Second*2, "initial TTL should not exceed requested TTL")

		// 检查是否过期
		expired, err := l.IsExpired(ctx)
		require.NoError(t, err)
		assert.False(t, expired, "lock should not be expired immediately")

		// 手动续约
		success, err := l.Renew(ctx)
		require.NoError(t, err)
		assert.True(t, success, "lock renewal should succeed")

		// 检查续约后的 TTL
		renewedTTL, err := l.TTL(ctx)
		require.NoError(t, err)
		assert.True(t, renewedTTL > 0, "renewed TTL should be positive")

		// 释放锁
		err = l.Unlock(ctx)
		require.NoError(t, err)
	})
}

// BenchmarkLockInterface 性能基准测试
func BenchmarkLockInterface(b *testing.B) {
	// 创建协调器
	cfg := GetDefaultConfig("benchmark")
	provider, err := New(context.Background(), cfg)
	require.NoError(b, err)
	defer provider.Close()

	lockService := provider.Lock()
	ctx := context.Background()

	b.Run("acquire_and_release", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lockKey := fmt.Sprintf("benchmark-lock-%d", i)

			// 获取锁
			l, err := lockService.Acquire(ctx, lockKey, time.Second)
			require.NoError(b, err)

			// 模拟工作
			time.Sleep(time.Microsecond)

			// 释放锁
			err = l.Unlock(ctx)
			require.NoError(b, err)
		}
	})

	b.Run("try_acquire", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lockKey := fmt.Sprintf("benchmark-try-%d", i)

			// 尝试获取锁
			l, err := lockService.TryAcquire(ctx, lockKey, time.Second)
			require.NoError(b, err)

			// 释放锁
			err = l.Unlock(ctx)
			require.NoError(b, err)
		}
	})

	b.Run("concurrent_locking", func(b *testing.B) {
		const numWorkers = 4
		const lockKey = "concurrent-benchmark-lock"

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup

			// 启动多个 worker 并发获取同一把锁
			for j := 0; j < numWorkers; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					// 阻塞获取锁
					l, err := lockService.Acquire(ctx, lockKey, time.Second)
					if err != nil {
						return
					}

					// 模拟工作
					time.Sleep(time.Microsecond)

					// 释放锁
					l.Unlock(ctx)
				}()
			}

			wg.Wait()
		}
	})

	b.Run("ttl_operations", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lockKey := fmt.Sprintf("benchmark-ttl-%d", i)

			// 获取锁
			l, err := lockService.Acquire(ctx, lockKey, time.Second*5)
			require.NoError(b, err)

			// 执行 TTL 操作
			_, _ = l.TTL(ctx)
			_, _ = l.IsExpired(ctx)
			_, _ = l.Renew(ctx)

			// 释放锁
			_ = l.Unlock(ctx)
		}
	})
}

// TestLockInterfaceWithCancellation 测试上下文取消功能
func TestLockInterfaceWithCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := GetDefaultConfig("test")
	provider, err := New(context.Background(), cfg)
	require.NoError(t, err)
	defer provider.Close()

	lockService := provider.Lock()
	lockKey := "cancellation-test"

	t.Run("cancel during blocking acquire", func(t *testing.T) {
		// 先获取锁
		lock1, err := lockService.Acquire(context.Background(), lockKey, time.Second*5)
		require.NoError(t, err)
		defer lock1.Unlock(context.Background())

		// 创建可取消的上下文
		ctx, cancel := context.WithCancel(context.Background())

		// 启动 goroutine 尝试获取锁
		done := make(chan struct{})
		go func() {
			_, err := lockService.Acquire(ctx, lockKey, time.Second*5)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "canceled")
			close(done)
		}()

		// 等待一下然后取消上下文
		time.Sleep(time.Millisecond * 100)
		cancel()

		// 等待操作完成
		select {
		case <-done:
			// 成功取消
		case <-time.After(time.Second):
			t.Fatal("context cancellation did not work")
		}
	})
}
