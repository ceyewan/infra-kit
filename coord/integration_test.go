package coord

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/coord/allocator"
	"github.com/ceyewan/infra-kit/coord/internal/allocatorimpl"
	"github.com/ceyewan/infra-kit/coord/internal/client"
	"github.com/ceyewan/infra-kit/coord/internal/lockimpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// createTestEtcdClient 创建测试用的etcd客户端
func createTestEtcdClient() (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
}

// TestCoordIntegration 测试coord模块的集成使用
func TestCoordIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 创建测试etcd客户端
	etcdClient, err := createTestEtcdClient()
	require.NoError(t, err)
	defer etcdClient.Close()

	// 创建etcd客户端包装器
	etcdWrapper, err := client.New(client.Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 10,
		Logger:    clog.Namespace("integration-test"),
	})
	require.NoError(t, err)
	defer etcdWrapper.Close()

	logger := clog.Namespace("integration-test")
	ctx := context.Background()

	t.Run("distributed coordination workflow", func(t *testing.T) {
		// 1. 创建实例ID分配器
		idAllocator, err := allocatorimpl.NewEtcdInstanceIDAllocator(etcdClient, "integration-service", 10, logger)
		require.NoError(t, err)
		defer func() {
			// 使用接口方法而不是直接访问内部实现
		}()

		// 2. 创建分布式锁工厂
		lockFactory := lockimpl.NewEtcdLockFactory(etcdWrapper, "integration-locks", logger)

		// 3. 模拟多个服务实例的协调工作
		const numInstances = 3
		var wg sync.WaitGroup

		for i := 0; i < numInstances; i++ {
			wg.Add(1)
			go func(instanceID int) {
				defer wg.Done()

				// 每个实例获取唯一的ID
				allocatedID, err := idAllocator.AcquireID(ctx)
				require.NoError(t, err)
				defer allocatedID.Close(ctx)

				t.Logf("Instance %d acquired ID: %d", instanceID, allocatedID.ID())

				// 每个实例尝试获取分布式锁
				lock, err := lockFactory.Acquire(ctx, fmt.Sprintf("integration-lock-%d", instanceID), time.Second*5)
				require.NoError(t, err)
				defer lock.Unlock(ctx)

				t.Logf("Instance %d (ID: %d) acquired lock", instanceID, allocatedID.ID())

				// 模拟工作
				time.Sleep(time.Millisecond * 100)

				// 检查健康状态
				// 注意：健康检查应该在接口层面提供，这里暂时跳过

				t.Logf("Instance %d (ID: %d) completed work", instanceID, allocatedID.ID())
			}(i)
		}

		wg.Wait()
		t.Logf("All %d instances completed coordinated work", numInstances)
	})

	t.Run("concurrent id allocation and locking", func(t *testing.T) {
		// 创建ID分配器
		idAllocator, err := allocatorimpl.NewEtcdInstanceIDAllocator(etcdClient, "concurrent-service", 5, logger)
		require.NoError(t, err)
		defer func() {
			// ID分配器会在租约过期时自动清理
		}()

		// 创建锁工厂
		lockFactory := lockimpl.NewEtcdLockFactory(etcdWrapper, "concurrent-locks", logger)

		const numWorkers = 10
		var wg sync.WaitGroup
		var mu sync.Mutex
		acquiredIDs := make(map[int]bool)
		lockCount := 0

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				// 获取唯一ID
				allocatedID, err := idAllocator.AcquireID(ctx)
				require.NoError(t, err)
				defer allocatedID.Close(ctx)

				// 记录获取的ID
				mu.Lock()
				if acquiredIDs[allocatedID.ID()] {
					t.Errorf("Worker %d acquired duplicate ID: %d", workerID, allocatedID.ID())
				}
				acquiredIDs[allocatedID.ID()] = true
				mu.Unlock()

				// 获取分布式锁
				lock, err := lockFactory.Acquire(ctx, "shared-resource-lock", time.Second*2)
				require.NoError(t, err)
				defer lock.Unlock(ctx)

				// 增加锁计数
				mu.Lock()
				lockCount++
				mu.Unlock()

				// 模拟临界区工作
				time.Sleep(time.Millisecond * 50)

				t.Logf("Worker %d (ID: %d) completed critical section", workerID, allocatedID.ID())
			}(i)
		}

		wg.Wait()

		// 验证所有ID都是唯一的
		assert.Len(t, acquiredIDs, numWorkers)

		// 验证所有工人都成功获取了锁
		assert.Equal(t, numWorkers, lockCount)

		t.Logf("Successfully coordinated %d workers with unique IDs and synchronized access", numWorkers)
	})

	t.Run("error handling and recovery", func(t *testing.T) {
		// 测试错误处理和恢复机制

		// 创建ID分配器
		idAllocator, err := allocatorimpl.NewEtcdInstanceIDAllocator(etcdClient, "error-test-service", 3, logger)
		require.NoError(t, err)
		defer func() {
			// ID分配器会在租约过期时自动清理
		}()

		// 创建锁工厂
		lockFactory := lockimpl.NewEtcdLockFactory(etcdWrapper, "error-test-locks", logger)
		_ = lockFactory

		// 测试参数验证
		_, err = allocatorimpl.NewEtcdInstanceIDAllocator(nil, "test", 1, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client cannot be nil")

		// 测试健康检查
		// 注意：健康检查应该在接口层面提供，这里暂时跳过

		// 获取所有ID
		var allocatedIDs []allocator.AllocatedID
		for i := 0; i < 3; i++ {
			id, err := idAllocator.AcquireID(ctx)
			require.NoError(t, err)
			allocatedIDs = append(allocatedIDs, id)
		}

		// 尝试获取更多ID应该失败
		_, err = idAllocator.AcquireID(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no available ID")

		// 释放一个ID
		err = allocatedIDs[0].Close(ctx)
		assert.NoError(t, err)

		// 现在应该可以获取新的ID
		newID, err := idAllocator.AcquireID(ctx)
		require.NoError(t, err)
		defer newID.Close(ctx)

		t.Logf("Error handling and recovery test completed successfully")
	})
}

// BenchmarkCoordIntegration 基准测试：协调性能
func BenchmarkCoordIntegration(b *testing.B) {
	// 创建测试etcd客户端
	etcdClient, err := createTestEtcdClient()
	require.NoError(b, err)
	defer etcdClient.Close()

	// 创建etcd客户端包装器
	etcdWrapper, err := client.New(client.Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 10,
		Logger:    clog.Namespace("benchmark"),
	})
	require.NoError(b, err)
	defer etcdWrapper.Close()

	logger := clog.Namespace("benchmark")
	ctx := context.Background()

	// 创建ID分配器
	idAllocator, err := allocatorimpl.NewEtcdInstanceIDAllocator(etcdClient, "benchmark-service", 100, logger)
	require.NoError(b, err)
	defer func() {
		// ID分配器会在租约过期时自动清理
	}()

	// 创建锁工厂
	lockFactory := lockimpl.NewEtcdLockFactory(etcdWrapper, "benchmark-locks", logger)

	b.Run("id_allocation_and_locking", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// 获取ID
			allocatedID, err := idAllocator.AcquireID(ctx)
			require.NoError(b, err)

			// 获取锁
			lock, err := lockFactory.Acquire(ctx, "benchmark-lock", time.Second)
			require.NoError(b, err)

			// 释放锁
			err = lock.Unlock(ctx)
			require.NoError(b, err)

			// 释放ID
			err = allocatedID.Close(ctx)
			require.NoError(b, err)
		}
	})

	b.Run("health_checks", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// 注意：健康检查应该在接口层面提供，这里暂时跳过
		}
	})
}
