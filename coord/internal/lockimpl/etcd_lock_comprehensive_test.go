package lockimpl

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/coord/lock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEtcdLockFactory_Comprehensive 测试分布式锁的综合场景
func TestEtcdLockFactory_Comprehensive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive lock tests in short mode")
	}

	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	factory := NewEtcdLockFactory(client, "/test-locks", logger)
	ctx := context.Background()

	t.Run("lock lifecycle", func(t *testing.T) {
		lockKey := "lifecycle-test"

		// 获取锁
		lock, err := factory.Acquire(ctx, lockKey, time.Second*10)
		require.NoError(t, err)
		require.NotNil(t, lock)

		// 验证锁属性
		assert.NotEmpty(t, lock.Key())
		assert.Contains(t, lock.Key(), lockKey)

		// 检查TTL
		ttl, err := lock.TTL(ctx)
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, time.Second*10)

		// 释放锁
		err = lock.Unlock(ctx)
		assert.NoError(t, err)
	})

	t.Run("concurrent access", func(t *testing.T) {
		lockKey := "concurrent-test"
		const numWorkers = 5
		const numOperations = 3

		var wg sync.WaitGroup
		successCount := 0
		mu := sync.Mutex{}

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					// 尝试获取锁
					lock, err := factory.Acquire(ctx, lockKey, time.Second*5)
					if err != nil {
						t.Errorf("Worker %d failed to acquire lock: %v", workerID, err)
						continue
					}

					// 模拟临界区操作
					time.Sleep(time.Millisecond * 50)

					// 释放锁
					err = lock.Unlock(ctx)
					if err != nil {
						t.Errorf("Worker %d failed to unlock: %v", workerID, err)
						continue
					}

					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}(i)
		}

		wg.Wait()
		expectedSuccess := numWorkers * numOperations
		assert.Equal(t, expectedSuccess, successCount, "All lock operations should succeed")
	})

	t.Run("tryAcquire behavior", func(t *testing.T) {
		lockKey := "tryAcquire-test"

		// 先获取锁
		lock1, err := factory.Acquire(ctx, lockKey, time.Second*10)
		require.NoError(t, err)
		defer lock1.Unlock(ctx)

		// 尝试非阻塞获取锁 - 应该失败
		lock2, err := factory.TryAcquire(ctx, lockKey, time.Second*10)
		assert.Error(t, err)
		assert.Nil(t, lock2)
		assert.Contains(t, err.Error(), "lock is already held")

		// 释放第一把锁
		err = lock1.Unlock(ctx)
		assert.NoError(t, err)

		// 现在应该能够获取锁
		lock3, err := factory.TryAcquire(ctx, lockKey, time.Second*10)
		assert.NoError(t, err)
		assert.NotNil(t, lock3)
		lock3.Unlock(ctx)
	})

	t.Run("context cancellation", func(t *testing.T) {
		lockKey := "cancel-test"

		// 先获取锁
		lock1, err := factory.Acquire(ctx, lockKey, time.Second*10)
		require.NoError(t, err)
		defer lock1.Unlock(ctx)

		// 创建已取消的context
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		// 在已取消的context中尝试获取锁
		start := time.Now()
		lock2, err := factory.Acquire(cancelCtx, lockKey, time.Second*10)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Nil(t, lock2)
		assert.Less(t, duration, time.Second, "Should fail fast when context is cancelled")
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("multiple locks", func(t *testing.T) {
		// 测试同时持有多个不同的锁
		locks := make([]lock.Lock, 0)
		keys := []string{"multi-1", "multi-2", "multi-3"}

		// 获取多个锁
		for _, key := range keys {
			lock, err := factory.Acquire(ctx, key, time.Second*10)
			require.NoError(t, err)
			locks = append(locks, lock)
		}

		// 验证所有锁都有不同的key
		for i := 0; i < len(locks); i++ {
			for j := i + 1; j < len(locks); j++ {
				assert.NotEqual(t, locks[i].Key(), locks[j].Key())
			}
		}

		// 释放所有锁
		for _, lock := range locks {
			err := lock.Unlock(ctx)
			assert.NoError(t, err)
		}
	})

	t.Run("TTL monitoring", func(t *testing.T) {
		lockKey := "ttl-monitor-test"
		ttl := time.Second * 5

		// 获取锁
		lock, err := factory.Acquire(ctx, lockKey, ttl)
		require.NoError(t, err)
		defer lock.Unlock(ctx)

		// 监控TTL变化
		initialTTL, err := lock.TTL(ctx)
		require.NoError(t, err)
		assert.Greater(t, initialTTL, time.Duration(0))
		assert.LessOrEqual(t, initialTTL, ttl)

		// 由于etcd session自动续约，TTL不会显著减少
		time.Sleep(time.Second)

		currentTTL, err := lock.TTL(ctx)
		require.NoError(t, err)
		assert.Greater(t, currentTTL, time.Duration(0))
	})

	t.Run("error handling", func(t *testing.T) {
		testCases := []struct {
			name    string
			key     string
			ttl     time.Duration
			wantErr bool
			errMsg  string
		}{
			{
				name:    "empty key",
				key:     "",
				ttl:     time.Second * 10,
				wantErr: true,
				errMsg:  "key cannot be empty",
			},
			{
				name:    "zero TTL",
				key:     "zero-ttl",
				ttl:     0,
				wantErr: true,
				errMsg:  "ttl must be positive",
			},
			{
				name:    "negative TTL",
				key:     "negative-ttl",
				ttl:     -time.Second,
				wantErr: true,
				errMsg:  "ttl must be positive",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				lock, err := factory.Acquire(ctx, tc.key, tc.ttl)
				if tc.wantErr {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.errMsg)
					assert.Nil(t, lock)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, lock)
					lock.Unlock(ctx)
				}
			})
		}
	})
}

