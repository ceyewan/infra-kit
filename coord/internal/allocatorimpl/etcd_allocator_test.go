package allocatorimpl

import (
	"context"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/clog"
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

// TestEtcdInstanceIDAllocator_New 测试ID分配器创建
func TestEtcdInstanceIDAllocator_New(t *testing.T) {
	// 创建测试etcd客户端
	etcdClient, err := createTestEtcdClient()
	require.NoError(t, err)
	defer etcdClient.Close()

	logger := clog.Namespace("test")

	t.Run("valid creation", func(t *testing.T) {
		allocator, err := NewEtcdInstanceIDAllocator(etcdClient, "test-service", 10, logger)
		require.NoError(t, err)
		require.NotNil(t, allocator)

		// 强制类型转换以调用Close方法
		etcdAllocator := allocator.(*etcdInstanceIDAllocator)
		err = etcdAllocator.Close()
		require.NoError(t, err)
	})

	t.Run("empty service name", func(t *testing.T) {
		allocator, err := NewEtcdInstanceIDAllocator(etcdClient, "", 10, logger)
		require.Error(t, err)
		require.Nil(t, allocator)
		require.Contains(t, err.Error(), "service name cannot be empty")
	})

	t.Run("zero max ID", func(t *testing.T) {
		allocator, err := NewEtcdInstanceIDAllocator(etcdClient, "test-service", 0, logger)
		require.Error(t, err)
		require.Nil(t, allocator)
		require.Contains(t, err.Error(), "max ID must be positive")
	})

	t.Run("negative max ID", func(t *testing.T) {
		allocator, err := NewEtcdInstanceIDAllocator(etcdClient, "test-service", -1, logger)
		require.Error(t, err)
		require.Nil(t, allocator)
		require.Contains(t, err.Error(), "max ID must be positive")
	})
}

// TestEtcdInstanceIDAllocator_AcquireID 测试ID获取
func TestEtcdInstanceIDAllocator_AcquireID(t *testing.T) {
	// 创建测试etcd客户端
	etcdClient, err := createTestEtcdClient()
	require.NoError(t, err)
	defer etcdClient.Close()

	logger := clog.Namespace("test")
	ctx := context.Background()

	allocator, err := NewEtcdInstanceIDAllocator(etcdClient, "test-service", 5, logger)
	require.NoError(t, err)
	require.NotNil(t, allocator)

	// 清理资源
	defer func() {
		etcdAllocator := allocator.(*etcdInstanceIDAllocator)
		err = etcdAllocator.Close()
		require.NoError(t, err)
	}()

	t.Run("acquire single ID", func(t *testing.T) {
		allocatedID, err := allocator.AcquireID(ctx)
		require.NoError(t, err)
		require.NotNil(t, allocatedID)

		// 检查ID值
		id := allocatedID.ID()
		require.Greater(t, id, 0)
		require.LessOrEqual(t, id, 5)

		// 释放ID
		err = allocatedID.Close(ctx)
		require.NoError(t, err)
	})
}

// TestEtcdInstanceIDAllocator_Health 测试健康检查
func TestEtcdInstanceIDAllocator_Health(t *testing.T) {
	// 创建测试etcd客户端
	etcdClient, err := createTestEtcdClient()
	require.NoError(t, err)
	defer etcdClient.Close()

	logger := clog.Namespace("test")
	ctx := context.Background()

	allocator, err := NewEtcdInstanceIDAllocator(etcdClient, "test-service", 5, logger)
	require.NoError(t, err)
	require.NotNil(t, allocator)

	// 清理资源
	defer func() {
		etcdAllocator := allocator.(*etcdInstanceIDAllocator)
		err = etcdAllocator.Close()
		require.NoError(t, err)
	}()

	t.Run("health check success", func(t *testing.T) {
		// 强制类型转换以访问Health方法
		etcdAllocator := allocator.(*etcdInstanceIDAllocator)
		err = etcdAllocator.Health(ctx)
		require.NoError(t, err)
	})

	t.Run("health check after close", func(t *testing.T) {
		// 强制类型转换以访问Health方法
		etcdAllocator := allocator.(*etcdInstanceIDAllocator)

		// 关闭分配器
		err = etcdAllocator.Close()
		require.NoError(t, err)

		// 健康检查应该失败
		err = etcdAllocator.Health(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "allocator is closed")
	})
}
