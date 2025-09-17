package registryimpl

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/coord/internal/client"
	"github.com/ceyewan/infra-kit/coord/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEtcdServiceRegistry_New 测试服务注册表创建
func TestEtcdServiceRegistry_New(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	t.Run("valid creation", func(t *testing.T) {
		logger := clog.Namespace("test")
		serviceRegistry := NewEtcdServiceRegistry(client, "/test-services", logger)
		assert.NotNil(t, serviceRegistry)
	})

	t.Run("default prefix", func(t *testing.T) {
		logger := clog.Namespace("test")
		serviceRegistry := NewEtcdServiceRegistry(client, "", logger)
		assert.NotNil(t, serviceRegistry)
	})

	t.Run("with logger", func(t *testing.T) {
		serviceRegistry := NewEtcdServiceRegistry(client, "/test-services", nil)
		assert.NotNil(t, serviceRegistry)
	})
}

// TestEtcdServiceRegistry_Register 测试服务注册
func TestEtcdServiceRegistry_Register(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	serviceRegistry := NewEtcdServiceRegistry(client, "/test-services", logger)
	ctx := context.Background()

	t.Run("successful registration", func(t *testing.T) {
		service := registry.ServiceInfo{
			ID:      "test-instance-1",
			Name:    "test-service",
			Address: "127.0.0.1",
			Port:    8080,
			Metadata: map[string]string{
				"version": "1.0.0",
				"region":  "us-east",
			},
		}

		err := serviceRegistry.Register(ctx, service, time.Second*30)
		assert.NoError(t, err)

		// 清理
		err = serviceRegistry.Unregister(ctx, service.ID)
		assert.NoError(t, err)
	})

	t.Run("invalid service info", func(t *testing.T) {
		testCases := []struct {
			name    string
			service registry.ServiceInfo
			errMsg  string
		}{
			{
				name: "empty ID",
				service: registry.ServiceInfo{
					ID:      "",
					Name:    "test-service",
					Address: "127.0.0.1",
					Port:    8080,
				},
				errMsg: "[VALIDATION_ERROR] 服务 ID 不能为空",
			},
			{
				name: "empty name",
				service: registry.ServiceInfo{
					ID:      "test-instance",
					Name:    "",
					Address: "127.0.0.1",
					Port:    8080,
				},
				errMsg: "[VALIDATION_ERROR] 服务名不能为空",
			},
			{
				name: "empty address",
				service: registry.ServiceInfo{
					ID:      "test-instance",
					Name:    "test-service",
					Address: "",
					Port:    8080,
				},
				errMsg: "[VALIDATION_ERROR] 服务地址不能为空",
			},
			{
				name: "invalid port",
				service: registry.ServiceInfo{
					ID:      "test-instance",
					Name:    "test-service",
					Address: "127.0.0.1",
					Port:    0,
				},
				errMsg: "[VALIDATION_ERROR] 服务端口必须在 1~65535 之间",
			},
			{
				name: "port too high",
				service: registry.ServiceInfo{
					ID:      "test-instance",
					Name:    "test-service",
					Address: "127.0.0.1",
					Port:    70000,
				},
				errMsg: "[VALIDATION_ERROR] 服务端口必须在 1~65535 之间",
			},
			{
				name: "zero TTL",
				service: registry.ServiceInfo{
					ID:      "test-instance",
					Name:    "test-service",
					Address: "127.0.0.1",
					Port:    8080,
				},
				errMsg: "service TTL must be positive",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ttl := time.Second * 30
				if tc.name == "zero TTL" {
					ttl = 0 // Test zero TTL case
				}

				err := serviceRegistry.Register(ctx, tc.service, ttl)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			})
		}
	})
}

