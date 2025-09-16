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

// UserService 业务服务示例
type UserService struct {
	logger clog.Logger
}

// OrderService 订单服务示例
type OrderService struct {
	logger clog.Logger
}

func main() {
	fmt.Println("=== clog 核心使用场景演示 ===")

	// 场景 1: Provider 模式初始化
	demonstrateProviderPattern()

	// 场景 2: 上下文感知与链路追踪
	demonstrateContextualLogging()

	// 场景 3: 层次化命名空间
	demonstrateHierarchicalNamespaces()

	// 场景 4: HTTP 中间件集成
	demonstrateHTTPMiddleware()

	// 场景 5: 错误处理与调试
	demonstrateErrorHandling()

	// 场景 6: 性能优化特性
	demonstratePerformanceFeatures()

	// 场景 7: 动态配置监听（模拟）
	demonstrateDynamicConfig()

	// 场景 8: 健康检查
	demonstrateHealthCheck()

	fmt.Println("\n=== 所有核心场景演示完成 ===")
}

// demonstrateProviderPattern 演示 Provider 模式
func demonstrateProviderPattern() {
	fmt.Println("\n--- 场景 1: Provider 模式初始化 ---")

	ctx := context.Background()

	// 1. 使用 New 创建 Provider 实例
	config := clog.GetDefaultConfig("production")
	config.Level = "debug"
	config.Output = "stdout"
	config.AddSource = true

	provider, err := clog.New(ctx, config, clog.WithNamespace("user-service"))
	if err != nil {
		log.Fatalf("创建 Provider 失败: %v", err)
	}
	defer provider.Close()

	// 2. 使用 Provider 方法
	logger := provider.WithContext(ctx)
	logger.Info("Provider 模式初始化成功",
		clog.Component("clog"),
		clog.Version("1.0.0"))

	// 3. 层次化命名空间
	userLogger := provider.Namespace("user")
	userLogger.Info("用户服务日志")

	authLogger := userLogger.Namespace("auth")
	authLogger.Warn("认证服务警告")

	// 4. 链路追踪
	traceID := uuid.NewString()
	ctxWithTrace := provider.WithTraceID(ctx, traceID)
	traceLogger := provider.WithContext(ctxWithTrace)
	traceLogger.Info("带链路追踪的日志")

	fmt.Printf("Provider 创建成功，根命名空间: user-service\n")
}

// demonstrateContextualLogging 演示上下文感知日志
func demonstrateContextualLogging() {
	fmt.Println("\n--- 场景 2: 上下文感知与链路追踪 ---")

	// 1. 初始化全局 clog
	config := clog.GetDefaultConfig("development")
	if err := clog.Init(context.Background(), config, clog.WithNamespace("demo-service")); err != nil {
		log.Fatalf("初始化 clog 失败: %v", err)
	}

	// 2. 模拟完整的请求处理流程
	processRequest := func(ctx context.Context, userID string) error {
		startTime := time.Now()

		// 从 context 获取带 traceID 的 logger
		logger := clog.WithContext(ctx)
		logger.Info("开始处理用户请求",
			clog.UserID(userID),
			clog.Operation("process_user"),
			clog.Time("start_time", startTime))

		// 业务处理
		userService := &UserService{logger: logger.Namespace("user")}
		if err := userService.ProcessUser(ctx, userID); err != nil {
			logger.Error("用户处理失败",
				clog.Err(err),
				clog.Duration("duration", time.Since(startTime)),
				clog.Bool("success", false))
			return err
		}

		logger.Info("用户处理成功",
			clog.Duration("total_duration", time.Since(startTime)),
			clog.Bool("success", true))

		return nil
	}

	// 3. 模拟多个并发请求
	traceID := uuid.NewString()
	ctx := clog.WithTraceID(context.Background(), traceID)

	if err := processRequest(ctx, "user-123"); err != nil {
		log.Printf("请求处理失败: %v", err)
	}

	fmt.Printf("上下文感知日志演示完成，traceID: %s\n", traceID)
}

// demonstrateHierarchicalNamespaces 演示层次化命名空间
func demonstrateHierarchicalNamespaces() {
	fmt.Println("\n--- 场景 3: 层次化命名空间 ---")

	ctx := context.Background()

	// 创建不同层级的 logger
	mainLogger := clog.Namespace("payment-service")
	orderLogger := mainLogger.Namespace("order")
	paymentLogger := orderLogger.Namespace("payment")
	refundLogger := orderLogger.Namespace("refund")

	// 记录不同层级的日志
	mainLogger.Info("支付服务启动",
		clog.Version("2.1.0"),
		clog.Host("payment-01"))

	orderLogger.Info("创建订单",
		clog.String("order_id", "order-456"),
		clog.Float64("amount", 99.99))

	paymentLogger.Info("处理支付",
		clog.String("payment_method", "credit_card"),
		clog.String("transaction_id", "txn-789"))

	refundLogger.Warn("退款处理延迟",
		clog.String("order_id", "order-456"),
		clog.Duration("delay", 5*time.Minute))

	// 链式调用示例
	analyticsLogger := clog.Namespace("analytics").Namespace("metrics").Namespace("performance")
	analyticsLogger.Info("性能指标",
		clog.Metrics("response_time", 125.5),
		clog.Metrics("throughput", 1000.0))

	fmt.Println("层次化命名空间演示完成")
}

