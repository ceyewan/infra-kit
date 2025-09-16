# 基础设施: clog 结构化日志

## 1. 设计理念

`clog` 是 `infra-kit` 项目的结构化日志组件，基于 `uber-go/zap` 构建。它为微服务架构提供了统一、高性能、上下文感知的日志解决方案。

### 核心设计原则

- **统一接口**: 通过 Provider 模式提供一致的日志体验，支持全局和实例化两种使用方式
- **层次化命名空间**: 支持链式调用构建清晰的日志标识体系，便于服务、模块、组件的分层管理
- **上下文感知**: 自动从 `context.Context` 中提取 `trace_id`，实现完整的请求链路追踪
- **高性能**: 基于 `zap` 的零内存分配日志记录引擎，对业务性能影响降至最低
- **组件自治**: 支持动态配置，内部监听配置变更并自动更新日志行为

### 组件价值

- **可观测性**: 通过结构化日志和链路追踪，提供系统行为的完整可见性
- **调试效率**: 层次化命名空间和上下文信息，快速定位问题根源
- **运维友好**: JSON 格式输出，便于日志收集、分析和监控
- **开发体验**: 简洁的 API 设计，降低使用门槛

## 2. 核心 API 契约

### 2.1 构造函数与配置

```go
// Config 是 clog 组件的配置结构体
type Config struct {
    Level       string `json:"level"`        // 日志级别: debug, info, warn, error, fatal
    Format      string `json:"format"`       // 输出格式: json 或 console
    Output      string `json:"output"`       // 输出目标: stdout, stderr 或文件路径
    AddSource   bool   `json:"addSource"`    // 是否添加源代码信息
    EnableColor bool   `json:"enableColor"`  // 是否启用颜色输出
    Rotation    struct {
        MaxSize    int    `json:"maxSize"`    // 单个日志文件最大大小(MB)
        MaxAge     int    `json:"maxAge"`     // 日志文件最大保存天数
        MaxBackups int    `json:"maxBackups"` // 最大备份文件数量
        Compress   bool   `json:"compress"`   // 是否压缩备份文件
    } `json:"rotation"`
}

// GetDefaultConfig 返回环境相关的默认配置
func GetDefaultConfig(env string) *Config

// Option 功能选项
type Option func(*options)

// WithNamespace 设置根命名空间
func WithNamespace(name string) Option

// WithLogger 注入日志依赖（用于内部组件）
func WithLogger(logger Logger) Option

// WithCoordProvider 注入配置中心依赖，用于动态配置管理
func WithCoordProvider(coord coord.Provider) Option

// New 创建 clog Provider 实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)

// Init 初始化全局默认日志器
func Init(ctx context.Context, config *Config, opts ...Option) error
```

### 2.2 Provider 接口设计

```go
// Provider 是 clog 组件的主接口
type Provider interface {
    // WithContext 从 context 中获取带 trace_id 的日志器
    WithContext(ctx context.Context) Logger
    // WithTraceID 将 trace_id 注入到 context 中
    WithTraceID(ctx context.Context, traceID string) context.Context
    // Namespace 创建带命名空间的日志器
    Namespace(name string) Logger
    // Close 关闭 Provider，释放资源
    Close() error
}

// Logger 定义了日志记录器的核心接口
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    Fatal(msg string, fields ...Field)

    // Namespace 创建子命名空间
    Namespace(name string) Logger
}

// Field 是结构化日志的字段类型
type Field = zap.Field

// 标准字段构造函数
func String(key, val string) Field
func Int(key string, val int) Field
func Int64(key string, val int64) Field
func Float64(key string, val float64) Field
func Bool(key string, val bool) Field
func Time(key string, val time.Time) Field
func Err(err error) Field
func Any(key string, val interface{}) Field
```

### 2.3 动态配置支持

```go
// WatchConfig 监听配置变更
func (p *clogProvider) watchConfigChanges(ctx context.Context) {
    if p.coord == nil {
        return
    }

    configPath := "/config/clog/"
    watcher, err := p.coord.Config().WatchPrefix(ctx, configPath, &p.config)
    if err != nil {
        p.logger.Error("监听 clog 配置失败", clog.Err(err))
        return
    }

    go func() {
        for range watcher.Changes() {
            p.updateLogger()
        }
    }()
}

func (p *clogProvider) updateLogger() {
    // 根据新配置重新创建日志器
    newLogger := p.createLogger(p.config)

    // 原子性地更新日志器引用
    atomic.StorePointer(&p.loggerPtr, unsafe.Pointer(&newLogger))

    p.logger.Info("clog 配置已更新")
}
```

## 3. 实现要点

### 3.1 层次化命名空间实现

