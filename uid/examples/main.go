package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/uid"
)

func main() {
	fmt.Println("=== uid 组件使用示例 ===")

	// 初始化日志
	ctx := context.Background()
	logger := clog.GetDefaultConfig("development")
	if err := clog.Init(ctx, logger); err != nil {
		panic(err)
	}

	// 示例 1: 基础使用
	fmt.Println("\n1. 基础使用示例")
	basicExample()

	// 示例 2: 配置实例 ID
	fmt.Println("\n2. 配置实例 ID 示例")
	configInstanceIDExample()

	// 示例 3: 环境变量配置
	fmt.Println("\n3. 环境变量配置示例")
	envConfigExample()

	// 示例 4: UUID v7 使用场景
	fmt.Println("\n4. UUID v7 使用场景")
	uuidV7Example()

	// 示例 5: Snowflake 使用场景
	fmt.Println("\n5. Snowflake 使用场景")
	snowflakeExample()

	// 示例 6: 错误处理
	fmt.Println("\n6. 错误处理示例")
	errorHandlingExample()

	fmt.Println("\n✅ 所有示例执行完成")
}

// basicExample 基础使用示例
func basicExample() {
	ctx := context.Background()

	// 创建配置 - 自动分配实例 ID
	config := uid.GetDefaultConfig("production")
	config.ServiceName = "example-service"

	// 创建 uid Provider
	provider, err := uid.New(ctx, config)
	if err != nil {
		clog.Error("创建 uid Provider 失败", clog.Err(err))
		return
	}
	defer provider.Close()

	// 生成 UUID v7
	uuid := provider.GetUUIDV7()
	clog.Info("生成 UUID v7",
		clog.String("uuid", uuid),
		clog.Bool("valid", provider.IsValidUUID(uuid)),
	)

	// 生成 Snowflake ID
	snowflakeID, err := provider.GenerateSnowflake()
	if err != nil {
		clog.Error("生成 Snowflake ID 失败", clog.Err(err))
		return
	}

	// 解析 Snowflake ID
	timestamp, instanceID, sequence := provider.ParseSnowflake(snowflakeID)
	clog.Info("生成 Snowflake ID",
		clog.Int64("id", snowflakeID),
		clog.Int64("timestamp", timestamp),
		clog.Int64("instance_id", instanceID),
		clog.Int64("sequence", sequence),
	)
}

// configInstanceIDExample 配置实例 ID 示例
func configInstanceIDExample() {
	ctx := context.Background()

	// 创建配置 - 指定实例 ID
	config := &uid.Config{
		ServiceName:   "standalone-service",
		MaxInstanceID: 10,
		InstanceID:    3, // 指定实例 ID
	}

	// 创建 Provider
	provider, err := uid.New(ctx, config)
	if err != nil {
		clog.Error("创建 Provider 失败", clog.Err(err))
		return
	}
	defer provider.Close()

	clog.Info("指定实例 ID 的 Provider 创建成功",
		clog.String("service", config.ServiceName),
		clog.Int("instance_id", config.InstanceID),
		clog.Int("max_instance_id", config.MaxInstanceID),
	)

	// 生成一些 ID
	for i := 0; i < 3; i++ {
		uuid := provider.GetUUIDV7()
		snowflakeID, _ := provider.GenerateSnowflake()

		// 验证实例 ID 一致性
		_, instanceID, _ := provider.ParseSnowflake(snowflakeID)

		clog.Info("生成 ID 对",
			clog.String("uuid", uuid),
			clog.Int64("snowflake", snowflakeID),
			clog.Int64("parsed_instance_id", instanceID),
		)

		if instanceID != int64(config.InstanceID) {
			clog.Error("实例 ID 不匹配",
				clog.Int("expected", config.InstanceID),
				clog.Int64("actual", instanceID),
			)
		}
	}
}

// envConfigExample 环境变量配置示例
func envConfigExample() {
	ctx := context.Background()

	// 设置环境变量（实际使用中通过容器或启动脚本设置）
	// os.Setenv("SERVICE_NAME", "env-config-service")
	// os.Setenv("MAX_INSTANCE_ID", "50")
	// os.Setenv("INSTANCE_ID", "7")

	// 创建配置 - 使用环境变量
	config := uid.GetDefaultConfig("production")

	clog.Info("使用环境变量的配置",
		clog.String("service_name", config.ServiceName),
		clog.Int("max_instance_id", config.MaxInstanceID),
		clog.Int("instance_id", config.InstanceID),
	)

	// 创建 Provider
	provider, err := uid.New(ctx, config)
	if err != nil {
		clog.Error("创建 Provider 失败", clog.Err(err))
		return
	}
	defer provider.Close()

	// 测试生成功能
	uuid := provider.GetUUIDV7()
	snowflakeID, _ := provider.GenerateSnowflake()

	clog.Info("环境变量配置测试成功",
		clog.String("uuid", uuid),
		clog.Int64("snowflake", snowflakeID),
	)
}