// demonstrateHTTPMiddleware 演示 HTTP 中间件集成
func demonstrateHTTPMiddleware() {
	fmt.Println("\n--- 场景 4: HTTP 中间件集成 ---")

	// 1. 创建路由
	r := mux.NewRouter()

	// 2. 添加中间件
	r.Use(TraceMiddleware())
	r.Use(LoggingMiddleware())

	// 3. 添加路由
	r.HandleFunc("/api/users/{id}", handleUserRequest).Methods("GET")
	r.HandleFunc("/api/orders", handleOrderRequest).Methods("POST")

	// 4. 模拟 HTTP 请求处理
	simulateHTTPRequest(r)

	fmt.Println("HTTP 中间件集成演示完成")
}

// TraceMiddleware 链路追踪中间件
func TraceMiddleware() func(http.Handler) http.Handler {
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

			// 记录请求开始
			logger := clog.WithContext(ctx)
			logger.Info("HTTP 请求开始",
				clog.String("method", r.Method),
				clog.String("path", r.URL.Path),
				clog.String("remote_addr", r.RemoteAddr))

			// 调用下一个处理器
			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 包装 ResponseWriter 来捕获状态码
			wrapped := &responseWriter{w, http.StatusOK}

			// 调用下一个处理器
			next.ServeHTTP(wrapped, r)

			// 记录请求完成
			logger := clog.WithContext(r.Context())
			logger.Info("HTTP 请求完成",
				clog.String("method", r.Method),
				clog.String("path", r.URL.Path),
				clog.Int("status", wrapped.status),
				clog.Duration("duration", time.Since(start)),
				clog.String("user_agent", r.UserAgent()))
		})
	}
}

// responseWriter 包装 http.ResponseWriter 来捕获状态码
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// handleUserRequest 处理用户请求
func handleUserRequest(w http.ResponseWriter, r *http.Request) {
	logger := clog.WithContext(r.Context()).Namespace("user")

	vars := mux.Vars(r)
	userID := vars["id"]

	logger.Info("获取用户信息",
		clog.UserID(userID),
		clog.String("handler", "get_user"))

	// 模拟业务处理
	time.Sleep(50 * time.Millisecond)

	w.Write([]byte(fmt.Sprintf(`{"id": "%s", "name": "John Doe"}`, userID)))
}

// handleOrderRequest 处理订单请求
func handleOrderRequest(w http.ResponseWriter, r *http.Request) {
	logger := clog.WithContext(r.Context()).Namespace("order")

	logger.Info("创建订单",
		clog.Operation("create_order"),
		clog.String("handler", "create_order"))

	// 模拟业务处理
	time.Sleep(100 * time.Millisecond)

	orderID := uuid.NewString()
	w.Write([]byte(fmt.Sprintf(`{"id": "%s", "status": "created"}`, orderID)))
}

