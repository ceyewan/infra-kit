package coord

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/coord/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain 设置测试环境
func TestMain(m *testing.M) {
	// 初始化日志（测试环境使用简单配置）
	config := clog.GetDefaultConfig("test")
	if err := clog.Init(context.Background(), config); err != nil {
		panic("Failed to initialize clog for tests: " + err.Error())
	}

	m.Run()
}

// TestNewCoordinator 测试创建coordinator
func TestNewCoordinator(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		options []Option
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  GetDefaultConfig("test"),
			options: []Option{WithLogger(clog.Namespace("test"))},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "cannot be nil",
		},
		{
			name: "empty endpoints",
			config: &Config{
				Endpoints: []string{},
			},
			wantErr: true,
			errMsg:  "endpoints cannot be empty",
		},
		{
			name: "invalid endpoints format",
			config: &Config{
				Endpoints: []string{"invalid-endpoint"},
			},
			wantErr: true,
			errMsg:  "invalid endpoint format",
		},
		{
			name: "invalid timeout",
			config: &Config{
				Endpoints:   []string{"localhost:2379"},
				DialTimeout: 0,
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			provider, err := New(ctx, tt.config, tt.options...)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, provider)

			// 清理资源
			err = provider.Close()
			assert.NoError(t, err)
		})
	}
}

// TestCoordinatorHealth 测试协调器健康检查功能
func TestCoordinatorHealth(t *testing.T) {
	ctx := context.Background()
	config := GetDefaultConfig("test")

	t.Run("healthy coordinator", func(t *testing.T) {
		provider, err := New(ctx, config, WithLogger(clog.Namespace("test")))
		require.NoError(t, err)
		defer provider.Close()

		err = provider.Health(ctx)
		assert.NoError(t, err)
	})

	t.Run("closed coordinator", func(t *testing.T) {
		provider, err := New(ctx, config, WithLogger(clog.Namespace("test")))
		require.NoError(t, err)

		// 关闭协调器
		err = provider.Close()
		assert.NoError(t, err)

		// 健康检查应该失败
		err = provider.Health(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "coordinator is closed")
	})

	t.Run("health check with timeout", func(t *testing.T) {
		provider, err := New(ctx, config, WithLogger(clog.Namespace("test")))
		require.NoError(t, err)
		defer provider.Close()

		// 使用超时上下文
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
		defer cancel()

		err = provider.Health(timeoutCtx)
		assert.NoError(t, err)
	})
}

