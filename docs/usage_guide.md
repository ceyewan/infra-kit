# GoChat Kit 使用指南

本指南提供完整的使用示例，展示如何在典型的微服务中正确使用 GoChat Kit 的所有组件。

## 架构概览

GoChat Kit 遵循以下核心设计原则：

- **Provider 模式**：所有组件都通过统一的 Provider 接口提供
- **标准构造函数**：`func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)`
- **默认配置**：`func GetDefaultConfig(env string) *Config`
- **配置分离**：config 用于核心配置，opts 用于依赖注入
- **组件自治**：内部监听配置变化，实现热更新
- **上下文感知**：所有 I/O 操作都接受 context.Context 作为第一个参数

## 统一初始化：典型服务架构

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gochat-kit/breaker"
    "github.com/gochat-kit/cache"
    "github.com/gochat-kit/clog"
    "github.com/gochat-kit/coord"
    "github.com/gochat-kit/db"
    "github.com/gochat-kit/es"
    "github.com/gochat-kit/metrics"
    "github.com/gochat-kit/mq"
    "github.com/gochat-kit/once"
    "github.com/gochat-kit/ratelimit"
    "github.com/gochat-kit/uid"
)

const (
    serviceName = "message-service"
    environment = "production"
)

func main() {
    // --- 1. 基础组件初始化 ---
    
    // 初始化日志组件
    clogConfig := clog.GetDefaultConfig(environment)
    clogConfig.ServiceName = serviceName
    logger := clog.New(context.Background(), clogConfig)
    
    logger.Info("服务开始启动...")

    // 初始化监控组件
    metricsConfig := metrics.GetDefaultConfig(serviceName, environment)
    metricsConfig.PrometheusListenAddr = ":9090"
    metricsConfig.ExporterEndpoint = "http://jaeger:14268/api/traces"
    metricsProvider, err := metrics.New(context.Background(), metricsConfig,
        metrics.WithLogger(logger),
    )
    if err != nil {
        logger.Fatal("初始化 metrics 失败", clog.Err(err))
    }
    defer metricsProvider.Shutdown(context.Background())
    logger.Info("metrics Provider 初始化成功")

    // --- 2. 核心依赖组件初始化 ---
    
    // 初始化分布式协调组件
    coordConfig := coord.GetDefaultConfig(environment)
    coordConfig.Endpoints = []string{"etcd1:2379", "etcd2:2379", "etcd3:2379"}
    coordProvider, err := coord.New(context.Background(), coordConfig,
        coord.WithLogger(logger),
    )
    if err != nil {
        logger.Fatal("初始化 coord 失败", clog.Err(err))
    }
    defer coordProvider.Close()
    logger.Info("coord Provider 初始化成功")

    // --- 3. 上层组件初始化 ---
    
    // 初始化缓存组件
    cacheConfig := cache.GetDefaultConfig(environment)
    cacheConfig.Addr = "redis-cluster:6379"
    cacheProvider, err := cache.New(context.Background(), cacheConfig,
        cache.WithLogger(logger),
        cache.WithCoordProvider(coordProvider),
    )
    if err != nil {
        logger.Fatal("初始化 cache 失败", clog.Err(err))
    }
    defer cacheProvider.Close()
    logger.Info("cache Provider 初始化成功")

    // 初始化数据库组件
    dbConfig := db.GetDefaultConfig(environment)
    dbConfig.DSN = "user:password@tcp(mysql:3306)/message_service?charset=utf8mb4&parseTime=True&loc=Local"
    dbProvider, err := db.New(context.Background(), dbConfig,
        db.WithLogger(logger),
    )
    if err != nil {
        logger.Fatal("初始化 db 失败", clog.Err(err))
    }
    defer dbProvider.Close()
    logger.Info("db Provider 初始化成功")

    // 初始化 UID 组件
    uidConfig := uid.GetDefaultConfig(environment)
    uidConfig.ServiceName = serviceName
    uidProvider, err := uid.New(context.Background(), uidConfig,
        uid.WithLogger(logger),
        uid.WithCoordProvider(coordProvider),
    )
    if err != nil {
        logger.Fatal("初始化 uid 失败", clog.Err(err))
    }
    defer uidProvider.Close()
    logger.Info("uid Provider 初始化成功")

    // 初始化消息队列组件
    mqConfig := mq.GetDefaultConfig(environment)
    mqConfig.Brokers = []string{"kafka1:9092", "kafka2:9092", "kafka3:9092"}
    mqProducer, err := mq.NewProducer(context.Background(), mqConfig,
        mq.WithLogger(logger),
        mq.WithCoordProvider(coordProvider),
    )
    if err != nil {
        logger.Fatal("初始化 mq producer 失败", clog.Err(err))
    }
    defer mqProducer.Close()
    logger.Info("mq producer 初始化成功")

    // 初始化限流组件
    ratelimitConfig := ratelimit.GetDefaultConfig(environment)
    ratelimitConfig.ServiceName = serviceName
    ratelimitConfig.RulesPath = "/config/prod/message-service/ratelimit/"
    rateLimitProvider, err := ratelimit.New(context.Background(), ratelimitConfig,
        ratelimit.WithLogger(logger),
        ratelimit.WithCoordProvider(coordProvider),
        ratelimit.WithCacheProvider(cacheProvider),
    )
    if err != nil {
        logger.Fatal("初始化 ratelimit 失败", clog.Err(err))
    }
    defer rateLimitProvider.Close()
    logger.Info("ratelimit Provider 初始化成功")

    // 初始化幂等组件
    onceConfig := once.GetDefaultConfig(environment)
    onceConfig.ServiceName = serviceName
    onceProvider, err := once.New(context.Background(), onceConfig,
        once.WithLogger(logger),
        once.WithCacheProvider(cacheProvider),
    )
    if err != nil {
        logger.Fatal("初始化 once 失败", clog.Err(err))
    }
    defer onceProvider.Close()
    logger.Info("once Provider 初始化成功")

    // 初始化熔断器组件
    breakerConfig := breaker.GetDefaultConfig(serviceName, environment)
    breakerConfig.PoliciesPath = "/config/prod/message-service/breakers/"
    breakerProvider, err := breaker.New(context.Background(), breakerConfig,
        breaker.WithLogger(logger),
        breaker.WithCoordProvider(coordProvider),
    )
    if err != nil {
        logger.Fatal("初始化 breaker 失败", clog.Err(err))
    }
    defer breakerProvider.Close()
    logger.Info("breaker Provider 初始化成功")

    // 初始化搜索引擎组件
    esConfig := es.GetDefaultConfig(environment)
    esConfig.Addresses = []string{"http://elasticsearch:9200"}
    esProvider, err := es.New(context.Background(), esConfig,
        es.WithLogger(logger),
    )
    if err != nil {
        logger.Fatal("初始化 es 失败", clog.Err(err))
    }
    defer esProvider.Close()
    logger.Info("es Provider 初始化成功")

    // --- 4. 启动业务服务 ---
    messageSvc := NewMessageService(
        dbProvider, 
        cacheProvider, 
        mqProducer, 
        uidProvider, 
        rateLimitProvider, 
        onceProvider, 
        breakerProvider, 
        esProvider,
        logger,
    )
    
    // 启动 gRPC/HTTP 服务器
    go func() {
        if err := messageSvc.Start(); err != nil {
            logger.Fatal("服务启动失败", clog.Err(err))
        }
    }()
    
    logger.Info("所有组件初始化完毕，服务正在运行...")

    // --- 5. 优雅关闭 ---
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    logger.Warn("服务开始关闭...")

    if err := messageSvc.Stop(); err != nil {
        logger.Error("服务关闭失败", clog.Err(err))
    }

    logger.Info("服务已优雅关闭")
}
```

## 1. 结构化日志 (clog)

### 基础用法

```go
// 在业务逻辑中使用
func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
    logger := clog.WithContext(ctx)
    userLogger := logger.Namespace("get_user")
    
    userLogger.Info("开始获取用户信息", clog.String("user_id", userID))
    
    user, err := s.userRepo.GetUser(ctx, userID)
    if err != nil {
        userLogger.Error("获取用户信息失败", clog.Err(err))
        return nil, err
    }
    
    userLogger.Info("成功获取用户信息")
    return user, nil
}
```

### 中间件集成

```go
// HTTP 中间件
func TraceMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        traceID := c.GetHeader("X-Trace-ID")
        if traceID == "" {
            traceID = uuid.New().String()
        }
        
        ctx := clog.WithTraceID(c.Request.Context(), traceID)
        c.Request = c.Request.WithContext(ctx)
        c.Header("X-Trace-ID", traceID)
        
        c.Next()
    }
}
```

## 2. 分布式缓存 (cache)

### 缓存用户信息

```go
func (s *UserService) GetUserProfile(ctx context.Context, userID string) (*Profile, error) {
    logger := clog.WithContext(ctx)
    key := fmt.Sprintf("user:%s:profile", userID)
    
    // 尝试从缓存获取
    profileJSON, err := s.cache.String().Get(ctx, key)
    if err == nil {
        var profile Profile
        if json.Unmarshal([]byte(profileJSON), &profile) == nil {
            logger.Info("用户资料缓存命中", clog.String("user_id", userID))
            return &profile, nil
        }
    }
    
    // 缓存未命中，从数据库获取
    profile, err := s.getUserFromDB(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // 写入缓存
    profileData, _ := json.Marshal(profile)
    if err := s.cache.String().Set(ctx, key, profileData, time.Hour); err != nil {
        logger.Error("写入缓存失败", clog.Err(err))
    }
    
    return profile, nil
}
```

### 分布式锁使用

```go
func (s *OrderService) ProcessOrder(ctx context.Context, orderID string) error {
    logger := clog.WithContext(ctx)
    
    // 获取分布式锁
    lock, err := s.cache.Lock().Acquire(ctx, fmt.Sprintf("lock:order:%s", orderID), 30*time.Second)
    if err != nil {
        return fmt.Errorf("获取订单处理锁失败: %w", err)
    }
    defer lock.Unlock(ctx)
    
    logger.Info("开始处理订单", clog.String("order_id", orderID))
    
    // 处理订单逻辑
    if err := s.processOrderLogic(ctx, orderID); err != nil {
        return fmt.Errorf("处理订单失败: %w", err)
    }
    
    logger.Info("订单处理完成", clog.String("order_id", orderID))
    return nil
}
```

## 3. 唯一 ID 生成 (uid)

### 生成业务 ID

```go
func (s *MessageService) CreateMessage(ctx context.Context, content string) (*Message, error) {
    logger := clog.WithContext(ctx)
    
    // 生成消息 ID
    messageID, err := s.uid.GenerateSnowflake()
    if err != nil {
        return nil, fmt.Errorf("生成消息ID失败: %w", err)
    }
    
    message := &Message{
        ID:        messageID,
        Content:   content,
        CreatedAt: time.Now(),
    }
    
    // 保存消息
    if err := s.messageRepo.CreateMessage(ctx, message); err != nil {
        return nil, fmt.Errorf("保存消息失败: %w", err)
    }
    
    logger.Info("消息创建成功", clog.Int64("message_id", messageID))
    return message, nil
}
```

### 生成请求 ID

```go
func RequestIDMiddleware(uidProvider uid.Provider) gin.HandlerFunc {
    return func(c *gin.Context) {
        requestID := c.GetHeader("X-Request-ID")
        
        if requestID == "" || !uidProvider.IsValidUUID(requestID) {
            requestID = uidProvider.GetUUIDV7()
        }
        
        c.Header("X-Request-ID", requestID)
        ctx := clog.WithTraceID(c.Request.Context(), requestID)
        c.Request = c.Request.WithContext(ctx)
        
        c.Next()
    }
}
```

## 4. 分布式限流 (ratelimit)

### API 限流

```go
func (s *APIService) SendSMS(ctx context.Context, req *SendSMSRequest) error {
    logger := clog.WithContext(ctx)
    
    // 检查用户发送频率
    allowed, err := s.rateLimit.Allow(ctx, fmt.Sprintf("user:%s:sms", req.UserID), "send_sms")
    if err != nil {
        logger.Error("限流检查失败", clog.Err(err))
        return fmt.Errorf("服务异常，请稍后重试")
    }
    
    if !allowed {
        return fmt.Errorf("发送过于频繁，请稍后重试")
    }
    
    // 发送短信逻辑
    if err := s.sendSMSLogic(ctx, req); err != nil {
        return fmt.Errorf("发送短信失败: %w", err)
    }
    
    logger.Info("短信发送成功", clog.String("user_id", req.UserID))
    return nil
}
```

## 5. 分布式幂等 (once)

### 支付处理幂等

```go
func (s *PaymentService) ProcessPayment(ctx context.Context, req *PaymentRequest) error {
    logger := clog.WithContext(ctx)
    
    // 使用幂等组件保证支付处理只执行一次
    err := s.once.Do(ctx, fmt.Sprintf("payment:order:%s", req.OrderID), 24*time.Hour, func() error {
        logger.Info("开始处理支付", clog.String("order_id", req.OrderID))
        
        // 执行支付逻辑
        if err := s.processPaymentLogic(ctx, req); err != nil {
            return fmt.Errorf("支付处理失败: %w", err)
        }
        
        logger.Info("支付处理成功", clog.String("order_id", req.OrderID))
        return nil
    })
    
    if err != nil {
        return fmt.Errorf("支付处理失败: %w", err)
    }
    
    return nil
}
```

### 带结果缓存的幂等操作

```go
func (s *DocumentService) GenerateReport(ctx context.Context, req *GenerateReportRequest) (*Report, error) {
    logger := clog.WithContext(ctx)
    
    // 使用带结果缓存的幂等操作
    result, err := s.once.Execute(ctx, fmt.Sprintf("report:%s", req.ReportID), 2*time.Hour, func() (interface{}, error) {
        logger.Info("开始生成报告", clog.String("report_id", req.ReportID))
        
        // 生成报告
        report, err := s.generateReportLogic(ctx, req)
        if err != nil {
            return nil, fmt.Errorf("生成报告失败: %w", err)
        }
        
        logger.Info("报告生成成功", clog.String("report_id", req.ReportID))
        return report, nil
    })
    
    if err != nil {
        return nil, fmt.Errorf("生成报告失败: %w", err)
    }
    
    return result.(*Report), nil
}
```

## 6. 熔断器 (breaker)

### gRPC 调用保护

```go
func (s *UserService) GetUserInfoFromRemote(ctx context.Context, userID string) (*UserInfo, error) {
    logger := clog.WithContext(ctx)
    
    // 获取熔断器实例
    b := s.breaker.GetBreaker("grpc:user-service:GetUserInfo")
    
    // 将操作包裹在熔断器中执行
    var userInfo *UserInfo
    err := b.Do(ctx, func() error {
        // 调用远程服务
        info, err := s.userClient.GetUserInfo(ctx, &pb.GetUserInfoRequest{UserId: userID})
        if err != nil {
            return err
        }
        userInfo = &UserInfo{
            ID:       info.Id,
            Username: info.Username,
            Email:    info.Email,
        }
        return nil
    })
    
    // 处理熔断器错误
    if errors.Is(err, breaker.ErrBreakerOpen) {
        logger.Warn("用户服务熔断器打开，执行降级逻辑", clog.String("user_id", userID))
        return s.getUserInfoFromCache(ctx, userID)
    }
    
    if err != nil {
        return nil, fmt.Errorf("获取用户信息失败: %w", err)
    }
    
    return userInfo, nil
}
```

## 7. 分布式协调 (coord)

### 服务发现

```go
func (s *UserService) initUserClient() error {
    // 通过服务发现获取连接
    conn, err := s.coord.Registry().GetConnection(context.Background(), "user-service")
    if err != nil {
        return fmt.Errorf("获取用户服务连接失败: %w", err)
    }
    
    s.userClient = pb.NewUserServiceClient(conn)
    return nil
}
```

### 配置监听

```go
func (s *ConfigService) watchConfigChanges(ctx context.Context) {
    // 监听配置变更
    watcher, err := s.coord.Config().WatchPrefix(ctx, "/config/app/", &s.config)
    if err != nil {
        s.logger.Error("创建配置监听器失败", clog.Err(err))
        return
    }
    defer watcher.Close()
    
    for event := range watcher.Changes() {
        s.logger.Info("配置变更", clog.String("key", event.Key))
        s.handleConfigChange(event.Key, event.Value)
    }
}
```

## 8. 消息队列 (mq)

### 消息生产

```go
func (s *NotificationService) SendNotification(ctx context.Context, req *NotificationRequest) error {
    logger := clog.WithContext(ctx)
    
    // 构建消息
    message := &mq.Message{
        Topic: "notifications.email",
        Key:   []byte(req.UserID),
        Value: func() []byte {
            data, _ := json.Marshal(req)
            return data
        }(),
        Headers: map[string]string{
            "message_type": "email",
            "priority":     "normal",
        },
    }
    
    // 异步发送
    err := s.mqProducer.Send(ctx, message, func(err error) {
        if err != nil {
            logger.Error("发送通知消息失败", clog.Err(err))
        } else {
            logger.Info("通知消息发送成功", clog.String("user_id", req.UserID))
        }
    })
    
    if err != nil {
        return fmt.Errorf("发送通知失败: %w", err)
    }
    
    return nil
}
```

### 消息消费

```go
func (s *EmailService) StartEmailConsumer(ctx context.Context) error {
    // 创建消费者
    consumer, err := mq.NewConsumer(ctx, s.mqConfig, "email-service")
    if err != nil {
        return fmt.Errorf("创建消费者失败: %w", err)
    }
    
    // 订阅主题
    topics := []string{"notifications.email"}
    err = consumer.Subscribe(ctx, topics, func(ctx context.Context, msg *mq.Message) error {
        logger := clog.WithContext(ctx)
        
        var req NotificationRequest
        if err := json.Unmarshal(msg.Value, &req); err != nil {
            logger.Error("解析消息失败", clog.Err(err))
            return err
        }
        
        // 处理邮件发送
        if err := s.sendEmail(ctx, &req); err != nil {
            logger.Error("发送邮件失败", clog.Err(err))
            return err
        }
        
        logger.Info("邮件发送成功", clog.String("user_id", req.UserID))
        return nil
    })
    
    if err != nil {
        return fmt.Errorf("订阅消息失败: %w", err)
    }
    
    return nil
}
```

## 9. 搜索引擎 (es)

### 文档索引

```go
// Message 实现了 es.Indexable 接口
func (m *Message) GetID() string {
    return m.ID
}

func (s *SearchService) IndexMessages(ctx context.Context, messages []*Message) error {
    logger := clog.WithContext(ctx)
    
    // 批量索引消息
    err := s.esProvider.BulkIndex(ctx, messages)
    if err != nil {
        logger.Error("批量索引消息失败", clog.Err(err))
        return fmt.Errorf("索引消息失败: %w", err)
    }
    
    logger.Info("消息索引成功", clog.Int("count", len(messages)))
    return nil
}
```

### 搜索功能

```go
func (s *SearchService) SearchMessages(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
    logger := clog.WithContext(ctx)
    
    // 在会话内搜索消息
    results, err := s.esProvider.SearchInSession[Message](ctx, req.UserID, req.SessionID, req.Keyword, req.Page, req.PageSize)
    if err != nil {
        logger.Error("搜索消息失败", clog.Err(err))
        return nil, fmt.Errorf("搜索消息失败: %w", err)
    }
    
    logger.Info("消息搜索成功", 
        clog.String("keyword", req.Keyword),
        clog.Int("total", int(results.Total)))
    
    return &SearchResult{
        Messages: results.Messages,
        Total:    results.Total,
    }, nil
}
```

## 10. 监控指标 (metrics)

### gRPC 服务器集成

```go
func (s *Server) StartGRPCServer() error {
    // 创建 gRPC 服务器
    s.grpcServer = grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            s.metricsProvider.GRPCServerInterceptor(),
            s.traceInterceptor,
        ),
    )
    
    // 注册服务
    pb.RegisterMessageServiceServer(s.grpcServer, s.messageService)
    
    // 启动服务器
    lis, err := net.Listen("tcp", s.config.GRPCPort)
    if err != nil {
        return fmt.Errorf("监听失败: %w", err)
    }
    
    go func() {
        if err := s.grpcServer.Serve(lis); err != nil {
            s.logger.Error("gRPC 服务器错误", clog.Err(err))
        }
    }()
    
    return nil
}
```

### HTTP 服务器集成

```go
func (s *Server) StartHTTPServer() error {
    // 创建 Gin 引擎
    engine := gin.New()
    
    // 使用监控中间件
    engine.Use(s.metricsProvider.HTTPMiddleware())
    
    // 注册路由
    s.registerRoutes(engine)
    
    // 启动服务器
    go func() {
        if err := engine.Run(s.config.HTTPPort); err != nil {
            s.logger.Error("HTTP 服务器错误", clog.Err(err))
        }
    }()
    
    return nil
}
```

## 11. 业务服务集成示例

### 完整的业务服务

```go
type MessageService struct {
    dbProvider       db.Provider
    cacheProvider    cache.Provider
    mqProducer       mq.Producer
    uidProvider      uid.Provider
    rateLimitProvider ratelimit.Provider
    onceProvider     once.Provider
    breakerProvider  breaker.Provider
    esProvider       es.Provider
    logger           clog.Logger
}