// simulateHTTPRequest 模拟 HTTP 请求
func simulateHTTPRequest(r *mux.Router) {
	// 创建测试请求
	req, _ := http.NewRequest("GET", "/api/users/123", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("X-Trace-ID", "test-trace-123")

	// 创建响应记录器
	w := &responseWriter{ResponseWriter: &dummyResponseWriter{}}

	// 处理请求
	r.ServeHTTP(w, req)
}

// dummyResponseWriter 虚拟响应写入器
type dummyResponseWriter struct{}

func (w *dummyResponseWriter) Header() http.Header {
	return make(http.Header)
}

func (w *dummyResponseWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (w *dummyResponseWriter) WriteHeader(statusCode int) {
	// do nothing
}

// demonstrateErrorHandling 演示错误处理
func demonstrateErrorHandling() {
	fmt.Println("\n--- 场景 5: 错误处理与调试 ---")

	ctx := context.Background()

	// 模拟错误处理场景
	handleError := func(ctx context.Context, operation string) error {
		logger := clog.WithContext(ctx).Namespace("error-handling")

		logger.Info("开始处理操作",
			clog.Operation(operation))

		// 模拟错误
		if time.Now().Unix()%2 == 0 {
			err := fmt.Errorf("模拟业务错误: %s 失败", operation)
			logger.Error("操作失败",
				clog.Err(err),
				clog.ErrorDetails(err),
				clog.Operation(operation),
				clog.String("error_type", "business_error"))
			return err
		}

		logger.Info("操作成功", clog.Operation(operation))
		return nil
	}

	// 测试错误处理
	operations := []string{"database_query", "api_call", "cache_operation"}
	for _, op := range operations {
		if err := handleError(ctx, op); err != nil {
			clog.Warn("操作失败，继续处理", clog.Err(err))
		}
	}

	fmt.Println("错误处理演示完成")
}

// demonstratePerformanceFeatures 演示性能优化特性
func demonstratePerformanceFeatures() {
	fmt.Println("\n--- 场景 6: 性能优化特性 ---")

	ctx := context.Background()
	startTime := time.Now()

	// 模拟高频日志记录
	logger := clog.WithContext(ctx).Namespace("performance")

	// 记录性能指标
	logger.Info("开始性能测试",
		clog.Time("start_time", startTime),
		clog.String("test_type", "high_frequency_logging"))

	// 模拟高频日志记录（1000次）
	for i := 0; i < 1000; i++ {
		logger.Debug("高频日志记录",
			clog.Int("iteration", i),
			clog.String("test_id", uuid.NewString()))
	}

	duration := time.Since(startTime)
	logger.Info("性能测试完成",
		clog.Duration("total_duration", duration),
		clog.Metrics("logs_per_second", float64(1000)/duration.Seconds()),
		clog.Int("total_logs", 1000))

	fmt.Printf("性能优化演示完成，处理 1000 条日志耗时: %v\n", duration)
}

// demonstrateDynamicConfig 演示动态配置（模拟）
func demonstrateDynamicConfig() {
	fmt.Println("\n--- 场景 7: 动态配置监听（模拟） ---")

	ctx := context.Background()

	// 1. 创建 Provider
	config := clog.GetDefaultConfig("development")
	provider, err := clog.New(ctx, config, clog.WithNamespace("dynamic-config-demo"))
	if err != nil {
		log.Fatalf("创建 Provider 失败: %v", err)
	}
	defer provider.Close()

	// 2. 模拟配置变更
	logger := provider.WithContext(ctx)
	logger.Info("动态配置演示开始")

	// 3. 模拟配置监听（实际项目中会通过 coord 配置中心）
	go func() {
		time.Sleep(2 * time.Second)
		logger.Info("模拟配置变更：日志级别从 debug 改为 info")

		// 在实际项目中，这里会通过 coord 配置中心更新配置
		// provider 会自动监听变更并更新 logger
	}()

	logger.Info("动态配置演示完成")
}

// demonstrateHealthCheck 演示健康检查
func demonstrateHealthCheck() {
	fmt.Println("\n--- 场景 8: 健康检查 ---")

	ctx := context.Background()

	// 1. 执行健康检查
	if err := clog.HealthCheck(ctx); err != nil {
		log.Printf("健康检查失败: %v", err)
	} else {
		clog.Info("健康检查通过", clog.Component("clog"))
	}

	// 2. 创建带有 coord 的 Provider 进行健康检查
	config := clog.GetDefaultConfig("production")
	provider, err := clog.New(ctx, config, clog.WithNamespace("health-check-demo"))
	if err != nil {
		log.Printf("创建 Provider 失败: %v", err)
		return
	}
	defer provider.Close()

	// 3. 检查 Provider 健康状态
	if p, ok := provider.(*clog.ClogProvider); ok {
		if err := p.HealthCheck(ctx); err != nil {
			clog.Error("Provider 健康检查失败", clog.Err(err))
		} else {
			clog.Info("Provider 健康检查通过", clog.Component("clog-provider"))
		}
	}

	fmt.Println("健康检查演示完成")
}

// UserService 实现
func (s *UserService) ProcessUser(ctx context.Context, userID string) error {
	s.logger.Info("处理用户数据",
		clog.UserID(userID),
		clog.Operation("process_user"))

	// 模拟数据库查询
	time.Sleep(100 * time.Millisecond)

	// 模拟偶尔的错误
	if time.Now().Unix()%5 == 0 {
		return fmt.Errorf("用户不存在: %s", userID)
	}

	s.logger.Info("用户数据处理完成",
		clog.UserID(userID),
		clog.Bool("success", true))

	return nil
}

// OrderService 实现
func (s *OrderService) CreateOrder(ctx context.Context, userID string, amount float64) error {
	s.logger.Info("创建订单",
		clog.UserID(userID),
		clog.Float64("amount", amount),
		clog.Operation("create_order"))

	// 模拟业务处理
	time.Sleep(150 * time.Millisecond)

	s.logger.Info("订单创建成功",
		clog.String("order_id", uuid.NewString()),
		clog.Bool("success", true))

	return nil
}