// BenchmarkCoordinatorHealth 基准测试：健康检查性能
func BenchmarkCoordinatorHealth(b *testing.B) {
	ctx := context.Background()
	config := GetDefaultConfig("test")

	provider, err := New(ctx, config)
	if err != nil {
		b.Fatal(err)
	}
	defer provider.Close()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := provider.Health(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestCoordinatorServices 测试coordinator提供的服务
func TestCoordinatorServices(t *testing.T) {
	ctx := context.Background()
	config := GetDefaultConfig("test")
	config.Endpoints = []string{"localhost:2379"}

	provider, err := New(ctx, config)
	require.NoError(t, err)
	defer provider.Close()

	t.Run("Lock service", func(t *testing.T) {
		lockService := provider.Lock()
		assert.NotNil(t, lockService)

		// 验证是同一实例
		lockService2 := provider.Lock()
		assert.Same(t, lockService, lockService2)
	})

	t.Run("Registry service", func(t *testing.T) {
		registryService := provider.Registry()
		assert.NotNil(t, registryService)

		// 验证是同一实例
		registryService2 := provider.Registry()
		assert.Same(t, registryService, registryService2)
	})

	t.Run("Config service", func(t *testing.T) {
		configService := provider.Config()
		assert.NotNil(t, configService)

		// 验证是同一实例
		configService2 := provider.Config()
		assert.Same(t, configService, configService2)
	})
}

// TestValidateConfig 测试配置验证功能
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &Config{
				Endpoints:   []string{"localhost:2379"},
				DialTimeout: 5 * time.Second,
			},
			expectError: false,
		},
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "config cannot be nil",
		},
		{
			name: "empty endpoints",
			config: &Config{
				Endpoints:   []string{},
				DialTimeout: 5 * time.Second,
			},
			expectError: true,
			errorMsg:    "at least one endpoint must be specified",
		},
		{
			name: "empty endpoint in list",
			config: &Config{
				Endpoints:   []string{"localhost:2379", ""},
				DialTimeout: 5 * time.Second,
			},
			expectError: true,
			errorMsg:    "endpoint 1 cannot be empty",
		},
		{
			name: "zero dial timeout",
			config: &Config{
				Endpoints:   []string{"localhost:2379"},
				DialTimeout: 0,
			},
			expectError: true,
			errorMsg:    "dial timeout must be positive",
		},
		{
			name: "negative dial timeout",
			config: &Config{
				Endpoints:   []string{"localhost:2379"},
				DialTimeout: -1 * time.Second,
			},
			expectError: true,
			errorMsg:    "dial timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCoordinatorInstanceIDAllocator 测试实例ID分配器
func TestCoordinatorInstanceIDAllocator(t *testing.T) {
	ctx := context.Background()
	config := GetDefaultConfig("test")
	config.Endpoints = []string{"localhost:2379"}

	provider, err := New(ctx, config)
	require.NoError(t, err)
	defer provider.Close()

	t.Run("create allocator", func(t *testing.T) {
		allocator, err := provider.InstanceIDAllocator("test-service", 10)
		require.NoError(t, err)
		assert.NotNil(t, allocator)
	})

	t.Run("allocator caching", func(t *testing.T) {
		// 多次调用应该返回同一实例
		allocator1, err := provider.InstanceIDAllocator("cached-service", 5)
		require.NoError(t, err)

		allocator2, err := provider.InstanceIDAllocator("cached-service", 5)
		require.NoError(t, err)

		assert.Same(t, allocator1, allocator2)
	})

	t.Run("different parameters create different allocators", func(t *testing.T) {
		allocator1, err := provider.InstanceIDAllocator("service-A", 10)
		require.NoError(t, err)

		allocator2, err := provider.InstanceIDAllocator("service-A", 20) // different maxID
		require.NoError(t, err)

		allocator3, err := provider.InstanceIDAllocator("service-B", 10) // different serviceName
		require.NoError(t, err)

		assert.NotSame(t, allocator1, allocator2)
		assert.NotSame(t, allocator1, allocator3)
		assert.NotSame(t, allocator2, allocator3)
	})

	t.Run("invalid parameters", func(t *testing.T) {
		// 空服务名
		allocator, err := provider.InstanceIDAllocator("", 10)
		assert.Error(t, err)
		assert.Nil(t, allocator)

		// 无效的maxID
		allocator, err = provider.InstanceIDAllocator("test-service", 0)
		assert.Error(t, err)
		assert.Nil(t, allocator)

		allocator, err = provider.InstanceIDAllocator("test-service", -1)
		assert.Error(t, err)
		assert.Nil(t, allocator)
	})
}

// TestCoordinatorClose 测试coordinator的关闭功能
func TestCoordinatorClose(t *testing.T) {
	ctx := context.Background()
	config := GetDefaultConfig("test")
	config.Endpoints = []string{"localhost:2379"}

	provider, err := New(ctx, config)
	require.NoError(t, err)

	t.Run("close once", func(t *testing.T) {
		err := provider.Close()
		assert.NoError(t, err)
	})

	t.Run("close multiple times", func(t *testing.T) {
		// 多次关闭应该是安全的
		for i := 0; i < 3; i++ {
			err := provider.Close()
			assert.NoError(t, err)
		}
	})

	t.Run("services after close", func(t *testing.T) {
		// 关闭后仍然可以获取服务（但不保证可用）
		lockService := provider.Lock()
		assert.NotNil(t, lockService)

		registryService := provider.Registry()
		assert.NotNil(t, registryService)

		configService := provider.Config()
		assert.NotNil(t, configService)
	})
}

// TestCoordinatorConcurrency 测试coordinator的并发安全性
func TestCoordinatorConcurrency(t *testing.T) {
	ctx := context.Background()
	config := GetDefaultConfig("test")
	config.Endpoints = []string{"localhost:2379"}

	provider, err := New(ctx, config)
	require.NoError(t, err)
	defer provider.Close()

	const numGoroutines = 10
	const numOperations = 50

	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines*numOperations)

	// 并发创建分配器
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				serviceName := "concurrent-service"
				maxID := 10

				allocator, err := provider.InstanceIDAllocator(serviceName, maxID)
				if err != nil {
					errs <- err
					continue
				}

				// 验证分配器功能
				allocatedID, err := allocator.AcquireID(ctx)
				if err != nil {
					errs <- err
					continue
				}

				assert.Greater(t, allocatedID.ID(), 0)
				assert.LessOrEqual(t, allocatedID.ID(), maxID)

				// 释放ID
				err = allocatedID.Close(ctx)
				if err != nil {
					errs <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	// 检查是否有错误
	for err := range errs {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}

// TestCoordinatorWithRealEtcd 测试与真实etcd的集成
func TestCoordinatorWithRealEtcd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real etcd integration test in short mode")
	}

	ctx := context.Background()
	config := GetDefaultConfig("test")
	config.Endpoints = []string{"localhost:2379", "localhost:12379", "localhost:22379"}

	provider, err := New(ctx, config)
	require.NoError(t, err)
	defer provider.Close()

	t.Run("real etcd connection", func(t *testing.T) {
		// 测试分布式锁
		lockService := provider.Lock()
		lock, err := lockService.Acquire(ctx, "test-lock", time.Second*10)
		require.NoError(t, err)

		ttl, err := lock.TTL(ctx)
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0))

		err = lock.Unlock(ctx)
		assert.NoError(t, err)
	})

	t.Run("real service registry", func(t *testing.T) {
		registryService := provider.Registry()

		service := registry.ServiceInfo{
			ID:      "test-instance",
			Name:    "test-service",
			Address: "127.0.0.1",
			Port:    8080,
		}

		err := registryService.Register(ctx, service, time.Second*30)
		assert.NoError(t, err)

		services, err := registryService.Discover(ctx, "test-service")
		assert.NoError(t, err)
		assert.Len(t, services, 1)
		assert.Equal(t, service.ID, services[0].ID)

		err = registryService.Unregister(ctx, service.ID)
		assert.NoError(t, err)
	})

	t.Run("real config center", func(t *testing.T) {
		configService := provider.Config()

		testConfig := map[string]string{"key": "value"}
		err := configService.Set(ctx, "test/config", testConfig)
		assert.NoError(t, err)

		var result map[string]string
		err = configService.Get(ctx, "test/config", &result)
		assert.NoError(t, err)
		assert.Equal(t, testConfig, result)
	})
}

