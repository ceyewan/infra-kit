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

	// 新增示例 7: 实际业务场景
	fmt.Println("\n7. 实际业务场景示例")
	businessScenarioExample()

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

	// 演示时间戳提取
	generatedTime := time.Unix(timestamp/1000+1609459200, (timestamp%1000)*1000000)
	clog.Info("Snowflake ID 时间信息",
		clog.String("generated_time", generatedTime.Format("2006-01-02 15:04:05.000")),
		clog.String("time_from_now", time.Since(generatedTime).String()),
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

	// 演示 UUID v7 时间戳提取（虽然目前 internal 包未导出）
	clog.Info("UUID v7 特性说明",
		clog.String("note", "UUID v7 基于时间戳，具有时间排序特性"),
		clog.String("request_id", requestID),
		clog.String("format_check", "版本号为7，变体为RFC4122"),
	)

	// 验证 UUID 格式
	clog.Info("UUID 格式验证",
		clog.String("request_id", requestID),
		clog.Bool("is_valid", provider.IsValidUUID(requestID)),
		clog.String("version_character", string(requestID[14])),
	)

	// 批量生成演示
	fmt.Println("\n批量 UUID v7 生成演示:")
	for i := 0; i < 5; i++ {
		uuid := provider.GetUUIDV7()
		fmt.Printf("  UUID %d: %s\n", i+1, uuid)
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

	// 解析订单 ID 信息
	timestamp, instanceID, sequence := provider.ParseSnowflake(orderID)
	generatedTime := time.Unix(timestamp/1000+1609459200, (timestamp%1000)*1000000)
	clog.Info("订单 ID 解析",
		clog.Int64("order_id", orderID),
		clog.String("generated_time", generatedTime.Format("2006-01-02 15:04:05.000")),
		clog.Int64("instance_id", instanceID),
		clog.Int64("sequence", sequence),
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

	// 演示 Snowflake ID 的排序性
	fmt.Println("\nSnowflake ID 排序性演示:")
	var ids []int64
	for i := 0; i < 5; i++ {
		id, err := provider.GenerateSnowflake()
		if err != nil {
			continue
		}
		ids = append(ids, id)
		time.Sleep(1 * time.Millisecond) // 确保时间戳递增
	}

	fmt.Println("生成的 ID 序列:")
	for i, id := range ids {
		fmt.Printf("  ID %d: %d\n", i+1, id)
	}

	// 验证排序性
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			clog.Error("Snowflake ID 应该按时间排序",
				clog.Int64("prev_id", ids[i-1]),
				clog.Int64("current_id", ids[i]),
			)
		}
	}
	clog.Info("排序性验证", clog.Bool("is_sorted", true))

	// 演示高并发生成
	fmt.Println("\n高并发生成演示:")
	start := time.Now()
	count := 100
	for i := 0; i < count; i++ {
		_, _ = provider.GenerateSnowflake()
	}
	duration := time.Since(start)
	clog.Info("高并发生成性能",
		clog.Int("count", count),
		clog.String("duration", duration.String()),
		clog.Float64("ids_per_second", float64(count)/duration.Seconds()),
	)
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
		"0189d1b0-6a7e-6b3e-8c4d-123456789012", // 错误版本 (version 6)
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

// businessScenarioExample 实际业务场景示例
func businessScenarioExample() {
	ctx := context.Background()

	// 模拟订单服务的 ID 生成器
	orderService := &OrderService{
		uidProvider: createTestProvider(ctx),
	}

	// 模拟用户服务的 ID 生成器
	userService := &UserService{
		uidProvider: createTestProvider(ctx),
	}

	fmt.Println("\n=== 订单服务场景 ===")
	order, err := orderService.CreateOrder(&CreateOrderRequest{
		UserID:  "user123",
		Amount:  99.99,
		Product: "Go 编程书籍",
	})
	if err != nil {
		clog.Error("创建订单失败", clog.Err(err))
		return
	}

	clog.Info("订单创建成功",
		clog.String("order_id", fmt.Sprintf("%d", order.ID)),
		clog.String("user_id", order.UserID),
		clog.Float64("amount", order.Amount),
		clog.String("status", order.Status),
	)

	fmt.Println("\n=== 用户服务场景 ===")
	session, err := userService.CreateSession("user456")
	if err != nil {
		clog.Error("创建会话失败", clog.Err(err))
		return
	}

	clog.Info("用户会话创建成功",
		clog.String("session_id", session.ID),
		clog.String("user_id", session.UserID),
		clog.String("expires_at", session.ExpiresAt.Format("2006-01-02 15:04:05")),
	)

	fmt.Println("\n=== ID 类型对比 ===")
	fmt.Printf("订单 ID (Snowflake): %d (可排序、高性能)\n", order.ID)
	fmt.Printf("会话 ID (UUID v7): %s (全局唯一、不可猜测)\n", session.ID)
}

// OrderService 订单服务模拟
type OrderService struct {
	uidProvider uid.Provider
}

type Order struct {
	ID        int64
	UserID    string
	Amount    float64
	Product   string
	Status    string
	CreatedAt time.Time
}

type CreateOrderRequest struct {
	UserID  string
	Amount  float64
	Product string
}

func (s *OrderService) CreateOrder(req *CreateOrderRequest) (*Order, error) {
	orderID, err := s.uidProvider.GenerateSnowflake()
	if err != nil {
		return nil, fmt.Errorf("生成订单 ID 失败: %w", err)
	}

	order := &Order{
		ID:        orderID,
		UserID:    req.UserID,
		Amount:    req.Amount,
		Product:   req.Product,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	return order, nil
}

// UserService 用户服务模拟
type UserService struct {
	uidProvider uid.Provider
}

type Session struct {
	ID        string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (s *UserService) CreateSession(userID string) (*Session, error) {
	sessionID := s.uidProvider.GetUUIDV7()

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	return session, nil
}

// createTestProvider 创建测试用的 Provider
func createTestProvider(ctx context.Context) uid.Provider {
	config := uid.GetDefaultConfig("production")
	config.ServiceName = "test-business-service"

	provider, err := uid.New(ctx, config)
	if err != nil {
		panic(err)
	}

	return provider
}

// 辅助函数：格式化 int64 切片（保留用于其他示例）
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