```go
type namespacedLogger struct {
    provider *clogProvider
    namespace string
}

func (n *namespacedLogger) Namespace(name string) Logger {
    // 支持链式调用，构建深层命名空间
    if n.namespace == "" {
        return &namespacedLogger{
            provider: n.provider,
            namespace: name,
        }
    }
    return &namespacedLogger{
        provider: n.provider,
        namespace: n.namespace + "." + name,
    }
}

func (n *namespacedLogger) Info(msg string, fields ...Field) {
    // 自动添加命名空间字段
    namespaceField := zap.String("namespace", n.getFullNamespace())
    fields = append([]Field{namespaceField}, fields...)

    // 获取实际的 zap.Logger 并记录
    logger := n.provider.getLogger()
    logger.Info(msg, fields...)
}

func (n *namespacedLogger) getFullNamespace() string {
    // 组合根命名空间和子命名空间
    if n.provider.rootNamespace != "" {
        return n.provider.rootNamespace + "." + n.namespace
    }
    return n.namespace
}
```

### 3.2 上下文与链路追踪实现

```go
// traceIDKey 是 context 中存储 trace_id 的键
type traceIDKey struct{}

// WithTraceID 将 trace_id 注入到 context
func (p *clogProvider) WithTraceID(ctx context.Context, traceID string) context.Context {
    return context.WithValue(ctx, traceIDKey{}, traceID)
}

// WithContext 从 context 中提取 trace_id 并创建日志器
func (p *clogProvider) WithContext(ctx context.Context) Logger {
    // 获取 trace_id
    traceID, _ := ctx.Value(traceIDKey{}).(string)

    // 创建基础日志器
    logger := p.Namespace("")

    // 如果有 trace_id，添加到所有日志中
    if traceID != "" {
        return logger.With(zap.String("trace_id", traceID))
    }

    return logger
}
```

### 3.3 日志器创建与管理

```go
type clogProvider struct {
    config         *Config
    rootNamespace  string
    loggerPtr      unsafe.Pointer // 原子指针，用于动态更新
    coord          coord.Provider
    logger         clog.Logger
}

func (p *clogProvider) createLogger(config *Config) *zap.Logger {
    // 创建 zap 配置
    zapConfig := zap.NewProductionConfig()

    // 根据配置调整
    if config.Format == "console" {
        zapConfig = zap.NewDevelopmentConfig()
    }

    zapConfig.Level = zap.NewAtomicLevelAt(parseLevel(config.Level))
    zapConfig.OutputPaths = []string{config.Output}
    zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    // 创建日志器
    logger, _ := zapConfig.Build()
    return logger
}

func (p *clogProvider) getLogger() *zap.Logger {
    // 原子性地获取当前日志器
    return (*zap.Logger)(atomic.LoadPointer(&p.loggerPtr))
}
```

## 4. 标准用法示例

### 4.1 基础初始化

```go
func main() {
    ctx := context.Background()

    // 1. 初始化日志组件
    config := clog.GetDefaultConfig("production")
    config.Output = "/var/log/app.log"

    clogProvider, err := clog.New(ctx, config,
        clog.WithNamespace("user-service"),
        clog.WithCoordProvider(coordProvider), // 可选，用于动态配置
    )
    if err != nil {
        log.Fatal("clog 初始化失败:", err)
    }
    defer clogProvider.Close()

    // 或者使用全局初始化
    if err := clog.Init(ctx, config, clog.WithNamespace("user-service")); err != nil {
        log.Fatal("clog 初始化失败:", err)
    }
}
```

### 4.2 在 HTTP 中间件中使用

```go
func TraceMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 获取或生成 traceID
        traceID := c.GetHeader("X-Trace-ID")
        if traceID == "" {
            traceID = uuid.New().String()
        }

        // 2. 注入到 context
        ctx := clog.WithTraceID(c.Request.Context(), traceID)
        c.Request = c.Request.WithContext(ctx)

        c.Header("X-Trace-ID", traceID)
        c.Next()
    }
}

func (s *UserService) GetUser(c *gin.Context) {
    ctx := c.Request.Context()
    logger := clog.WithContext(ctx)

    user, err := s.userRepo.GetUser(ctx, c.Param("id"))
    if err != nil {
        logger.Error("获取用户失败",
            clog.String("user_id", c.Param("id")),
            clog.Err(err))
        c.JSON(500, gin.H{"error": "获取用户失败"})
        return
    }

    logger.Info("获取用户成功", clog.String("user_id", user.ID))
    c.JSON(200, user)
}
```

### 4.3 层次化命名空间使用

```go
func (s *OrderService) ProcessOrder(ctx context.Context, orderID string) error {
    logger := clog.WithContext(ctx)

    // 创建不同层次的命名空间
    orderLogger := logger.Namespace("order")
    paymentLogger := orderLogger.Namespace("payment")
    inventoryLogger := orderLogger.Namespace("inventory")

    orderLogger.Info("开始处理订单", clog.String("order_id", orderID))

    // 支付处理
    if err := s.processPayment(ctx, orderID); err != nil {
        paymentLogger.Error("支付处理失败", clog.Err(err))
        return err
    }

    // 库存检查
    if err := s.checkInventory(ctx, orderID); err != nil {
        inventoryLogger.Error("库存检查失败", clog.Err(err))
        return err
    }

    orderLogger.Info("订单处理成功", clog.String("order_id", orderID))
    return nil
}
```

