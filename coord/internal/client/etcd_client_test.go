package client

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// TestEtcdClient_New 测试etcd客户端创建
func TestEtcdClient_New(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := Config{
			Endpoints: []string{"localhost:2379"},
			Timeout:   time.Second * 5,
			Logger:    clog.Namespace("test"),
		}

		client, err := New(config)
		require.NoError(t, err)
		assert.NotNil(t, client)

		err = client.Close()
		assert.NoError(t, err)
	})

	t.Run("multiple endpoints", func(t *testing.T) {
		config := Config{
			Endpoints: []string{"localhost:2379", "localhost:12379", "localhost:22379"},
			Timeout:   time.Second * 5,
			Logger:    clog.Namespace("test"),
		}

		client, err := New(config)
		require.NoError(t, err)
		assert.NotNil(t, client)

		err = client.Close()
		assert.NoError(t, err)
	})

	t.Run("with authentication", func(t *testing.T) {
		config := Config{
			Endpoints: []string{"localhost:2379"},
			Username:  "test-user",
			Password:  "test-pass",
			Timeout:   time.Second * 5,
			Logger:    clog.Namespace("test"),
		}

		client, err := New(config)
		// 即使认证失败，也应该返回客户端，连接时才会出错
		require.NoError(t, err)
		assert.NotNil(t, client)

		err = client.Close()
		assert.NoError(t, err)
	})

	t.Run("with retry config", func(t *testing.T) {
		config := Config{
			Endpoints: []string{"localhost:2379"},
			Timeout:   time.Second * 5,
			RetryConfig: &RetryConfig{
				MaxAttempts:  3,
				InitialDelay: time.Millisecond * 100,
				MaxDelay:     time.Second * 2,
				Multiplier:   2.0,
			},
			Logger: clog.Namespace("test"),
		}

		client, err := New(config)
		require.NoError(t, err)
		assert.NotNil(t, client)

		err = client.Close()
		assert.NoError(t, err)
	})
}

