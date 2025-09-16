package uid

import (
	"context"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/uid/internal"
	"github.com/stretchr/testify/assert"
)

// TestUIDProvider 测试 UID Provider 基本功能
func TestUIDProvider(t *testing.T) {
	ctx := context.Background()

	// 创建测试配置
	config := &Config{
		ServiceName:   "test-service",
		MaxInstanceID: 10,
		InstanceID:    1, // 指定实例 ID
	}

	// 创建 Provider
	provider, err := New(ctx, config)
	assert.NoError(t, err)
	defer provider.Close()

	// 测试 UUID v7 生成
	uuid := provider.GetUUIDV7()
	assert.True(t, provider.IsValidUUID(uuid))
	assert.Equal(t, byte('7'), uuid[14]) // 验证版本号

	// 测试 Snowflake ID 生成
	snowflakeID, err := provider.GenerateSnowflake()
	assert.NoError(t, err)
	assert.Greater(t, snowflakeID, int64(0))

	// 测试 Snowflake ID 解析
	timestamp, instanceID, sequence := provider.ParseSnowflake(snowflakeID)
	assert.GreaterOrEqual(t, timestamp, int64(0))
	assert.GreaterOrEqual(t, instanceID, int64(0))
	assert.Less(t, instanceID, int64(config.MaxInstanceID+1))
	assert.GreaterOrEqual(t, sequence, int64(0))
	assert.Less(t, sequence, int64(4096))
}

// TestUIDProviderAutoInstanceID 测试自动分配实例 ID
func TestUIDProviderAutoInstanceID(t *testing.T) {
	ctx := context.Background()

	// 创建测试配置，不指定实例 ID
	config := &Config{
		ServiceName:   "test-service",
		MaxInstanceID: 10,
		InstanceID:    0, // 自动分配
	}

	// 创建 Provider
	provider, err := New(ctx, config)
	assert.NoError(t, err)
	defer provider.Close()

	// 测试 Snowflake ID 生成
	snowflakeID, err := provider.GenerateSnowflake()
	assert.NoError(t, err)
	assert.Greater(t, snowflakeID, int64(0))

	// 验证实例 ID 在合理范围内
	_, instanceID, sequence := provider.ParseSnowflake(snowflakeID)
	assert.GreaterOrEqual(t, instanceID, int64(0))
	assert.Less(t, instanceID, int64(config.MaxInstanceID+1))
	assert.GreaterOrEqual(t, sequence, int64(0))
	assert.Less(t, sequence, int64(4096))
}

// TestSnowflakeGenerator 测试 Snowflake 生成器
func TestSnowflakeGenerator(t *testing.T) {
	instanceID := rand.Int63n(1024)
	generator := internal.NewSnowflakeGenerator(instanceID)

	// 测试单个 ID 生成
	id, err := generator.Generate()
	assert.NoError(t, err)
	assert.Greater(t, id, int64(0))

	// 验证 ID 组成
	timestamp, instID, sequence := generator.Parse(id)
	assert.Equal(t, instanceID, instID)
	assert.GreaterOrEqual(t, timestamp, int64(0))
	assert.GreaterOrEqual(t, sequence, int64(0))
	assert.Less(t, sequence, int64(4096))

	// 测试批量生成
	// ids, err := generator.GenerateBatch(100)
	// assert.NoError(t, err)
	// assert.Len(t, ids, 100)

	// // 验证批量 ID 的唯一性和递增性
	// idSet := make(map[int64]bool)
	// for i, id := range ids {
	// 	assert.False(t, idSet[id], "ID 重复: %d", id)
	// 	idSet[id] = true

	// 	// 验证实例 ID 一致性
	// 	_, instID, _ := generator.Parse(id)
	// 	assert.Equal(t, instanceID, instID)

	// 	// 验证时间戳递增（允许相同毫秒内的序列号递增）
	// 	if i > 0 {
	// 		prevTimestamp, _, prevSequence := generator.Parse(ids[i-1])
	// 		currTimestamp, _, currSequence := generator.Parse(id)

	// 		if currTimestamp > prevTimestamp {
	// 			continue // 时间戳递增，正常
	// 		} else if currTimestamp == prevTimestamp {
	// 			assert.Greater(t, currSequence, prevSequence, "序列号应该递增")
	// 		} else {
	// 			t.Errorf("时间戳不应该递减")
	// 		}
	// 	}
	// }
}