func NewMessageService(
    dbProvider db.Provider,
    cacheProvider cache.Provider,
    mqProducer mq.Producer,
    uidProvider uid.Provider,
    rateLimitProvider ratelimit.Provider,
    onceProvider once.Provider,
    breakerProvider breaker.Provider,
    esProvider es.Provider,
    logger clog.Logger,
) *MessageService {
    return &MessageService{
        dbProvider:       dbProvider,
        cacheProvider:    cacheProvider,
        mqProducer:       mqProducer,
        uidProvider:      uidProvider,
        rateLimitProvider: rateLimitProvider,
        onceProvider:     onceProvider,
        breakerProvider:  breakerProvider,
        esProvider:       esProvider,
        logger:           logger,
    }
}

func (s *MessageService) SendMessage(ctx context.Context, req *SendMessageRequest) (*SendMessageResponse, error) {
    logger := clog.WithContext(ctx)
    
    // 1. 限流检查
    allowed, err := s.rateLimitProvider.Allow(ctx, fmt.Sprintf("user:%s:send_message", req.UserID), "user_send_message")
    if err != nil {
        logger.Error("限流检查失败", clog.Err(err))
        return nil, fmt.Errorf("服务异常，请稍后重试")
    }
    
    if !allowed {
        return nil, fmt.Errorf("发送过于频繁，请稍后重试")
    }
    
    // 2. 幂等处理
    var message *Message
    err = s.onceProvider.Do(ctx, fmt.Sprintf("send_message:%s", req.ID), time.Hour, func() error {
        // 生成消息ID
        messageID, err := s.uidProvider.GenerateSnowflake()
        if err != nil {
            return fmt.Errorf("生成消息ID失败: %w", err)
        }
        
        // 创建消息
        message = &Message{
            ID:        fmt.Sprintf("%d", messageID),
            UserID:    req.UserID,
            Content:   req.Content,
            SessionID: req.SessionID,
            CreatedAt: time.Now(),
        }
        
        // 保存到数据库
        if err := s.dbProvider.DB(ctx).Create(message).Error; err != nil {
            return fmt.Errorf("保存消息失败: %w", err)
        }
        
        // 缓存消息
        cacheKey := fmt.Sprintf("message:%s", message.ID)
        messageData, _ := json.Marshal(message)
        if err := s.cacheProvider.String().Set(ctx, cacheKey, messageData, time.Hour); err != nil {
            logger.Error("缓存消息失败", clog.Err(err))
        }
        
        // 索引到ES
        if err := s.esProvider.BulkIndex(ctx, []*Message{message}); err != nil {
            logger.Error("索引消息失败", clog.Err(err))
        }
        
        // 发送消息到MQ
        mqMessage := &mq.Message{
            Topic: "messages.new",
            Key:   []byte(message.ID),
            Value: func() []byte {
                data, _ := json.Marshal(message)
                return data
            }(),
        }
        
        if err := s.mqProducer.Send(ctx, mqMessage, func(err error) {
            if err != nil {
                logger.Error("发送消息到MQ失败", clog.Err(err))
            }
        }); err != nil {
            logger.Error("发送消息到MQ失败", clog.Err(err))
        }
        
        logger.Info("消息发送成功", clog.String("message_id", message.ID))
        return nil
    })
    
    if err != nil {
        return nil, fmt.Errorf("发送消息失败: %w", err)
    }
    
    return &SendMessageResponse{
        MessageID: message.ID,
        Success:   true,
    }, nil
}
```

## 最佳实践总结

### 1. 组件初始化顺序

1. **基础组件**: clog, metrics
2. **核心依赖**: coord
3. **业务组件**: cache, db, mq, uid
4. **服务治理**: ratelimit, once, breaker
5. **扩展组件**: es

### 2. Provider 模式使用

- **统一接口**: 所有组件都实现 Provider 接口
- **标准构造**: 使用 `New(ctx, config, opts...)` 构造
- **依赖注入**: 通过 opts 注入依赖组件
- **资源管理**: 调用 Close() 方法释放资源

### 3. 错误处理策略

- **网络错误**: 组件内置重试机制
- **配置错误**: 启动时快速失败
- **业务异常**: 提供明确的错误类型
- **降级处理**: 提供降级逻辑

### 4. 性能优化建议

- **连接池**: 合理配置连接池大小
- **缓存策略**: 使用多级缓存
- **批量操作**: 尽可能使用批量操作
- **异步处理**: 使用异步消息队列

### 5. 监控和日志

- **结构化日志**: 使用 clog 进行日志记录
- **链路追踪**: 自动传播 TraceID
- **性能监控**: 使用 metrics 进行监控
- **健康检查**: 定期检查组件健康状态

### 6. 配置管理

- **配置分离**: config 用于静态配置，opts 用于动态配置
- **环境适配**: 使用 GetDefaultConfig 获取环境相关配置
- **热更新**: 通过 coord 组件实现配置热更新
- **配置验证**: 启动时验证配置的正确性

---

遵循这些最佳实践可以确保系统的稳定性、可维护性和高性能。所有组件都遵循统一的 Provider 模式，提供一致的使用体验和强大的功能支持。