package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// User 用户模型
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserService 用户服务
type UserService struct {
	logger clog.Logger
}

// OrderService 订单服务
type OrderService struct {
	logger clog.Logger
}

func main() {
	fmt.Println("=== clog 完整使用示例 ===")

	// 初始化 clog
	initializeClog()

	// 演示各种使用场景
	demonstrateBasicUsage()
	demonstrateContextualLogging()
	demonstrateHierarchicalNamespaces()
	demonstrateHTTPIntegration()
	demonstrateErrorHandling()
	demonstratePerformanceMonitoring()

	fmt.Println("\n=== 所有示例演示完成 ===")
}

// initializeClog 初始化 clog
func initializeClog() {
	ctx := context.Background()

	// 使用生产环境配置
	config := clog.GetDefaultConfig("production")
	config.Level = "info"
	config.AddSource = true

	// 初始化全局 clog
	if err := clog.Init(ctx, config, clog.WithNamespace("example-service")); err != nil {
		log.Fatalf("初始化 clog 失败: %v", err)
	}

	clog.Info("clog 初始化成功",
		clog.Component("example-service"),
		clog.Version("1.0.0"))
}

// demonstrateBasicUsage 演示基础使用
func demonstrateBasicUsage() {
	fmt.Println("\n--- 基础使用示例 ---")

	// 1. 使用全局函数
	clog.Info("应用启动",
		clog.String("environment", "production"),
		clog.String("version", "1.0.0"))

	// 2. 创建命名空间
	userLogger := clog.Namespace("user")
	userLogger.Info("用户服务启动")

	// 3. 使用不同级别的日志
	clog.Debug("调试信息")
	clog.Info("信息日志")
	clog.Warn("警告日志")
	clog.Error("错误日志")

	// 4. 使用 Provider 模式
	provider, err := clog.New(context.Background(), clog.GetDefaultConfig("development"),
		clog.WithNamespace("temp-provider"))
	if err != nil {
		clog.Error("创建 Provider 失败", clog.Err(err))
		return
	}
	defer provider.Close()

	providerLogger := provider.WithContext(context.Background())
	providerLogger.Info("Provider 模式使用示例")

	fmt.Println("基础使用示例完成")
}

// demonstrateContextualLogging 演示上下文感知日志
func demonstrateContextualLogging() {
	fmt.Println("\n--- 上下文感知日志示例 ---")

	// 1. 创建带 traceID 的 context
	traceID := uuid.NewString()
	ctx := clog.WithTraceID(context.Background(), traceID)

	// 2. 处理用户请求
	processUserRequest(ctx, "user-123")

	// 3. 处理订单请求
	processOrderRequest(ctx, "order-456")

	fmt.Printf("上下文感知日志示例完成，traceID: %s\n", traceID)
}

// demonstrateHierarchicalNamespaces 演示层次化命名空间
func demonstrateHierarchicalNamespaces() {
	fmt.Println("\n--- 层次化命名空间示例 ---")

	// 1. 链式创建命名空间
	paymentLogger := clog.Namespace("payment").Namespace("process").Namespace("credit_card")
	paymentLogger.Info("处理信用卡支付",
		clog.String("order_id", "order-789"),
		clog.Float64("amount", 299.99))

	// 2. 独立创建命名空间
	userLogger := clog.Namespace("user")
	authLogger := userLogger.Namespace("auth")
	profileLogger := userLogger.Namespace("profile")

	authLogger.Info("用户认证",
		clog.String("user_id", "user-456"),
		clog.String("method", "password"))

	profileLogger.Info("更新用户资料",
		clog.String("user_id", "user-456"),
		clog.String("field", "avatar"))

	// 3. 嵌套使用
	clog.Namespace("system").Namespace("monitor").Namespace("cpu").Info("CPU 使用率",
		clog.Float64("usage", 75.5))

	fmt.Println("层次化命名空间示例完成")
}

// demonstrateHTTPIntegration 演示 HTTP 集成
func demonstrateHTTPIntegration() {
	fmt.Println("\n--- HTTP 集成示例 ---")

	// 1. 创建 HTTP 服务器
	r := mux.NewRouter()

	// 2. 添加中间件
	r.Use(loggingMiddleware())
	r.Use(traceMiddleware())

	// 3. 添加路由
	r.HandleFunc("/api/users/{id}", handleGetUser).Methods("GET")
	r.HandleFunc("/api/orders", handleCreateOrder).Methods("POST")

	// 4. 模拟 HTTP 请求
	simulateHTTPRequest(r)

	fmt.Println("HTTP 集成示例完成")
}

