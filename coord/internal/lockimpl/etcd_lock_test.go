package lockimpl

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/coord/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEtcdLockFactory_New 测试锁工厂创建
func TestEtcdLockFactory_New(t *testing.T) {
	// 创建etcd客户端
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := createTestLogger()
	factory := NewEtcdLockFactory(client, "/test-locks", logger)

	t.Run("valid factory creation", func(t *testing.T) {
		assert.NotNil(t, factory)
	})

	t.Run("default prefix", func(t *testing.T) {
		factory2 := NewEtcdLockFactory(client, "", logger)
		assert.NotNil(t, factory2)
	})

	t.Run("with logger", func(t *testing.T) {
		factory3 := NewEtcdLockFactory(client, "/test-locks", nil)
		assert.NotNil(t, factory3)
	})
}

// TestEtcdLockFactory_Acquire 测试阻塞式获取锁
func TestEtcdLockFactory_Acquire(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := createTestLogger()
	factory := NewEtcdLockFactory(client, "/test-locks", logger)
	ctx := context.Background()

	t.Run("successful acquire", func(t *testing.T) {
		lock, err := factory.Acquire(ctx, "test-key", time.Second*10)
		require.NoError(t, err)
		require.NotNil(t, lock)

		// 验证锁属性
		assert.NotEmpty(t, lock.Key())
		assert.Contains(t, lock.Key(), "test-key")

		// 释放锁
		err = lock.Unlock(ctx)
		assert.NoError(t, err)
	})

	t.Run("empty key", func(t *testing.T) {
		lock, err := factory.Acquire(ctx, "", time.Second*10)
		assert.Error(t, err)
		assert.Nil(t, lock)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})

	t.Run("zero TTL", func(t *testing.T) {
		lock, err := factory.Acquire(ctx, "test-key-zero-ttl", 0)
		assert.Error(t, err)
		assert.Nil(t, lock)
		assert.Contains(t, err.Error(), "ttl must be positive")
	})

	t.Run("negative TTL", func(t *testing.T) {
		lock, err := factory.Acquire(ctx, "test-key-negative-ttl", -time.Second)
		assert.Error(t, err)
		assert.Nil(t, lock)
		assert.Contains(t, err.Error(), "ttl must be positive")
	})
}

// TestEtcdLockFactory_TryAcquire 测试非阻塞获取锁
func TestEtcdLockFactory_TryAcquire(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := createTestLogger()
	factory := NewEtcdLockFactory(client, "/test-locks", logger)
	ctx := context.Background()

	t.Run("successful try acquire", func(t *testing.T) {
		lock, err := factory.TryAcquire(ctx, "try-test-key", time.Second*10)
		require.NoError(t, err)
		require.NotNil(t, lock)

		err = lock.Unlock(ctx)
		assert.NoError(t, err)
	})

	t.Run("lock already held", func(t *testing.T) {
		// 先获取锁
		lock1, err := factory.Acquire(ctx, "contention-key", time.Second*10)
		require.NoError(t, err)
		defer lock1.Unlock(ctx)

		// 尝试获取同一把锁
		lock2, err := factory.TryAcquire(ctx, "contention-key", time.Second*10)
		assert.Error(t, err)
		assert.Nil(t, lock2)
		assert.Contains(t, err.Error(), "lock is already held")
	})
}

// TestEtcdLock_ConcurrentAccess 测试并发锁访问
func TestEtcdLock_ConcurrentAccess(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	factory := NewEtcdLockFactory(client, "/test-locks", logger)
	ctx := context.Background()

	const numGoroutines = 10
	const numOperations = 5

	var wg sync.WaitGroup
	successCount := 0
	mu := &sync.Mutex{}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				lock, err := factory.Acquire(ctx, "concurrent-key", time.Second*5)
				if err != nil {
					t.Errorf("Goroutine %d failed to acquire lock: %v", id, err)
					continue
				}

				// 持有锁做一些工作
				time.Sleep(time.Millisecond * 10)

				err = lock.Unlock(ctx)
				if err != nil {
					t.Errorf("Goroutine %d failed to unlock: %v", id, err)
					continue
				}

				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	expectedSuccess := numGoroutines * numOperations
	assert.Equal(t, expectedSuccess, successCount, "All operations should succeed")
}

// TestEtcdLock_TTL 测试锁的TTL功能
func TestEtcdLock_TTL(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	factory := NewEtcdLockFactory(client, "/test-locks", logger)
	ctx := context.Background()

	t.Run("get TTL", func(t *testing.T) {
		lock, err := factory.Acquire(ctx, "ttl-test-key", time.Second*30)
		require.NoError(t, err)
		defer lock.Unlock(ctx)

		ttl, err := lock.TTL(ctx)
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, time.Second*30)
	})

	t.Run("TTL decreases", func(t *testing.T) {
		t.Skip("etcd sessions automatically renew leases, TTL does not decrease over time")

		// 以下是原始测试代码，保留供参考
		/*
			lock, err := factory.Acquire(ctx, "ttl-decrease-key", time.Second*5)
			require.NoError(t, err)
			defer lock.Unlock(ctx)

			ttl1, err := lock.TTL(ctx)
			require.NoError(t, err)

			time.Sleep(time.Second)

			ttl2, err := lock.TTL(ctx)
			require.NoError(t, err)

			assert.Less(t, ttl2, ttl1, "TTL should decrease over time")
		*/
	})
}