### 4.4 错误处理和调试

```go
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
    logger := clog.WithContext(ctx).Namespace("user").Namespace("create")

    logger.Info("开始创建用户",
        clog.String("email", req.Email),
        clog.String("username", req.Username))

    // 验证输入
    if err := validateUserRequest(req); err != nil {
        logger.Warn("用户请求验证失败", clog.Err(err))
        return nil, fmt.Errorf("验证失败: %w", err)
    }

    // 检查用户是否存在
    exists, err := s.userRepo.UserExists(ctx, req.Email)
    if err != nil {
        logger.Error("检查用户存在性失败", clog.Err(err))
        return nil, fmt.Errorf("检查用户失败: %w", err)
    }

    if exists {
        logger.Warn("用户已存在", clog.String("email", req.Email))
        return nil, fmt.Errorf("用户已存在")
    }

    // 创建用户
    user, err := s.userRepo.CreateUser(ctx, req)
    if err != nil {
        logger.Error("创建用户失败", clog.Err(err))
        return nil, fmt.Errorf("创建用户失败: %w", err)
    }

    logger.Info("用户创建成功",
        clog.String("user_id", user.ID),
        clog.String("email", user.Email))

    return user, nil
}
```

## 5. 性能优化

### 5.1 零内存分配

```go
// 使用对象池减少内存分配
var fieldPool = sync.Pool{
    New: func() interface{} {
        return make([]Field, 0, 10)
    },
}

func (n *namespacedLogger) Info(msg string, fields ...Field) {
    // 从池中获取字段切片
    pooledFields := fieldPool.Get().([]Field)
    defer func() {
        // 重置并归还到池中
        pooledFields = pooledFields[:0]
        fieldPool.Put(pooledFields)
    }()

    // 添加命名空间字段
    pooledFields = append(pooledFields, zap.String("namespace", n.getFullNamespace()))
    pooledFields = append(pooledFields, fields...)

    // 获取实际的 zap.Logger 并记录
    logger := n.provider.getLogger()
    logger.Info(msg, pooledFields...)
}
```

### 5.2 异步日志输出

```go
func createLogger(config *Config) *zap.Logger {
    zapConfig := zap.NewProductionConfig()

    // 启用异步输出以提高性能
    zapConfig.OutputPaths = []string{"stdout"}
    zapConfig.ErrorOutputPaths = []string{"stderr"}

    // 根据环境调整缓冲区大小
    if os.Getenv("GO_ENV") == "production" {
        zapConfig.EncoderConfig.TimeKey = "timestamp"
    }

    logger, _ := zapConfig.Build()
    return logger
}
```

## 6. 监控与维护

### 6.1 日志健康检查

```go
func (p *clogProvider) HealthCheck(ctx context.Context) error {
    // 检查日志器是否正常工作
    logger := p.getLogger()

    // 尝试写入测试日志
    if err := logger.Sync(); err != nil {
        return fmt.Errorf("日志同步失败: %w", err)
    }

    // 检查配置中心连接
    if p.coord != nil {
        if err := p.coord.Ping(ctx); err != nil {
            return fmt.Errorf("配置中心连接失败: %w", err)
        }
    }

    return nil
}
```

### 6.2 日志轮转和清理

```go
func (p *clogProvider) setupRotation(config *Config) {
    if config.Output == "stdout" || config.Output == "stderr" {
        return
    }

    // 创建日志轮转配置
    rotationConfig := rotatelogs.Config{
        Path:         config.Output,
        RotationTime: time.Hour,
        MaxAge:       time.Duration(config.Rotation.MaxAge) * 24 * time.Hour,
        Rotator:      rotatelogs.NewDefaultRotator(),
    }

    // 设置轮转钩子
    logger := p.getLogger()
    logger = logger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
        // 在日志轮转时执行清理操作
        return nil
    }))
}
```

## 7. 最佳实践

### 7.1 日志级别使用

- **DEBUG**: 详细的调试信息，仅在开发环境使用
- **INFO**: 重要的业务事件和状态变化
- **WARN**: 潜在问题，不影响正常运行
- **ERROR**: 错误情况，需要关注但可以恢复
- **FATAL**: 严重错误，导致程序无法继续运行

### 7.2 结构化日志字段

```go
// 推荐的字段命名规范
logger.Info("用户登录成功",
    clog.String("user_id", "12345"),
    clog.String("username", "john_doe"),
    clog.String("ip_address", "192.168.1.100"),
    clog.String("user_agent", "Mozilla/5.0..."),
    clog.Time("login_time", time.Now()),
)
```

### 7.3 性能考虑

- **避免高频日志**: 生产环境避免使用 DEBUG 级别
- **合理使用字段**: 只记录必要的信息，避免过多字段
- **异步处理**: 启用异步日志输出以提高性能
- **资源监控**: 监控日志文件大小和磁盘使用情况

---

*遵循这些指南可以确保日志组件的高质量实现和稳定运行。*