// demonstrateErrorHandling 演示错误处理
func demonstrateErrorHandling() {
	fmt.Println("\n--- 错误处理示例 ---")

	ctx := context.Background()
	logger := clog.WithContext(ctx)

	// 1. 处理成功场景
	if err := processBusinessLogic(ctx, "success-case"); err != nil {
		logger.Error("业务逻辑处理失败", clog.Err(err))
	}

	// 2. 处理失败场景
	if err := processBusinessLogic(ctx, "failure-case"); err != nil {
		logger.Error("业务逻辑处理失败",
			clog.Err(err),
			clog.ErrorDetails(err),
			clog.String("case", "failure-case"))
	}

	// 3. 处理异常场景
	if err := processBusinessLogic(ctx, "exception-case"); err != nil {
		logger.Error("业务逻辑处理失败",
			clog.Err(err),
			clog.String("case", "exception-case"),
			clog.Bool("recovered", true))
	}

	fmt.Println("错误处理示例完成")
}

// demonstratePerformanceMonitoring 演示性能监控
func demonstratePerformanceMonitoring() {
	fmt.Println("\n--- 性能监控示例 ---")

	ctx := context.Background()
	logger := clog.WithContext(ctx).Namespace("performance")

	// 1. 监控函数性能
	startTime := time.Now()
	result, err := monitorFunctionPerformance(ctx)
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("函数执行失败",
			clog.Err(err),
			clog.Duration("execution_time", duration))
	} else {
		logger.Info("函数执行成功",
			clog.Duration("execution_time", duration),
			clog.Any("result", result))
	}

	// 2. 监控批量操作
	batchStartTime := time.Now()
	successCount := monitorBatchOperation(ctx, 100)
	batchDuration := time.Since(batchStartTime)

	logger.Info("批量操作完成",
		clog.Duration("total_duration", batchDuration),
		clog.Int("total_items", 100),
		clog.Int("success_count", successCount),
		clog.Metrics("success_rate", float64(successCount)/100.0))

	fmt.Println("性能监控示例完成")
}

// processUserRequest 处理用户请求
func processUserRequest(ctx context.Context, userID string) {
	startTime := time.Now()
	logger := clog.WithContext(ctx).Namespace("user")

	logger.Info("开始处理用户请求",
		clog.UserID(userID),
		clog.Time("start_time", startTime))

	// 模拟业务处理
	time.Sleep(50 * time.Millisecond)

	// 创建用户服务
	userService := &UserService{logger: logger}
	user, err := userService.GetUser(ctx, userID)
	if err != nil {
		logger.Error("获取用户失败",
			clog.Err(err),
			clog.Duration("processing_time", time.Since(startTime)))
		return
	}

	logger.Info("用户请求处理完成",
		clog.UserID(userID),
		clog.String("user_name", user.Name),
		clog.Duration("total_duration", time.Since(startTime)))
}

// processOrderRequest 处理订单请求
func processOrderRequest(ctx context.Context, orderID string) {
	startTime := time.Now()
	logger := clog.WithContext(ctx).Namespace("order")

	logger.Info("开始处理订单请求",
		clog.String("order_id", orderID),
		clog.Time("start_time", startTime))

	// 模拟业务处理
	time.Sleep(100 * time.Millisecond)

	// 创建订单服务
	orderService := &OrderService{logger: logger}
	order, err := orderService.GetOrder(ctx, orderID)
	if err != nil {
		logger.Error("获取订单失败",
			clog.Err(err),
			clog.Duration("processing_time", time.Since(startTime)))
		return
	}

	logger.Info("订单请求处理完成",
		clog.String("order_id", orderID),
		clog.Float64("amount", order.Amount),
		clog.Duration("total_duration", time.Since(startTime)))
}

// loggingMiddleware 日志中间件
func loggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 包装 ResponseWriter 来捕获状态码
			wrapped := &responseWriter{w, http.StatusOK}

			// 处理请求
			next.ServeHTTP(wrapped, r)

			// 记录日志
			logger := clog.WithContext(r.Context())
			logger.Info("HTTP 请求",
				clog.String("method", r.Method),
				clog.String("path", r.URL.Path),
				clog.Int("status", wrapped.status),
				clog.Duration("duration", time.Since(start)),
				clog.String("user_agent", r.UserAgent()))
		})
	}
}

