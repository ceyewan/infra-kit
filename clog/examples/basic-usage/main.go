package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/google/uuid"
)

func main() {
	fmt.Println("=== clog 基础使用示例 ===")

	// 场景 1: 最简单的使用方式
	basicUsage()

	// 场景 2: Provider 模式
	providerPattern()

	// 场景 3: 上下文与链路追踪
	contextualLogging()

	// 场景 4: 层次化命名空间
	hierarchicalNamespaces()

	// 场景 5: 结构化字段
	structuredFields()

	fmt.Println("\n=== 基础使用示例完成 ===")
}

// basicUsage 最简单的使用方式
func basicUsage() {
	fmt.Println("\n--- 基础使用 ---")

	// 1. 初始化（最简单的方式）
	config := clog.GetDefaultConfig("development")
	if err := clog.Init(context.Background(), config, clog.WithNamespace("my-app")); err != nil {
		fmt.Printf("初始化失败: %v\n", err)
		return
	}

	// 2. 直接使用全局函数
	clog.Info("应用启动", clog.String("version", "1.0.0"))
	clog.Debug("调试信息", clog.Int("user_count", 42))
	clog.Warn("警告信息", clog.String("reason", "内存使用率过高"))
	clog.Error("错误信息", clog.Err(fmt.Errorf("数据库连接失败")))

	fmt.Println("基础使用演示完成")
}

// providerPattern Provider 模式
func providerPattern() {
	fmt.Println("\n--- Provider 模式 ---")

	ctx := context.Background()

	// 1. 创建 Provider
	config := clog.GetDefaultConfig("production")
	provider, err := clog.New(ctx, config, clog.WithNamespace("order-service"))
	if err != nil {
		fmt.Printf("创建 Provider 失败: %v\n", err)
		return
	}
	defer provider.Close()

	// 2. 使用 Provider 方法
	logger := provider.WithContext(ctx)
	logger.Info("Provider 模式使用示例",
		clog.Component("order-service"),
		clog.Version("2.0.0"))

	// 3. 创建命名空间
	orderLogger := provider.Namespace("order")
	orderLogger.Info("订单创建",
		clog.String("order_id", "12345"),
		clog.Float64("amount", 99.99))

	fmt.Println("Provider 模式演示完成")
}

// contextualLogging 上下文与链路追踪
func contextualLogging() {
	fmt.Println("\n--- 上下文与链路追踪 ---")

	// 1. 创建带 traceID 的 context
	traceID := uuid.NewString()
	ctx := clog.WithTraceID(context.Background(), traceID)

	// 2. 从 context 获取 logger
	logger := clog.WithContext(ctx)

	// 3. 记录日志（自动包含 traceID）
	logger.Info("开始处理用户请求",
		clog.UserID("user-123"),
		clog.Operation("get_user_info"))

	// 4. 模拟业务处理
	processUserRequest(ctx)

	fmt.Printf("链路追踪演示完成，traceID: %s\n", traceID)
}

// hierarchicalNamespaces 层次化命名空间
func hierarchicalNamespaces() {
	fmt.Println("\n--- 层次化命名空间 ---")

	// 1. 链式创建命名空间
	paymentLogger := clog.Namespace("payment").Namespace("process").Namespace("stripe")

	// 2. 记录日志（自动包含完整命名空间路径）
	paymentLogger.Info("处理支付请求",
		clog.String("order_id", "order-678"),
		clog.Float64("amount", 199.99),
		clog.String("method", "credit_card"))

	// 3. 不同的命名空间层级
	userLogger := clog.Namespace("user")
	authLogger := userLogger.Namespace("auth")
	profileLogger := userLogger.Namespace("profile")

	authLogger.Info("用户认证", clog.String("user_id", "user-456"))
	profileLogger.Info("更新用户资料", clog.String("user_id", "user-456"))

	fmt.Println("层次化命名空间演示完成")
}

// structuredFields 结构化字段
func structuredFields() {
	fmt.Println("\n--- 结构化字段 ---")

	ctx := context.Background()

	// 1. 基本字段类型
	logger := clog.WithContext(ctx)
	logger.Info("用户登录成功",
		clog.String("user_id", "user-789"),
		clog.String("email", "user@example.com"),
		clog.String("ip_address", "192.168.1.100"),
		clog.Bool("is_premium", true),
		clog.Int("login_count", 5),
		clog.Float64("account_balance", 1250.50),
		clog.Time("login_time", time.Now()),
		clog.Duration("session_timeout", 30*time.Minute))

	// 2. 复杂字段
	logger.Info("处理订单",
		clog.Any("order_data", map[string]interface{}{
			"id":     "order-999",
			"items":  []string{"item1", "item2"},
			"total":  299.99,
			"status": "completed",
		}),
		clog.Err(fmt.Errorf("库存不足警告")))

	// 3. 自定义字段类型
	logger.Info("系统监控",
		clog.Component("monitoring"),
		clog.Version("1.2.3"),
		clog.Host("server-01"),
		clog.Metrics("cpu_usage", 75.5),
		clog.Metrics("memory_usage", 2048.0),
		clog.Status("healthy"))

	fmt.Println("结构化字段演示完成")
}

// processUserRequest 模拟用户请求处理
func processUserRequest(ctx context.Context) {
	startTime := time.Now()
	logger := clog.WithContext(ctx)

	logger.Info("开始处理用户请求",
		clog.Time("start_time", startTime))

	// 模拟业务处理
	time.Sleep(100 * time.Millisecond)

	// 记录中间步骤
	logger.Debug("查询用户数据",
		clog.String("query", "SELECT * FROM users WHERE id = ?"))

	// 记录成功结果
	logger.Info("用户请求处理完成",
		clog.Duration("processing_time", time.Since(startTime)),
		clog.Bool("success", true))
}