// BenchmarkEtcdLock_Comprehensive 基准测试
func BenchmarkEtcdLock_Comprehensive(b *testing.B) {
	client, err := createTestEtcdClient()
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()

	logger := clog.Namespace("benchmark")
	factory := NewEtcdLockFactory(client, "/benchmark-locks", logger)
	ctx := context.Background()

	b.Run("Acquire-Release", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := "bench-key"
			lock, err := factory.Acquire(ctx, key, time.Second*10)
			if err != nil {
				b.Fatal(err)
			}

			err = lock.Unlock(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("TryAcquire-Release", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := "bench-try-key"
			lock, err := factory.TryAcquire(ctx, key, time.Second*10)
			if err != nil {
				// In case of contention, just skip
				continue
			}

			err = lock.Unlock(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("TTL", func(b *testing.B) {
		lock, err := factory.Acquire(ctx, "bench-ttl-key", time.Second*10)
		if err != nil {
			b.Fatal(err)
		}
		defer lock.Unlock(ctx)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := lock.TTL(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Concurrent", func(b *testing.B) {
		const numWorkers = 10
		var wg sync.WaitGroup

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for j := 0; j < b.N/numWorkers; j++ {
					key := "concurrent-bench"
					lock, err := factory.Acquire(ctx, key, time.Second*5)
					if err != nil {
						continue
					}

					time.Sleep(time.Microsecond) // Minimal work
					lock.Unlock(ctx)
				}
			}()
		}

		wg.Wait()
	})
}

// TestEtcdLockFactory_EdgeCases 测试边界情况
func TestEtcdLockFactory_EdgeCases(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	factory := NewEtcdLockFactory(client, "/test-locks", logger)
	ctx := context.Background()

	t.Run("double unlock", func(t *testing.T) {
		lock, err := factory.Acquire(ctx, "double-unlock", time.Second*10)
		require.NoError(t, err)

		// 第一次解锁
		err = lock.Unlock(ctx)
		assert.NoError(t, err)

		// 第二次解锁 - 应该优雅处理
		err = lock.Unlock(ctx)
		// 当前实现可能会返回错误，这是可以接受的
		// 关键是不能panic
		assert.Error(t, err)
	})

	t.Run("get TTL after unlock", func(t *testing.T) {
		lock, err := factory.Acquire(ctx, "ttl-after-unlock", time.Second*10)
		require.NoError(t, err)

		err = lock.Unlock(ctx)
		assert.NoError(t, err)

		// 解锁后获取TTL - 应该返回错误
		_, err = lock.TTL(ctx)
		assert.Error(t, err)
	})

	t.Run("long lock hold", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping long lock hold test in short mode")
		}

		lock, err := factory.Acquire(ctx, "long-hold", time.Second*30)
		require.NoError(t, err)

		// 持有锁一段时间
		holdTime := time.Second * 3
		time.Sleep(holdTime)

		// 验证锁仍然有效
		ttl, err := lock.TTL(ctx)
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0))

		err = lock.Unlock(ctx)
		assert.NoError(t, err)
	})
}