// TestEtcdLock_Reentrancy 测试锁的重入性
func TestEtcdLock_Reentrancy(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	factory := NewEtcdLockFactory(client, "/test-locks", logger)
	ctx := context.Background()

	t.Run("non-reentrant by default", func(t *testing.T) {
		// 获取第一把锁
		lock1, err := factory.Acquire(ctx, "reentrant-key", time.Second*10)
		require.NoError(t, err)
		defer lock1.Unlock(ctx)

		// 尝试在同一context中再次获取同一把锁
		lock2, err := factory.TryAcquire(ctx, "reentrant-key", time.Second*10)
		assert.Error(t, err)
		assert.Nil(t, lock2)
		assert.Contains(t, err.Error(), "lock is already held")
	})
}

// TestEtcdLock_ContextCancellation 测试上下文取消
func TestEtcdLock_ContextCancellation(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	factory := NewEtcdLockFactory(client, "/test-locks", logger)

	t.Run("acquire with cancelled context", func(t *testing.T) {
		// 先获取一把锁
		lock1, err := factory.Acquire(context.Background(), "cancel-key", time.Second*10)
		require.NoError(t, err)
		defer lock1.Unlock(context.Background())

		// 创建已取消的context
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()

		// 尝试在已取消的context中获取锁
		start := time.Now()
		lock2, err := factory.Acquire(cancelCtx, "cancel-key", time.Second*10)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Nil(t, lock2)
		assert.Less(t, duration, time.Second, "Should fail fast when context is cancelled")
	})
}

// TestEtcdLock_MultipleKeys 测试多个不同的锁
func TestEtcdLock_MultipleKeys(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	factory := NewEtcdLockFactory(client, "/test-locks", logger)
	ctx := context.Background()

	t.Run("multiple different locks", func(t *testing.T) {
		// 同时获取多个不同的锁
		lock1, err := factory.Acquire(ctx, "multi-key-1", time.Second*10)
		require.NoError(t, err)
		defer lock1.Unlock(ctx)

		lock2, err := factory.Acquire(ctx, "multi-key-2", time.Second*10)
		require.NoError(t, err)
		defer lock2.Unlock(ctx)

		lock3, err := factory.Acquire(ctx, "multi-key-3", time.Second*10)
		require.NoError(t, err)
		defer lock3.Unlock(ctx)

		// 验证所有锁都有不同的key
		assert.NotEqual(t, lock1.Key(), lock2.Key())
		assert.NotEqual(t, lock2.Key(), lock3.Key())
		assert.NotEqual(t, lock1.Key(), lock3.Key())
	})
}

// TestEtcdLock_LongHold 测试长时间持有锁
func TestEtcdLock_LongHold(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long hold test in short mode")
	}

	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	factory := NewEtcdLockFactory(client, "/test-locks", logger)
	ctx := context.Background()

	t.Run("long lock hold", func(t *testing.T) {
		lock, err := factory.Acquire(ctx, "long-hold-key", time.Second*30)
		require.NoError(t, err)
		defer lock.Unlock(ctx)

		// 持有锁一段时间
		holdTime := time.Second * 5
		time.Sleep(holdTime)

		// 检查锁仍然有效
		ttl, err := lock.TTL(ctx)
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0))
		assert.Less(t, ttl, time.Second*30-holdTime+time.Second) // 允许一些误差
	})
}

// TestEtcdLock_AutoRelease 测试锁的自动释放
// 注意：etcd的session会自动续约，所以这个测试被跳过
func TestEtcdLock_AutoRelease(t *testing.T) {
	t.Skip("etcd sessions automatically renew leases, TTL auto-release is not applicable")
}

// BenchmarkEtcdLock 基准测试
func BenchmarkEtcdLock(b *testing.B) {
	client, err := createTestEtcdClient()
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()

	logger := createTestLogger().Namespace("benchmark")
	factory := NewEtcdLockFactory(client, "/benchmark-locks", logger)
	ctx := context.Background()

	b.Run("Acquire and Release", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := "benchmark-key"
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

	b.Run("TryAcquire and Release", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := "benchmark-try-key"
			lock, err := factory.TryAcquire(ctx, key, time.Second*10)
			if err != nil {
				// Expected to fail sometimes due to contention
				continue
			}

			err = lock.Unlock(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("TTL", func(b *testing.B) {
		b.ReportAllocs()

		lock, err := factory.Acquire(ctx, "benchmark-ttl-key", time.Second*10)
		if err != nil {
			b.Fatal(err)
		}
		defer lock.Unlock(ctx)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := lock.TTL(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// createTestEtcdClient 创建测试用的etcd客户端
func createTestEtcdClient() (*client.EtcdClient, error) {
	// 创建一个 WARN 级别的 logger 用于测试
	testLogger, _ := clog.New(context.Background(), &clog.Config{
		Level:       "warn",
		Format:      "console",
		Output:      "stdout",
		AddSource:   false,
		EnableColor: false,
	})

	config := client.Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 5,
		Logger:    testLogger.Namespace("test-etcd-client"),
	}
	return client.New(config)
}

// createTestLogger 创建测试用的WARN级别logger
func createTestLogger() clog.Logger {
	testLogger, _ := clog.New(context.Background(), &clog.Config{
		Level:       "warn",
		Format:      "console",
		Output:      "stdout",
		AddSource:   false,
		EnableColor: false,
	})
	return testLogger.Namespace("test")
}