// TestUUIDV7Generation 测试 UUID v7 生成
func TestUUIDV7Generation(t *testing.T) {
	// 测试单个 UUID 生成
	uuid := internal.GenerateUUIDV7()
	assert.True(t, internal.IsValidUUID(uuid))
	assert.Equal(t, byte('7'), uuid[14]) // 验证版本号

	// 测试 UUID 格式
	assert.Len(t, uuid, 36)
	assert.Equal(t, byte('-'), uuid[8])
	assert.Equal(t, byte('-'), uuid[13])
	assert.Equal(t, byte('-'), uuid[18])
	assert.Equal(t, byte('-'), uuid[23])

	// 测试批量生成
	uuids := internal.GenerateUUIDV7Batch(100)
	assert.Len(t, uuids, 100)

	// 验证唯一性
	uuidSet := make(map[string]bool)
	for _, uuid := range uuids {
		assert.False(t, uuidSet[uuid], "UUID 重复: %s", uuid)
		uuidSet[uuid] = true
		assert.True(t, internal.IsValidUUID(uuid))
	}

	// 测试时间戳提取
	for _, uuid := range uuids {
		timestamp, err := internal.ExtractTimestampFromUUIDV7(uuid)
		assert.NoError(t, err)
		assert.Greater(t, timestamp, int64(1609459200000)) // 2021-01-01 之后

		timeObj, err := internal.ExtractTimeFromUUIDV7(uuid)
		assert.NoError(t, err)
		assert.WithinDuration(t, time.Now(), timeObj, 5*time.Second)
	}
}

// TestConcurrentSnowflakeGeneration 测试并发 Snowflake 生成
func TestConcurrentSnowflakeGeneration(t *testing.T) {
	instanceID := rand.Int63n(1024)
	generator := internal.NewSnowflakeGenerator(instanceID)

	var wg sync.WaitGroup
	ids := make(chan int64, 10000)

	// 启动 10 个 goroutine 并发生成 ID
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				id, err := generator.Generate()
				assert.NoError(t, err)
				ids <- id
			}
		}()
	}

	wg.Wait()
	close(ids)

	// 验证所有 ID 唯一
	idSet := make(map[int64]bool)
	for id := range ids {
		assert.False(t, idSet[id], "ID 重复: %d", id)
		idSet[id] = true

		// 验证实例 ID 一致性
		_, instID, _ := generator.Parse(id)
		assert.Equal(t, instanceID, instID)
	}
}

// TestConcurrentUUIDV7Generation 测试并发 UUID v7 生成
func TestConcurrentUUIDV7Generation(t *testing.T) {
	var wg sync.WaitGroup
	uuids := make(chan string, 10000)

	// 启动 10 个 goroutine 并发生成 UUID
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				uuid := internal.GenerateUUIDV7()
				uuids <- uuid
			}
		}()
	}

	wg.Wait()
	close(uuids)

	// 验证所有 UUID 唯一
	uuidSet := make(map[string]bool)
	for uuid := range uuids {
		assert.False(t, uuidSet[uuid], "UUID 重复: %s", uuid)
		uuidSet[uuid] = true
		assert.True(t, internal.IsValidUUID(uuid))
	}
}