// TestEtcdClient_Validation 测试配置验证
func TestEtcdClient_Validation(t *testing.T) {
	testCases := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "empty endpoints",
			config: Config{
				Endpoints: []string{},
				Timeout:   time.Second * 5,
			},
			wantErr: true,
			errMsg:  "endpoints cannot be empty",
		},
		{
			name: "nil endpoints",
			config: Config{
				Endpoints: nil,
				Timeout:   time.Second * 5,
			},
			wantErr: true,
			errMsg:  "endpoints cannot be empty",
		},
		{
			name: "invalid endpoint format - no port",
			config: Config{
				Endpoints: []string{"localhost"},
				Timeout:   time.Second * 5,
			},
			wantErr: true,
			errMsg:  "invalid endpoint format",
		},
		{
			name: "invalid endpoint format - invalid port",
			config: Config{
				Endpoints: []string{"localhost:99999"},
				Timeout:   time.Second * 5,
			},
			wantErr: true,
			errMsg:  "invalid endpoint format",
		},
		{
			name: "zero timeout",
			config: Config{
				Endpoints: []string{"localhost:2379"},
				Timeout:   0,
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "negative timeout",
			config: Config{
				Endpoints: []string{"localhost:2379"},
				Timeout:   -time.Second,
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "invalid retry config - negative attempts",
			config: Config{
				Endpoints: []string{"localhost:2379"},
				Timeout:   time.Second * 5,
				RetryConfig: &RetryConfig{
					MaxAttempts: -1,
				},
			},
			wantErr: true,
			errMsg:  "max_attempts cannot be negative",
		},
		{
			name: "invalid retry config - zero delay",
			config: Config{
				Endpoints: []string{"localhost:2379"},
				Timeout:   time.Second * 5,
				RetryConfig: &RetryConfig{
					MaxAttempts:  3,
					InitialDelay: 0,
				},
			},
			wantErr: true,
			errMsg:  "initial_delay must be positive",
		},
		{
			name: "invalid retry config - small multiplier",
			config: Config{
				Endpoints: []string{"localhost:2379"},
				Timeout:   time.Second * 5,
				RetryConfig: &RetryConfig{
					MaxAttempts:  3,
					InitialDelay: time.Millisecond * 100,
					Multiplier:   0.5,
				},
			},
			wantErr: true,
			errMsg:  "max_delay must be positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.config)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestEtcdClient_BasicOperations 测试基本操作
func TestEtcdClient_BasicOperations(t *testing.T) {
	config := Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 5,
		Logger:    clog.Namespace("test"),
	}

	client, err := New(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	t.Run("put and get", func(t *testing.T) {
		key := "test-basic-put-get"
		value := "test-value"

		// 存储值
		resp, err := client.Put(ctx, key, value)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// 获取值
		getResp, err := client.Get(ctx, key)
		assert.NoError(t, err)
		assert.NotNil(t, getResp)
		assert.Len(t, getResp.Kvs, 1)
		assert.Equal(t, value, string(getResp.Kvs[0].Value))

		// 清理
		_, err = client.Delete(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("put and delete", func(t *testing.T) {
		key := "test-basic-put-delete"
		value := "test-value"

		// 存储值
		_, err := client.Put(ctx, key, value)
		assert.NoError(t, err)

		// 删除值
		resp, err := client.Delete(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), resp.Deleted)

		// 验证已删除
		getResp, err := client.Get(ctx, key)
		assert.NoError(t, err)
		assert.Len(t, getResp.Kvs, 0)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		resp, err := client.Get(ctx, "non-existent-key")
		assert.NoError(t, err)
		assert.Len(t, resp.Kvs, 0)
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		resp, err := client.Delete(ctx, "non-existent-key")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), resp.Deleted)
	})
}

// TestEtcdClient_Ping 测试连接检查
func TestEtcdClient_Ping(t *testing.T) {
	config := Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 5,
		Logger:    clog.Namespace("test"),
	}

	client, err := New(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	t.Run("successful ping", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("ping with timeout", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
		defer cancel()

		err := client.Ping(timeoutCtx)
		// 应该很快完成
		assert.NoError(t, err)
	})
}

// TestEtcdClient_LeaseOperations 测试租约操作
func TestEtcdClient_LeaseOperations(t *testing.T) {
	config := Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 5,
		Logger:    clog.Namespace("test"),
	}

	client, err := New(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	t.Run("grant and revoke lease", func(t *testing.T) {
		// 创建租约
		resp, err := client.Grant(ctx, 5) // 5秒TTL
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Greater(t, resp.ID, int64(0))

		leaseID := resp.ID

		// 撤销租约
		_, err = client.Revoke(ctx, leaseID)
		assert.NoError(t, err)
	})

	t.Run("revoke non-existent lease", func(t *testing.T) {
		_, err := client.Revoke(ctx, 999999) // 不存在的租约ID
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("keep alive lease", func(t *testing.T) {
		// 创建租约
		resp, err := client.Grant(ctx, 10) // 10秒TTL
		assert.NoError(t, err)
		leaseID := resp.ID

		// 启动KeepAlive
		kaCh, err := client.KeepAlive(ctx, leaseID)
		assert.NoError(t, err)
		assert.NotNil(t, kaCh)

		// 等待几个心跳
		select {
		case kaResp := <-kaCh:
			assert.Equal(t, leaseID, kaResp.ID)
		case <-time.After(time.Second * 2):
			t.Fatal("Timeout waiting for keepalive response")
		}

		// 清理
		_, err = client.Revoke(ctx, leaseID)
		assert.NoError(t, err)
	})
}

// TestEtcdClient_RetryMechanism 测试重试机制
func TestEtcdClient_RetryMechanism(t *testing.T) {
	t.Run("with retry config", func(t *testing.T) {
		config := Config{
			Endpoints: []string{"localhost:2379"},
			Timeout:   time.Second * 5,
			RetryConfig: &RetryConfig{
				MaxAttempts:  3,
				InitialDelay: time.Millisecond * 100,
				MaxDelay:     time.Second * 2,
				Multiplier:   2.0,
			},
			Logger: clog.Namespace("test"),
		}

		client, err := New(config)
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()

		// 正常操作应该成功
		_, err = client.Put(ctx, "retry-test-key", "retry-test-value")
		assert.NoError(t, err)

		// 清理
		_, err = client.Delete(ctx, "retry-test-key")
		assert.NoError(t, err)
	})

	t.Run("without retry config", func(t *testing.T) {
		config := Config{
			Endpoints: []string{"localhost:2379"},
			Timeout:   time.Second * 5,
			Logger:    clog.Namespace("test"),
		}

		client, err := New(config)
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()

		// 正常操作应该成功
		_, err = client.Put(ctx, "no-retry-test-key", "no-retry-test-value")
		assert.NoError(t, err)

		// 清理
		_, err = client.Delete(ctx, "no-retry-test-key")
		assert.NoError(t, err)
	})
}

// TestEtcdClient_ErrorHandling 测试错误处理
func TestEtcdClient_ErrorHandling(t *testing.T) {
	t.Run("connection error", func(t *testing.T) {
		config := Config{
			Endpoints: []string{"localhost:9999"}, // 无效端口
			Timeout:   time.Second * 2,
			Logger:    clog.Namespace("test"),
		}

		_, err := New(config)
		// 创建客户端应该失败，因为连接测试会立即进行
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect to etcd")
	})

	t.Run("context cancellation", func(t *testing.T) {
		config := Config{
			Endpoints: []string{"localhost:2379"},
			Timeout:   time.Second * 5,
			Logger:    clog.Namespace("test"),
		}

		client, err := New(config)
		require.NoError(t, err)
		defer client.Close()

		// 创建已取消的context
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()

		// 在已取消的context上操作应该快速失败
		_, err = client.Put(cancelCtx, "cancel-test-key", "cancel-test-value")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

// TestEtcdClient_ConcurrentOperations 测试并发操作
func TestEtcdClient_ConcurrentOperations(t *testing.T) {
	config := Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 5,
		Logger:    clog.Namespace("test"),
	}

	client, err := New(config)
	require.NoError(t, err)
	defer client.Close()

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
				value := fmt.Sprintf("concurrent-value-%d-%d", workerID, j)

				// 并发Put操作
				_, err := client.Put(ctx, key, value)
				if err != nil {
					t.Errorf("Worker %d failed to put key %d: %v", workerID, j, err)
					continue
				}

				// 并发Get操作
				resp, err := client.Get(ctx, key)
				if err != nil {
					t.Errorf("Worker %d failed to get key %d: %v", workerID, j, err)
					continue
				}

				if len(resp.Kvs) == 0 {
					t.Errorf("Worker %d got empty result for key %d", workerID, j)
					continue
				}

				assert.Equal(t, value, string(resp.Kvs[0].Value))

				// 并发Delete操作
				_, err = client.Delete(ctx, key)
				if err != nil {
					t.Errorf("Worker %d failed to delete key %d: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestEtcdClient_Txn 测试事务操作
func TestEtcdClient_Txn(t *testing.T) {
	config := Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 5,
		Logger:    clog.Namespace("test"),
	}

	client, err := New(config)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	t.Run("successful transaction", func(t *testing.T) {
		key := "txn-test-key"
		value := "txn-test-value"

		// 事务：如果key不存在，则创建
		txn := client.Txn(ctx)
		resp, err := txn.
			If(clientv3.Compare(clientv3.ModRevision(key), "=", 0)).
			Then(clientv3.OpPut(key, value)).
			Commit()
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)

		// 验证值已设置
		getResp, err := client.Get(ctx, key)
		assert.NoError(t, err)
		assert.Len(t, getResp.Kvs, 1)
		assert.Equal(t, value, string(getResp.Kvs[0].Value))

		// 清理
		_, err = client.Delete(ctx, key)
		assert.NoError(t, err)
	})

	t.Run("failed transaction", func(t *testing.T) {
		key := "txn-fail-test-key"

		// 先设置值
		_, err := client.Put(ctx, key, "initial-value")
		assert.NoError(t, err)

		// 事务：如果key不存在，则创建（应该失败）
		txn := client.Txn(ctx)
		resp, err := txn.
			If(clientv3.Compare(clientv3.ModRevision(key), "=", 0)).
			Then(clientv3.OpPut(key, "new-value")).
			Commit()
		assert.NoError(t, err)
		assert.False(t, resp.Succeeded)

		// 验证值未被修改
		getResp, err := client.Get(ctx, key)
		assert.NoError(t, err)
		assert.Len(t, getResp.Kvs, 1)
		assert.Equal(t, "initial-value", string(getResp.Kvs[0].Value))

		// 清理
		_, err = client.Delete(ctx, key)
		assert.NoError(t, err)
	})
}

// BenchmarkEtcdClient 基准测试
func BenchmarkEtcdClient(b *testing.B) {
	config := Config{
		Endpoints: []string{"localhost:2379"},
		Timeout:   time.Second * 5,
		Logger:    clog.Namespace("benchmark"),
	}

	client, err := New(config)
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	b.Run("Put and Get", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark-key-%d", i)
			value := fmt.Sprintf("benchmark-value-%d", i)

			_, err := client.Put(ctx, key, value)
			if err != nil {
				b.Fatal(err)
			}

			_, err = client.Get(ctx, key)
			if err != nil {
				b.Fatal(err)
			}

			_, err = client.Delete(ctx, key)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Put only", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark-put-key-%d", i)
			value := fmt.Sprintf("benchmark-put-value-%d", i)

			_, err := client.Put(ctx, key, value)
			if err != nil {
				b.Fatal(err)
			}
		}

		// 清理
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark-put-key-%d", i)
			client.Delete(ctx, key)
		}
	})

	b.Run("Get only", func(b *testing.B) {
		// 预先设置数据
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark-get-key-%d", i)
			value := fmt.Sprintf("benchmark-get-value-%d", i)
			client.Put(ctx, key, value)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark-get-key-%d", i)
			_, err := client.Get(ctx, key)
			if err != nil {
				b.Fatal(err)
			}
		}

		// 清理
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("benchmark-get-key-%d", i)
			client.Delete(ctx, key)
		}
	})

	b.Run("Ping", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := client.Ping(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