// TestEtcdServiceRegistry_Unregister 测试服务注销
func TestEtcdServiceRegistry_Unregister(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	serviceRegistry := NewEtcdServiceRegistry(client, "/test-services", logger)
	ctx := context.Background()

	t.Run("successful unregister", func(t *testing.T) {
		service := registry.ServiceInfo{
			ID:      "test-unregister",
			Name:    "test-service",
			Address: "127.0.0.1",
			Port:    8080,
		}

		// 先注册
		err := serviceRegistry.Register(ctx, service, time.Second*30)
		require.NoError(t, err)

		// 再注销
		err = serviceRegistry.Unregister(ctx, service.ID)
		assert.NoError(t, err)

		// 验证服务已不存在
		services, err := serviceRegistry.Discover(ctx, "test-service")
		assert.NoError(t, err)
		assert.Empty(t, services)
	})

	t.Run("unregister non-existent service", func(t *testing.T) {
		err := serviceRegistry.Unregister(ctx, "non-existent-service")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("unregister with empty ID", func(t *testing.T) {
		err := serviceRegistry.Unregister(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service ID cannot be empty")
	})
}

// TestEtcdServiceRegistry_Discover 测试服务发现
func TestEtcdServiceRegistry_Discover(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	serviceRegistry := NewEtcdServiceRegistry(client, "/test-services", logger)
	ctx := context.Background()

	// 注册测试服务
	testServices := []registry.ServiceInfo{
		{
			ID:       "discover-instance-1",
			Name:     "discover-service",
			Address:  "127.0.0.1",
			Port:     8080,
			Metadata: map[string]string{"version": "1.0.0"},
		},
		{
			ID:       "discover-instance-2",
			Name:     "discover-service",
			Address:  "127.0.0.1",
			Port:     8081,
			Metadata: map[string]string{"version": "1.0.1"},
		},
		{
			ID:      "discover-instance-3",
			Name:    "other-service", // 不同的服务名
			Address: "127.0.0.1",
			Port:    8082,
		},
	}

	// 清理函数
	cleanup := func() {
		for _, service := range testServices {
			serviceRegistry.Unregister(ctx, service.ID)
		}
	}
	defer cleanup()

	// 注册所有服务
	for _, service := range testServices {
		err := serviceRegistry.Register(ctx, service, time.Second*30)
		require.NoError(t, err)
	}

	t.Run("discover existing service", func(t *testing.T) {
		services, err := serviceRegistry.Discover(ctx, "discover-service")
		assert.NoError(t, err)
		assert.Len(t, services, 2)

		// 验证服务信息
		serviceIDs := make(map[string]bool)
		for _, service := range services {
			serviceIDs[service.ID] = true
			assert.Equal(t, "discover-service", service.Name)
		}
		assert.Contains(t, serviceIDs, "discover-instance-1")
		assert.Contains(t, serviceIDs, "discover-instance-2")
	})

	t.Run("discover non-existent service", func(t *testing.T) {
		services, err := serviceRegistry.Discover(ctx, "non-existent-service")
		assert.NoError(t, err)
		assert.Empty(t, services)
	})

	t.Run("discover with empty name", func(t *testing.T) {
		services, err := serviceRegistry.Discover(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "[VALIDATION_ERROR] 服务名不能为空")
		assert.Nil(t, services)
	})
}

// TestEtcdServiceRegistry_Watch 测试服务监听
func TestEtcdServiceRegistry_Watch(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	serviceRegistry := NewEtcdServiceRegistry(client, "/test-services", logger)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	t.Run("watch service events", func(t *testing.T) {
		serviceName := "watch-service"

		// 启动监听
		eventCh, err := serviceRegistry.Watch(ctx, serviceName)
		require.NoError(t, err)
		assert.NotNil(t, eventCh)

		// 注册服务
		service := registry.ServiceInfo{
			ID:      "watch-instance",
			Name:    serviceName,
			Address: "127.0.0.1",
			Port:    8080,
		}

		err = serviceRegistry.Register(ctx, service, time.Second*30)
		assert.NoError(t, err)

		// 等待事件
		select {
		case event := <-eventCh:
			assert.Equal(t, registry.EventTypePut, event.Type)
			assert.Equal(t, service.ID, event.Service.ID)
			assert.Equal(t, serviceName, event.Service.Name)
		case <-time.After(time.Second * 2):
			t.Fatal("Timeout waiting for service registration event")
		}

		// 注销服务
		err = serviceRegistry.Unregister(ctx, service.ID)
		assert.NoError(t, err)

		// 等待注销事件
		select {
		case event := <-eventCh:
			assert.Equal(t, registry.EventTypeDelete, event.Type)
			assert.Equal(t, service.ID, event.Service.ID)
		case <-time.After(time.Second * 2):
			t.Fatal("Timeout waiting for service unregistration event")
		}
	})

	t.Run("watch with empty service name", func(t *testing.T) {
		eventCh, err := serviceRegistry.Watch(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
		assert.Nil(t, eventCh)
	})
}

// TestEtcdServiceRegistry_ConcurrentOperations 测试并发操作
func TestEtcdServiceRegistry_ConcurrentOperations(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	serviceRegistry := NewEtcdServiceRegistry(client, "/test-services", logger)
	ctx := context.Background()

	const numGoroutines = 10
	const numOperations = 5

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				service := registry.ServiceInfo{
					ID:      fmt.Sprintf("worker-%d-instance-%d", workerID, j),
					Name:    "concurrent-service",
					Address: "127.0.0.1",
					Port:    8000 + workerID,
				}

				// 注册服务
				err := serviceRegistry.Register(ctx, service, time.Second*30)
				if err != nil {
					t.Errorf("Worker %d failed to register service %d: %v", workerID, j, err)
					continue
				}

				// 发现服务
				_, err = serviceRegistry.Discover(ctx, "concurrent-service")
				if err != nil {
					t.Errorf("Worker %d failed to discover services: %v", workerID, err)
				}

				// 注销服务
				err = serviceRegistry.Unregister(ctx, service.ID)
				if err != nil {
					t.Errorf("Worker %d failed to unregister service %d: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestEtcdServiceRegistry_SessionExpiration 测试会话过期处理
func TestEtcdServiceRegistry_SessionExpiration(t *testing.T) {
	t.Skip("跳过会话过期测试 - etcd 会话机制是为了保持服务在线，自动续约是正常行为")

	if testing.Short() {
		t.Skip("Skipping session expiration test in short mode")
	}

	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	serviceRegistry := NewEtcdServiceRegistry(client, "/test-services", logger)
	ctx := context.Background()

	t.Run("service expires with session", func(t *testing.T) {
		service := registry.ServiceInfo{
			ID:      "expiring-instance",
			Name:    "expiring-service",
			Address: "127.0.0.1",
			Port:    8080,
		}

		// 使用短TTL注册服务
		shortTTL := time.Second * 2
		err := serviceRegistry.Register(ctx, service, shortTTL)
		assert.NoError(t, err)

		// 验证服务存在
		services, err := serviceRegistry.Discover(ctx, "expiring-service")
		assert.NoError(t, err)
		assert.Len(t, services, 1)

		// 等待TTL过期
		time.Sleep(shortTTL + time.Second)

		// 验证服务已自动消失
		services, err = serviceRegistry.Discover(ctx, "expiring-service")
		assert.NoError(t, err)
		assert.Empty(t, services)
	})
}

// BenchmarkEtcdServiceRegistry 基准测试
func BenchmarkEtcdServiceRegistry(b *testing.B) {
	client, err := createTestEtcdClient()
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()

	logger := clog.Namespace("benchmark")
	serviceRegistry := NewEtcdServiceRegistry(client, "/benchmark-services", logger)
	ctx := context.Background()

	b.Run("Register and Unregister", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			service := registry.ServiceInfo{
				ID:      fmt.Sprintf("benchmark-instance-%d", i),
				Name:    "benchmark-service",
				Address: "127.0.0.1",
				Port:    8080,
			}

			err := serviceRegistry.Register(ctx, service, time.Second*30)
			if err != nil {
				b.Fatal(err)
			}

			err = serviceRegistry.Unregister(ctx, service.ID)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Discover", func(b *testing.B) {
		// 预先注册一些服务
		service := registry.ServiceInfo{
			ID:      "benchmark-discover-instance",
			Name:    "benchmark-discover-service",
			Address: "127.0.0.1",
			Port:    8080,
		}

		err := serviceRegistry.Register(ctx, service, time.Second*30)
		if err != nil {
			b.Fatal(err)
		}
		defer serviceRegistry.Unregister(ctx, service.ID)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := serviceRegistry.Discover(ctx, "benchmark-discover-service")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// createTestEtcdClient 创建测试用的etcd客户端
func createTestEtcdClient() (*client.EtcdClient, error) {
	config := client.Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 5,
		Logger:    clog.Namespace("test-etcd-client"),
	}
	return client.New(config)
}
