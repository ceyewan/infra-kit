package configimpl

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/coord/config"
	"github.com/ceyewan/infra-kit/coord/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEtcdConfigCenter_New 测试配置中心创建
func TestEtcdConfigCenter_New(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	t.Run("valid creation", func(t *testing.T) {
		logger := clog.Namespace("test")
		configCenter := NewEtcdConfigCenter(client, "/test-config", logger)
		assert.NotNil(t, configCenter)
	})

	t.Run("default prefix", func(t *testing.T) {
		logger := clog.Namespace("test")
		configCenter := NewEtcdConfigCenter(client, "", logger)
		assert.NotNil(t, configCenter)
	})

	t.Run("with logger", func(t *testing.T) {
		configCenter := NewEtcdConfigCenter(client, "/test-config", nil)
		assert.NotNil(t, configCenter)
	})
}

// TestEtcdConfigCenter_GetSet 测试配置的获取和设置
func TestEtcdConfigCenter_GetSet(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	configCenter := NewEtcdConfigCenter(client, "/test-config", logger)
	ctx := context.Background()

	t.Run("string config", func(t *testing.T) {
		key := "test-string"
		value := "hello world"

		// 设置配置
		err := configCenter.Set(ctx, key, value)
		assert.NoError(t, err)

		// 获取配置
		var result string
		err = configCenter.Get(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)

		// 清理
		err = configCenter.Delete(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("map config", func(t *testing.T) {
		key := "test-map"
		value := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		}

		// 设置配置
		err := configCenter.Set(ctx, key, value)
		assert.NoError(t, err)

		// 获取配置
		var result map[string]interface{}
		err = configCenter.Get(ctx, key, &result)
		assert.NoError(t, err)
		// JSON 数字会默认解析为 float64，这是正常的
		assert.Equal(t, value["key1"], result["key1"])
		assert.Equal(t, float64(value["key2"].(int)), result["key2"])
		assert.Equal(t, value["key3"], result["key3"])

		// 清理
		err = configCenter.Delete(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("struct config", func(t *testing.T) {
		type TestConfig struct {
			Name    string `json:"name"`
			Version int    `json:"version"`
			Enabled bool   `json:"enabled"`
		}

		key := "test-struct"
		value := TestConfig{
			Name:    "test-app",
			Version: 123,
			Enabled: true,
		}

		// 设置配置
		err := configCenter.Set(ctx, key, value)
		assert.NoError(t, err)

		// 获取配置
		var result TestConfig
		err = configCenter.Get(ctx, key, &result)
		assert.NoError(t, err)
		assert.Equal(t, value, result)

		// 清理
		err = configCenter.Delete(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("get non-existent config", func(t *testing.T) {
		var result string
		err := configCenter.Get(ctx, "non-existent-key", &result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("set with empty key", func(t *testing.T) {
		err := configCenter.Set(ctx, "", "value")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})

	t.Run("get with empty key", func(t *testing.T) {
		var result string
		err := configCenter.Get(ctx, "", &result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})

	t.Run("get with nil pointer", func(t *testing.T) {
		err := configCenter.Get(ctx, "test-key", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a non-nil pointer")
	})
}

// TestEtcdConfigCenter_CAS 测试CAS操作
func TestEtcdConfigCenter_CAS(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	configCenter := NewEtcdConfigCenter(client, "/test-config", logger)
	ctx := context.Background()

	t.Run("successful CAS", func(t *testing.T) {
		key := "cas-test"
		initialValue := "initial"
		newValue := "updated"

		// 设置初始值
		err := configCenter.Set(ctx, key, initialValue)
		assert.NoError(t, err)

		// 获取当前值和版本
		var currentValue string
		version, err := configCenter.GetWithVersion(ctx, key, &currentValue)
		assert.NoError(t, err)
		assert.Equal(t, initialValue, currentValue)
		assert.Greater(t, version, int64(0))

		// 执行CAS更新
		err = configCenter.CompareAndSet(ctx, key, newValue, version)
		assert.NoError(t, err)

		// 验证更新成功
		var updatedValue string
		err = configCenter.Get(ctx, key, &updatedValue)
		assert.NoError(t, err)
		assert.Equal(t, newValue, updatedValue)

		// 清理
		err = configCenter.Delete(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("CAS with wrong version", func(t *testing.T) {
		key := "cas-wrong-version"
		initialValue := "initial"

		// 设置初始值
		err := configCenter.Set(ctx, key, initialValue)
		assert.NoError(t, err)

		// 使用错误的版本号尝试CAS
		err = configCenter.CompareAndSet(ctx, key, "updated", 99999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "version mismatch")

		// 验证值未被修改
		var currentValue string
		err = configCenter.Get(ctx, key, &currentValue)
		assert.NoError(t, err)
		assert.Equal(t, initialValue, currentValue)

		// 清理
		err = configCenter.Delete(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("CAS with non-existent key", func(t *testing.T) {
		err := configCenter.CompareAndSet(ctx, "non-existent-key", "value", 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "version mismatch")
	})
}

// TestEtcdConfigCenter_Delete 测试配置删除
func TestEtcdConfigCenter_Delete(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	configCenter := NewEtcdConfigCenter(client, "/test-config", logger)
	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		key := "delete-test"
		value := "to-be-deleted"

		// 设置配置
		err := configCenter.Set(ctx, key, value)
		assert.NoError(t, err)

		// 删除配置
		err = configCenter.Delete(ctx, key)
		assert.NoError(t, err)

		// 验证配置已删除
		var result string
		err = configCenter.Get(ctx, key, &result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("delete non-existent config", func(t *testing.T) {
		err := configCenter.Delete(ctx, "non-existent-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found for deletion")
	})

	t.Run("delete with empty key", func(t *testing.T) {
		err := configCenter.Delete(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
	})
}

// TestEtcdConfigCenter_Watch 测试配置监听
func TestEtcdConfigCenter_Watch(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	configCenter := NewEtcdConfigCenter(client, "/test-config", logger)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	t.Run("watch single key", func(t *testing.T) {
		key := "watch-single-test"
		var targetValue string

		// 启动监听
		watcher, err := configCenter.Watch(ctx, key, &targetValue)
		require.NoError(t, err)
		defer watcher.Close()

		// 设置配置
		err = configCenter.Set(ctx, key, "initial-value")
		assert.NoError(t, err)

		// 等待事件
		select {
		case event := <-watcher.Chan():
			assert.Equal(t, config.EventTypePut, event.Type)
			assert.Equal(t, key, event.Key)
			assert.Equal(t, "initial-value", event.Value)
		case <-time.After(time.Second * 2):
			t.Fatal("Timeout waiting for config event")
		}

		// 更新配置
		err = configCenter.Set(ctx, key, "updated-value")
		assert.NoError(t, err)

		// 等待更新事件
		select {
		case event := <-watcher.Chan():
			assert.Equal(t, config.EventTypePut, event.Type)
			assert.Equal(t, key, event.Key)
			assert.Equal(t, "updated-value", event.Value)
		case <-time.After(time.Second * 2):
			t.Fatal("Timeout waiting for config update event")
		}

		// 删除配置
		err = configCenter.Delete(ctx, key)
		assert.NoError(t, err)

		// 等待删除事件
		select {
		case event := <-watcher.Chan():
			assert.Equal(t, config.EventTypeDelete, event.Type)
			assert.Equal(t, key, event.Key)
		case <-time.After(time.Second * 2):
			t.Fatal("Timeout waiting for config delete event")
		}
	})

	t.Run("watch prefix", func(t *testing.T) {
		prefix := "watch-prefix"
		var targetValue string

		// 启动前缀监听
		watcher, err := configCenter.WatchPrefix(ctx, prefix, &targetValue)
		require.NoError(t, err)
		defer watcher.Close()

		// 设置多个配置
		keys := []string{prefix + "/key1", prefix + "/key2", prefix + "/sub/key3"}
		for i, key := range keys {
			err = configCenter.Set(ctx, key, "value"+string(rune(i+'1')))
			assert.NoError(t, err)

			// 等待事件
			select {
			case event := <-watcher.Chan():
				assert.Equal(t, config.EventTypePut, event.Type)
				assert.Equal(t, key, event.Key)
			case <-time.After(time.Second * 2):
				t.Fatal("Timeout waiting for prefix config event")
			}
		}
	})

	t.Run("watch with empty key", func(t *testing.T) {
		var targetValue string
		watcher, err := configCenter.Watch(ctx, "", &targetValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key cannot be empty")
		assert.Nil(t, watcher)
	})

	t.Run("watch with nil pointer", func(t *testing.T) {
		watcher, err := configCenter.Watch(ctx, "test-key", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a non-nil pointer")
		assert.Nil(t, watcher)
	})
}

// TestEtcdConfigCenter_List 测试配置列表
func TestEtcdConfigCenter_List(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	configCenter := NewEtcdConfigCenter(client, "/test-config", logger)
	ctx := context.Background()

	// 设置测试配置
	testConfigs := map[string]string{
		"list-test/key1":     "value1",
		"list-test/key2":     "value2",
		"list-test/sub/key3": "value3",
		"other-key":          "other-value",
	}

	// 清理函数
	cleanup := func() {
		for key := range testConfigs {
			configCenter.Delete(ctx, key)
		}
	}
	defer cleanup()

	// 设置所有配置
	for key, value := range testConfigs {
		err := configCenter.Set(ctx, key, value)
		assert.NoError(t, err)
	}

	t.Run("list with prefix", func(t *testing.T) {
		keys, err := configCenter.List(ctx, "list-test")
		assert.NoError(t, err)
		assert.Contains(t, keys, "list-test/key1")
		assert.Contains(t, keys, "list-test/key2")
		assert.Contains(t, keys, "list-test/sub/key3")
		assert.NotContains(t, keys, "other-key")
	})

	t.Run("list all", func(t *testing.T) {
		keys, err := configCenter.List(ctx, "")
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(keys), len(testConfigs))
	})

	t.Run("list non-existent prefix", func(t *testing.T) {
		keys, err := configCenter.List(ctx, "non-existent")
		assert.NoError(t, err)
		assert.Empty(t, keys)
	})
}

// TestEtcdConfigCenter_ConcurrentOperations 测试并发操作
func TestEtcdConfigCenter_ConcurrentOperations(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	configCenter := NewEtcdConfigCenter(client, "/test-config", logger)
	ctx := context.Background()

	const numGoroutines = 10
	const numOperations = 20

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("concurrent-key-%d-%d", workerID, j)
				value := fmt.Sprintf("value-%d-%d", workerID, j)

				// 设置配置
				err := configCenter.Set(ctx, key, value)
				if err != nil {
					t.Errorf("Worker %d failed to set config %d: %v", workerID, j, err)
					continue
				}

				// 获取配置
				var result string
				err = configCenter.Get(ctx, key, &result)
				if err != nil {
					t.Errorf("Worker %d failed to get config %d: %v", workerID, j, err)
					continue
				}

				assert.Equal(t, value, result)

				// 删除配置
				err = configCenter.Delete(ctx, key)
				if err != nil {
					t.Errorf("Worker %d failed to delete config %d: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestEtcdConfigCenter_TypeSafety 测试类型安全
func TestEtcdConfigCenter_TypeSafety(t *testing.T) {
	client, err := createTestEtcdClient()
	require.NoError(t, err)
	defer client.Close()

	logger := clog.Namespace("test")
	configCenter := NewEtcdConfigCenter(client, "/test-config", logger)
	ctx := context.Background()

	t.Run("type mismatch", func(t *testing.T) {
		key := "type-mismatch-test"

		// 设置字符串值
		err := configCenter.Set(ctx, key, "string-value")
		assert.NoError(t, err)

		// 尝试获取为整数
		var intValue int
		err = configCenter.Get(ctx, key, &intValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not valid JSON")

		// 清理
		err = configCenter.Delete(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("interface{} target", func(t *testing.T) {
		key := "interface-target-test"

		// 设置复杂值
		complexValue := map[string]interface{}{
			"nested": map[string]interface{}{
				"key": "value",
			},
			"array": []int{1, 2, 3},
		}

		err := configCenter.Set(ctx, key, complexValue)
		assert.NoError(t, err)

		// 获取为interface{}
		var result interface{}
		err = configCenter.Get(ctx, key, &result)
		assert.NoError(t, err)

		// 验证结构
		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, resultMap, "nested")
		assert.Contains(t, resultMap, "array")

		// 清理
		err = configCenter.Delete(ctx, key)
		assert.NoError(t, err)
	})
}

// BenchmarkEtcdConfigCenter 基准测试
func BenchmarkEtcdConfigCenter(b *testing.B) {
	client, err := createTestEtcdClient()
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()

	logger := clog.Namespace("benchmark")
	configCenter := NewEtcdConfigCenter(client, "/benchmark-config", logger)
	ctx := context.Background()

	b.Run("Set and Get", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark-key-%d", i)
			value := fmt.Sprintf("benchmark-value-%d", i)

			err := configCenter.Set(ctx, key, value)
			if err != nil {
				b.Fatal(err)
			}

			var result string
			err = configCenter.Get(ctx, key, &result)
			if err != nil {
				b.Fatal(err)
			}

			err = configCenter.Delete(ctx, key)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("CAS operations", func(b *testing.B) {
		key := "benchmark-cas-key"
		value := "initial"

		// 设置初始值
		err := configCenter.Set(ctx, key, value)
		if err != nil {
			b.Fatal(err)
		}
		defer configCenter.Delete(ctx, key)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// 获取当前版本
			var currentValue string
			version, err := configCenter.GetWithVersion(ctx, key, &currentValue)
			if err != nil {
				b.Fatal(err)
			}

			// 执行CAS更新
			newValue := fmt.Sprintf("updated-value-%d", i)
			err = configCenter.CompareAndSet(ctx, key, newValue, version)
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