// TestCoordinatorEdgeCases 测试边界情况
func TestCoordinatorEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("context cancellation", func(t *testing.T) {
		config := GetDefaultConfig("test")
		config.Endpoints = []string{"localhost:9999"} // 无效地址，连接会超时

		cancelCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
		defer cancel()

		start := time.Now()
		provider, err := New(cancelCtx, config)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Less(t, duration, time.Second) // 应该快速失败，而不是等待默认超时
	})

	t.Run("logger injection", func(t *testing.T) {
		config := GetDefaultConfig("test")
		config.Endpoints = []string{"localhost:2379"}

		customLogger := clog.Namespace("custom-test")
		provider, err := New(ctx, config, WithLogger(customLogger))
		require.NoError(t, err)
		defer provider.Close()

		// 验证logger被正确注入（通过创建分配器来间接验证）
		allocator, err := provider.InstanceIDAllocator("logger-test", 5)
		assert.NoError(t, err)
		assert.NotNil(t, allocator)
	})
}

// BenchmarkCoordinator 基准测试
func BenchmarkCoordinator(b *testing.B) {
	ctx := context.Background()
	config := GetDefaultConfig("benchmark")
	config.Endpoints = []string{"localhost:2379"}

	b.Run("Create and Close", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			provider, err := New(ctx, config)
			if err != nil {
				b.Fatal(err)
			}
			provider.Close()
		}
	})

	b.Run("Service Access", func(b *testing.B) {
		provider, err := New(ctx, config)
		if err != nil {
			b.Fatal(err)
		}
		defer provider.Close()

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = provider.Lock()
			_ = provider.Registry()
			_ = provider.Config()
		}
	})

	b.Run("Allocator Creation", func(b *testing.B) {
		provider, err := New(ctx, config)
		if err != nil {
			b.Fatal(err)
		}
		defer provider.Close()

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := provider.InstanceIDAllocator("bench-service", 100)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