// TestUIDProviderValidation 测试配置验证
func TestUIDProviderValidation(t *testing.T) {
	ctx := context.Background()

	// 测试空服务名称
	config := &Config{
		ServiceName:   "",
		MaxInstanceID: 10,
	}
	_, err := New(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "服务名称不能为空")

	// 测试过大实例 ID
	config = &Config{
		ServiceName:   "test-service",
		MaxInstanceID: 2000,
	}
	_, err = New(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最大实例 ID 必须在 1-1023 范围内")

	// 测试无效实例 ID
	config = &Config{
		ServiceName:   "test-service",
		MaxInstanceID: 10,
		InstanceID:    15, // 超出范围
	}
	_, err = New(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "实例 ID 必须在 0-10 范围内")
}

// TestSnowflakeEdgeCases 测试 Snowflake 边界情况
func TestSnowflakeEdgeCases(t *testing.T) {
	instanceID := int64(0)
	generator := internal.NewSnowflakeGenerator(instanceID)

	// 测试最小实例 ID
	id, err := generator.Generate()
	assert.NoError(t, err)
	_, instID, _ := generator.Parse(id)
	assert.Equal(t, int64(0), instID)

	// 测试最大实例 ID
	instanceID = int64(1023)
	generator = internal.NewSnowflakeGenerator(instanceID)
	id, err = generator.Generate()
	assert.NoError(t, err)
	_, instID, _ = generator.Parse(id)
	assert.Equal(t, int64(1023), instID)

	// 测试序列号溢出处理
	startTime := time.Now()
	lastID := int64(0)
	count := 0

	// 快速生成 ID，测试序列号溢出处理
	for time.Since(startTime) < time.Second && count < 5000 {
		id, err = generator.Generate()
		assert.NoError(t, err)
		assert.Greater(t, id, lastID)
		lastID = id
		count++
	}

	t.Logf("在 1 秒内生成了 %d 个 ID", count)
}

// TestUUIDV7Validation 测试 UUID v7 验证
func TestUUIDV7Validation(t *testing.T) {
	// 测试有效的 UUID v7
	validUUIDs := []string{
		"0189d1b0-7a7e-7b3e-8c4d-123456789012",
		"0189d1b0-7a7e-7b3e-8c4d-abcdef123456",
		"0189d1b0-7a7e-7b3e-8c4d-0123456789ab",
	}

	for _, uuid := range validUUIDs {
		assert.True(t, internal.IsValidUUID(uuid), "UUID 应该有效: %s", uuid)
	}

	// 测试无效的 UUID
	invalidUUIDs := []string{
		"0189d1b0-7a7e-7b3e-8c4d-12345678901",   // 长度不足
		"0189d1b0-7a7e-7b3e-8c4d-1234567890123", // 长度过长
		"0189d1b0-6a7e-6b3e-8c4d-123456789012",  // 版本号错误
		"0189d1b0-7a7e-7b3e-0c4d-123456789012",  // 变体错误
		"invalid-uuid-format",                   // 格式错误
	}

	for _, uuid := range invalidUUIDs {
		assert.False(t, internal.IsValidUUID(uuid), "UUID 应该无效: %s", uuid)
	}
}

// TestConfigEnvVars 测试环境变量配置
func TestConfigEnvVars(t *testing.T) {
	// 设置环境变量
	oldServiceName := setEnv("SERVICE_NAME", "test-service-from-env")
	oldMaxInstanceID := setEnv("MAX_INSTANCE_ID", "100")
	oldInstanceID := setEnv("INSTANCE_ID", "5")
	defer func() {
		// 恢复环境变量
		setEnv("SERVICE_NAME", oldServiceName)
		setEnv("MAX_INSTANCE_ID", oldMaxInstanceID)
		setEnv("INSTANCE_ID", oldInstanceID)
	}()

	config := GetDefaultConfig("production")
	assert.Equal(t, "test-service-from-env", config.ServiceName)
	assert.Equal(t, 100, config.MaxInstanceID)
	assert.Equal(t, 5, config.InstanceID)
}

// TestBenchmark 性能基准测试
func TestBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过基准测试")
	}

	// Snowflake 生成基准测试
	t.Run("SnowflakeGeneration", func(t *testing.T) {
		instanceID := rand.Int63n(1024)
		generator := internal.NewSnowflakeGenerator(instanceID)

		start := time.Now()
		count := 100000

		for i := 0; i < count; i++ {
			_, err := generator.Generate()
			assert.NoError(t, err)
		}

		duration := time.Since(start)
		t.Logf("Snowflake 生成 %d 个 ID 耗时: %v (%.0f IDs/s)",
			count, duration, float64(count)/duration.Seconds())
	})

	// UUID v7 生成基准测试
	t.Run("UUIDV7Generation", func(t *testing.T) {
		start := time.Now()
		count := 100000

		for i := 0; i < count; i++ {
			uuid := internal.GenerateUUIDV7()
			assert.True(t, internal.IsValidUUID(uuid))
		}

		duration := time.Since(start)
		t.Logf("UUID v7 生成 %d 个 UUID 耗时: %v (%.0f UUIDs/s)",
			count, duration, float64(count)/duration.Seconds())
	})
}

// TestProviderWithLogger 测试带日志的 Provider
func TestProviderWithLogger(t *testing.T) {
	ctx := context.Background()

	// 创建测试日志器
	logger, err := clog.New(ctx, &clog.Config{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	})
	assert.NoError(t, err)

	// 创建带日志的 Provider
	config := &Config{
		ServiceName:   "test-logger-service",
		MaxInstanceID: 10,
		InstanceID:    1,
	}

	provider, err := New(ctx, config, WithLogger(logger))
	assert.NoError(t, err)
	defer provider.Close()

	// 测试功能正常
	uuid := provider.GetUUIDV7()
	assert.True(t, provider.IsValidUUID(uuid))

	snowflakeID, err := provider.GenerateSnowflake()
	assert.NoError(t, err)
	assert.Greater(t, snowflakeID, int64(0))
}

// 辅助函数：设置环境变量
func setEnv(key, value string) string {
	oldValue := ""
	if value != "" {
		oldValue = setEnvRestore(key, value)
	}
	return oldValue
}

func setEnvRestore(key, value string) string {
	oldValue := ""
	if v, exists := os.LookupEnv(key); exists {
		oldValue = v
	}
	os.Setenv(key, value)
	return oldValue
}