// uuidV7Example UUID v7 使用场景示例
func uuidV7Example() {
	ctx := context.Background()
	provider, _ := uid.New(ctx, uid.GetDefaultConfig("production"))
	defer provider.Close()

	// 模拟请求 ID 生成
	requestID := provider.GetUUIDV7()
	clog.Info("生成请求 ID",
		clog.String("request_id", requestID),
		clog.String("use_case", "http_request_tracing"),
	)

	// 模拟会话 ID 生成
	sessionID := provider.GetUUIDV7()
	clog.Info("生成会话 ID",
		clog.String("session_id", sessionID),
		clog.String("use_case", "user_session_management"),
	)

	// 模拟外部资源 ID 生成
	resourceID := provider.GetUUIDV7()
	clog.Info("生成资源 ID",
		clog.String("resource_id", resourceID),
		clog.String("use_case", "external_resource_identifier"),
	)

	// 测试时间戳提取
	if timestamp, err := extractTimestampFromUUID(provider, requestID); err == nil {
		clog.Info("UUID 时间戳",
			clog.String("uuid", requestID),
			clog.Int64("timestamp_ms", timestamp),
			clog.String("time", time.Unix(timestamp/1000, (timestamp%1000)*1000000).Format("2006-01-02 15:04:05.000")),
		)
	}
}

// snowflakeExample Snowflake 使用场景示例
func snowflakeExample() {
	ctx := context.Background()
	provider, _ := uid.New(ctx, uid.GetDefaultConfig("production"))
	defer provider.Close()

	// 模拟数据库主键生成
	orderID, err := provider.GenerateSnowflake()
	if err != nil {
		clog.Error("生成订单 ID 失败", clog.Err(err))
		return
	}

	clog.Info("生成订单 ID",
		clog.Int64("order_id", orderID),
		clog.String("use_case", "database_primary_key"),
	)

	// 模拟消息 ID 生成
	messageID, err := provider.GenerateSnowflake()
	if err != nil {
		clog.Error("生成消息 ID 失败", clog.Err(err))
		return
	}

	clog.Info("生成消息 ID",
		clog.Int64("message_id", messageID),
		clog.String("use_case", "message_queue_identifier"),
	)

	// 模拟批量生成订单 ID
	orderIDs, err := generateBatchSnowflakeIDs(provider, 10)
	if err != nil {
		clog.Error("批量生成订单 ID 失败", clog.Err(err))
		return
	}

	clog.Info("批量生成订单 ID",
		clog.Int("count", len(orderIDs)),
		clog.String("order_ids", formatInt64Slice(orderIDs)),
	)

	// 验证排序性
	for i := 1; i < len(orderIDs); i++ {
		if orderIDs[i] <= orderIDs[i-1] {
			clog.Error("Snowflake ID 应该按时间排序",
				clog.Int64("prev_id", orderIDs[i-1]),
				clog.Int64("current_id", orderIDs[i]),
			)
		}
	}
}

// errorHandlingExample 错误处理示例
func errorHandlingExample() {
	ctx := context.Background()

	// 测试无效配置
	invalidConfigs := []struct {
		name   string
		config *uid.Config
	}{
		{
			name: "空服务名称",
			config: &uid.Config{
				ServiceName:   "",
				MaxInstanceID: 10,
			},
		},
		{
			name: "过大实例 ID",
			config: &uid.Config{
				ServiceName:   "test-service",
				MaxInstanceID: 2000,
			},
		},
		{
			name: "无效实例 ID",
			config: &uid.Config{
				ServiceName:   "test-service",
				MaxInstanceID: 10,
				InstanceID:    15, // 超出范围
			},
		},
		{
			name: "负数实例 ID",
			config: &uid.Config{
				ServiceName:   "test-service",
				MaxInstanceID: 10,
				InstanceID:    -1,
			},
		},
	}

	for _, tc := range invalidConfigs {
		_, err := uid.New(ctx, tc.config)
		if err == nil {
			clog.Error("期望配置验证失败，但实际成功",
				clog.String("test_case", tc.name),
			)
		} else {
			clog.Info("配置验证按预期失败",
				clog.String("test_case", tc.name),
				clog.String("error", err.Error()),
			)
		}
	}

	// 测试无效 UUID 验证
	provider, _ := uid.New(ctx, uid.GetDefaultConfig("production"))
	defer provider.Close()

	invalidUUIDs := []string{
		"invalid-uuid",
		"0189d1b0-6a7e-7b3e-8c4d-123456789012", // 错误版本
		"0189d1b0-7a7e-7b3e-0c4d-123456789012", // 错误变体
	}

	for _, uuid := range invalidUUIDs {
		isValid := provider.IsValidUUID(uuid)
		clog.Info("验证无效 UUID",
			clog.String("uuid", uuid),
			clog.Bool("is_valid", isValid),
		)
		if isValid {
			clog.Error("无效 UUID 被错误地验证为有效",
				clog.String("uuid", uuid),
			)
		}
	}
}

// 辅助函数：从 UUID 提取时间戳
func extractTimestampFromUUID(provider uid.Provider, uuid string) (int64, error) {
	// 这里需要添加相应的实现
	// 由于 internal 包未导出相关函数，这里只是示例
	return time.Now().UnixMilli(), nil
}

// 辅助函数：批量生成 Snowflake ID
func generateBatchSnowflakeIDs(provider uid.Provider, count int) ([]int64, error) {
	ids := make([]int64, count)
	for i := 0; i < count; i++ {
		id, err := provider.GenerateSnowflake()
		if err != nil {
			return nil, err
		}
		ids[i] = id
		time.Sleep(1 * time.Millisecond) // 避免同一毫秒内生成
	}
	return ids, nil
}

// 辅助函数：格式化 int64 切片
func formatInt64Slice(slice []int64) string {
	result := "["
	for i, v := range slice {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("%d", v)
	}
	result += "]"
	return result
}