// traceMiddleware 链路追踪中间件
func traceMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 获取或生成 traceID
			traceID := r.Header.Get("X-Trace-ID")
			if traceID == "" {
				traceID = uuid.NewString()
			}

			// 注入 traceID 到 context
			ctx := clog.WithTraceID(r.Context(), traceID)
			r = r.WithContext(ctx)

			// 添加响应头
			w.Header().Set("X-Trace-ID", traceID)

			// 处理请求
			next.ServeHTTP(w, r)
		})
	}
}

// handleGetUser 处理获取用户请求
func handleGetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	logger := clog.WithContext(r.Context()).Namespace("user")
	logger.Info("获取用户信息",
		clog.UserID(userID))

	// 模拟用户数据
	user := User{
		ID:    userID,
		Name:  "John Doe",
		Email: "john@example.com",
	}

	w.Write([]byte(fmt.Sprintf(`{"id": "%s", "name": "%s", "email": "%s"}`, user.ID, user.Name, user.Email)))
}

// handleCreateOrder 处理创建订单请求
func handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	logger := clog.WithContext(r.Context()).Namespace("order")
	logger.Info("创建订单")

	// 模拟订单创建
	orderID := uuid.NewString()

	w.Write([]byte(fmt.Sprintf(`{"id": "%s", "status": "created"}`, orderID)))
}

// responseWriter 包装 http.ResponseWriter
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// simulateHTTPRequest 模拟 HTTP 请求
func simulateHTTPRequest(r *mux.Router) {
	req, _ := http.NewRequest("GET", "/api/users/123", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("X-Trace-ID", "test-trace-123")

	w := &responseWriter{ResponseWriter: &dummyResponseWriter{}}
	r.ServeHTTP(w, req)
}

// dummyResponseWriter 虚拟响应写入器
type dummyResponseWriter struct{}

func (w *dummyResponseWriter) Header() http.Header            { return make(http.Header) }
func (w *dummyResponseWriter) Write(data []byte) (int, error) { return len(data), nil }
func (w *dummyResponseWriter) WriteHeader(statusCode int)     {}

// processBusinessLogic 处理业务逻辑
func processBusinessLogic(ctx context.Context, caseType string) error {
	logger := clog.WithContext(ctx).Namespace("business")

	logger.Info("开始处理业务逻辑",
		clog.String("case_type", caseType))

	switch caseType {
	case "success-case":
		time.Sleep(50 * time.Millisecond)
		return nil
	case "failure-case":
		time.Sleep(30 * time.Millisecond)
		return fmt.Errorf("业务规则验证失败")
	case "exception-case":
		time.Sleep(20 * time.Millisecond)
		return fmt.Errorf("系统异常")
	default:
		return fmt.Errorf("未知的案例类型: %s", caseType)
	}
}

// monitorFunctionPerformance 监控函数性能
func monitorFunctionPerformance(ctx context.Context) (map[string]interface{}, error) {
	// 模拟函数执行
	time.Sleep(75 * time.Millisecond)

	// 模拟随机失败
	if time.Now().Unix()%3 == 0 {
		return nil, fmt.Errorf("函数执行失败")
	}

	return map[string]interface{}{
		"processed_items": 42,
		"success_rate":    0.95,
		"average_time":    75.0,
	}, nil
}

// monitorBatchOperation 监控批量操作
func monitorBatchOperation(ctx context.Context, totalItems int) int {
	successCount := 0

	for i := 0; i < totalItems; i++ {
		// 模拟批量处理
		time.Sleep(2 * time.Millisecond)

		// 模拟成功率为 95%
		if time.Now().Unix()%20 != 0 {
			successCount++
		}
	}

	return successCount
}

// UserService 方法实现
func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
	s.logger.Info("查询用户数据",
		clog.UserID(userID))

	// 模拟数据库查询
	time.Sleep(30 * time.Millisecond)

	// 模拟随机失败
	if time.Now().Unix()%5 == 0 {
		return nil, fmt.Errorf("用户不存在")
	}

	return &User{
		ID:    userID,
		Name:  "John Doe",
		Email: "john@example.com",
	}, nil
}

// OrderService 方法实现
type Order struct {
	ID     string  `json:"id"`
	Amount float64 `json:"amount"`
	Status string  `json:"status"`
}

func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	s.logger.Info("查询订单数据",
		clog.String("order_id", orderID))

	// 模拟数据库查询
	time.Sleep(50 * time.Millisecond)

	// 模拟随机失败
	if time.Now().Unix()%7 == 0 {
		return nil, fmt.Errorf("订单不存在")
	}

	return &Order{
		ID:     orderID,
		Amount: 199.99,
		Status: "completed",
	}, nil
}
