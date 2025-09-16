# clog - infra-kit 结构化日志组件

clog 是 infra-kit 项目的官方结构化日志组件，基于 uber-go/zap 构建。它提供了一个**简洁、高性能、上下文感知**的日志解决方案，完全符合 infra-kit 的开发标准。

## 🚀 快速开始

### 服务初始化

```go
import (
    "context"
    "github.com/ceyewan/infra-kit/clog"
)

// 使用环境相关的默认配置初始化
config := clog.GetDefaultConfig("production")
if err := clog.Init(context.Background(), config, clog.WithNamespace("my-service")); err != nil {
    log.Fatal(err)
}

clog.Info("服务启动成功")
// 输出: {"namespace": "my-service", "msg": "服务启动成功"}
```

### 基础使用

```go
// 全局日志器方法
clog.Info("用户登录", clog.String("user_id", "12345"))
clog.Warn("连接超时", clog.Int("timeout", 30))
clog.Error("数据库连接失败", clog.Err(err))
clog.Fatal("致命错误，程序退出", clog.String("reason", "配置错误"))
```

### 层次化命名空间

```go
// 可链式调用的层次化命名空间
userLogger := clog.Namespace("user")
authLogger := userLogger.Namespace("auth")
dbLogger := userLogger.Namespace("database")

userLogger.Info("开始用户注册", clog.String("email", "user@example.com"))
// 输出: {"namespace": "user", "msg": "开始用户注册", "email": "user@example.com"}

authLogger.Info("验证密码强度")
// 输出: {"namespace": "user.auth", "msg": "验证密码强度"}

dbLogger.Info("检查用户是否存在")
// 输出: {"namespace": "user.database", "msg": "检查用户是否存在"}
```

### 上下文感知日志

```go
// 在中间件中注入 TraceID
ctx := clog.WithTraceID(context.Background(), "abc123-def456")

// 在业务代码中自动获取带 TraceID 的日志器
logger := clog.WithContext(ctx)
logger.Info("处理请求", clog.String("method", "POST"))
// 输出: {"trace_id": "abc123-def456", "msg": "处理请求", "method": "POST"}

// 简短别名
clog.WithContext(ctx).Info("请求完成")
```

### Provider 模式创建独立日志器

```go
// 创建独立日志器实例
config := &clog.Config{
    Level:       "debug",
    Format:      "json",
    Output:      "/app/logs/app.log",
    AddSource:   true,
    EnableColor: false,
}

logger, err := clog.New(context.Background(), config, clog.WithNamespace("payment-service"))
if err != nil {
    log.Fatal(err)
}

logger.Info("独立日志器初始化完成")
```

## 📋 API 参考

### Provider 模式接口

```go
// 标准 Provider 签名，遵循 infra-kit 规范
func New(ctx context.Context, config *Config, opts ...Option) (Logger, error)
func Init(ctx context.Context, config *Config, opts ...Option) error
func GetDefaultConfig(env string) *Config  // "development" 或 "production"
```

### 全局日志方法

```go
clog.Debug(msg string, fields ...Field)   // 调试信息
clog.Info(msg string, fields ...Field)    // 一般信息
clog.Warn(msg string, fields ...Field)    // 警告
clog.Error(msg string, fields ...Field)   // 错误
clog.Fatal(msg string, fields ...Field)   // 致命错误（退出程序）
```

### 层次化命名空间

```go
// 创建命名空间日志器，可链式调用
func Namespace(name string) Logger

// 示例: 深度链式调用
logger := clog.Namespace("payment").Namespace("processor").Namespace("stripe")
```

### 上下文感知日志

```go
// 类型安全的 TraceID 注入
func WithTraceID(ctx context.Context, traceID string) context.Context

// 从上下文获取日志器（如果存在 trace_id 则自动添加）
func WithContext(ctx context.Context) Logger
```

### 函数式选项

```go
// 设置根命名空间
func WithNamespace(name string) Option
```

### 结构化字段构造器（zap.Field 别名）

```go
clog.String(key, value string) Field
clog.Int(key string, value int64) Field
clog.Bool(key string, value bool) Field
clog.Float64(key string, value float64) Field
clog.Duration(key string, value time.Duration) Field
clog.Time(key string, value time.Time) Field
clog.Err(err error) Field
clog.Any(key string, value interface{}) Field
```

## ⚙️ 配置

```go
type Config struct {
    Level       string           `json:"level"`      // "debug", "info", "warn", "error", "fatal"
    Format      string           `json:"format"`     // "json" (生产) 或 "console" (开发)
    Output      string           `json:"output"`     // "stdout", "stderr" 或文件路径
    AddSource   bool             `json:"add_source"` // 包含源文件:行号
    EnableColor bool             `json:"enable_color"` // 控制台颜色
    RootPath    string           `json:"root_path"`  // 项目根路径用于路径显示
    Rotation    *RotationConfig  `json:"rotation"`   // 文件轮转（如果 Output 是文件）
}

type RotationConfig struct {
    MaxSize    int  `json:"maxSize"`    // 最大文件大小 (MB)
    MaxBackups int  `json:"maxBackups"` // 最大备份文件数
    MaxAge     int  `json:"maxAge"`     // 保留天数
    Compress   bool `json:"compress"`   // 压缩轮转文件
}
```

### 环境相关默认值

```go
// 开发环境: 控制台，调试，带颜色
devConfig := clog.GetDefaultConfig("development")

// 生产环境: JSON，信息，无颜色
prodConfig := clog.GetDefaultConfig("production")
```

## 📝 使用示例

### 1. 服务初始化（推荐）

```go
func main() {
    config := clog.GetDefaultConfig("production")
    if err := clog.Init(context.Background(), config, clog.WithNamespace("my-service")); err != nil {
        log.Fatal(err)
    }
    clog.Info("服务启动")
}
```

### 2. Gin 中间件集成

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/ceyewan/infra-kit/clog"
)

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

func handler(c *gin.Context) {
    logger := clog.WithContext(c.Request.Context())
    logger.Info("处理请求", clog.String("path", c.Request.URL.Path))
}
```

### 3. 业务逻辑中的层次化命名空间

```go
func (s *PaymentService) ProcessPayment(ctx context.Context, req *PaymentRequest) error {
    logger := clog.WithContext(ctx)
    logger.Info("开始处理支付", clog.String("order_id", req.OrderID))
    
    validationLogger := logger.Namespace("validation")
    validationLogger.Info("验证支付数据")
    
    processorLogger := logger.Namespace("processor").Namespace("stripe")
    processorLogger.Info("调用 Stripe API")
    
    return nil
}
```

### 4. 带轮转的文件输出

```go
config := &clog.Config{
    Level:    "info",
    Format:   "json",
    Output:   "/app/logs/app.log",
    Rotation: &clog.RotationConfig{
        MaxSize:    100,  // 100MB
        MaxBackups: 3,
        MaxAge:     7,
        Compress:   true,
    },
}

clog.Init(context.Background(), config)
```

### 5. 高级日志轮转示例

#### 基础轮转
```go
// 简单轮转配置
config := &clog.Config{
    Level:  "info",
    Format: "json",
    Output: "./logs/app.log",
    Rotation: &clog.RotationConfig{
        MaxSize:    10,   // 每个文件 10MB
        MaxBackups: 3,    // 保留 3 个备份文件
        MaxAge:     7,    // 删除超过 7 天的文件
        Compress:   false, // 调试时不压缩
    },
}

if err := clog.Init(context.Background(), config); err != nil {
    log.Fatal(err)
}

// 生成日志测试轮转
for i := 0; i < 1000; i++ {
    clog.Info("测试日志消息", clog.Int("counter", i))
}
```

#### 生产环境压缩轮转
```go
// 生产环境压缩轮转配置
config := &clog.Config{
    Level:    "info",
    Format:   "json",
    Output:   "/var/log/myapp/app.log",
    AddSource: true,
    Rotation: &clog.RotationConfig{
        MaxSize:    100,  // 每个文件 100MB
        MaxBackups: 5,    // 保留 5 个备份文件
        MaxAge:     30,   // 保留文件 30 天
        Compress:   true, // 压缩轮转文件
    },
}

logger, err := clog.New(context.Background(), config, clog.WithNamespace("production"))
if err != nil {
    log.Fatal(err)
}

// 使用日志器
logger.Info("生产服务启动")
```

#### 大量日志的积极轮转
```go
// 大量日志服务的积极轮转
config := &clog.Config{
    Level:    "info",
    Format:   "json",
    Output:   "/app/logs/high-volume.log",
    Rotation: &clog.RotationConfig{
        MaxSize:    50,   // 50MB - 较小文件便于管理
        MaxBackups: 10,   // 保留更多备份用于审计
        MaxAge:     7,    // 大量数据的短保留期
        Compress:   true, // 节省空间的关键
    },
}

if err := clog.Init(context.Background(), config); err != nil {
    log.Fatal(err)
}

// 模拟大量日志记录
for i := 0; i < 10000; i++ {
    clog.Info("处理交易", 
        clog.String("tx_id", fmt.Sprintf("tx-%d", i)),
        clog.Float64("amount", rand.Float64()*1000),
        clog.Time("timestamp", time.Now()),
    )
    time.Sleep(time.Millisecond * 10) // 每秒 100 笔交易
}
```

#### 轮转监控和清理
```go
// 监控轮转事件和日志状态
func monitorRotation(ctx context.Context, logger clog.Logger) {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // 检查日志文件大小
            if stat, err := os.Stat("./logs/app.log"); err == nil {
                logger.Info("日志文件状态",
                    clog.String("file", "./logs/app.log"),
                    clog.Int64("size_bytes", stat.Size()),
                    clog.String("size_mb", fmt.Sprintf("%.2f", float64(stat.Size())/1024/1024)),
                )
            }
            
            // 列出备份文件
            if files, err := filepath.Glob("./logs/app.log.*"); err == nil {
                logger.Info("备份文件数量", clog.Int("count", len(files)))
            }
            
        case <-ctx.Done():
            return
        }
    }
}

// 在单独的 goroutine 中启动监控
go monitorRotation(context.Background(), clog.Namespace("monitor"))
```

### 6. 上下文传播最佳实践

```go
func processUserRequest(ctx context.Context, userID string) error {
    logger := clog.WithContext(ctx)
    logger.Info("处理用户请求", clog.String("user_id", userID))
    
    if err := validateUser(ctx, userID); err != nil {
        logger.Error("验证失败", clog.Err(err))
        return err
    }
    
    logger.Info("请求完成")
    return nil
}

func validateUser(ctx context.Context, userID string) error {
    logger := clog.WithContext(ctx).Namespace("validation")
    logger.Info("验证用户", clog.String("user_id", userID))
    // 验证逻辑...
    return nil
}
```

## 🎯 核心特性

- **标准兼容**: 遵循 infra-kit Provider 模式
- **上下文感知**: 自动提取 trace_id 进行分布式追踪
- **层次化命名空间**: 可链式调用，清晰的模块边界
- **类型安全**: 封装的上下文键，编译时检查
- **环境感知**: 开发和生产环境的优化默认值
- **高性能**: 通过 zap 实现零分配
- **可观测性**: 完整的命名空间路径用于过滤/分析
- **内置轮转**: 基于 lumberjack 的自动日志文件轮转
- **可配置保留**: 对日志文件生命周期的精细控制
- **压缩支持**: 可选的轮转文件压缩
- **Built-in Rotation**: Automatic log file rotation with lumberjack integration.
- **Configurable Retention**: Fine-grained control over log file lifecycle.
- **Compression Support**: Optional compression of rotated log files.

## 🔄 Log Rotation Features

### Automatic Rotation
clog provides built-in log rotation using the lumberjack library, requiring no external dependencies. The rotation is automatically triggered when log files reach the configured size limit.

### Configuration Options
- **MaxSize**: Maximum file size in megabytes before rotation (default: 100MB)
- **MaxBackups**: Maximum number of backup files to retain (default: 3)
- **MaxAge**: Maximum age of backup files in days (default: 7 days)
- **Compress**: Whether to compress rotated files using gzip (default: false)

### File Management
- **Current Log**: Active log file with the specified filename
- **Rotated Files**: Timestamp-suffixed backup files (e.g., `app.log.2024-01-15-14-30-00`)
- **Compressed Files**: `.gz` extension for compressed backups
- **Automatic Cleanup**: Old files are automatically deleted based on retention policies

### Performance Optimized
- **Atomic Operations**: Lock-free file rotation prevents log loss
- **Background Compression**: Non-blocking compression of rotated files
- **Buffered Writing**: Efficient I/O operations for optimal performance

## 🔄 日志轮转特性

### 自动轮转
clog 使用 lumberjack 库提供内置日志轮转，无需外部依赖。当日志文件达到配置的大小限制时自动触发轮转。

### 配置选项
- **MaxSize**: 轮转前的最大文件大小（MB，默认: 100MB）
- **MaxBackups**: 保留的最大备份文件数（默认: 3）
- **MaxAge**: 备份文件的最大天数（默认: 7 天）
- **Compress**: 是否使用 gzip 压缩轮转文件（默认: false）

### 文件管理
- **当前日志**: 具有指定文件名的活动日志文件
- **轮转文件**: 带时间戳后缀的备份文件（如 `app.log.2024-01-15-14-30-00`）
- **压缩文件**: 压缩备份的 `.gz` 扩展名
- **自动清理**: 基于保留策略自动删除旧文件

### 性能优化
- **原子操作**: 无锁文件轮转防止日志丢失
- **后台压缩**: 轮转文件的非阻塞压缩
- **缓冲写入**: 最佳性能的高效 I/O 操作

## 🛠️ 开发和测试

### 测试支持
```go
// 设置测试退出函数
clog.SetExitFunc(func(code int) {
    // 测试中不真正退出
})

// 创建测试日志器
testLogger, _ := clog.New(ctx, &clog.Config{
    Level:  "debug",
    Format: "console",
    Output: "stdout",
})
```

### 性能测试
clog 针对 <1% 的热路径开销进行了基准测试，使用 zap 的零分配引擎确保高性能。

## 📚 相关文档

- **[设计文档](DESIGN.md)**: 详细的架构设计和实现原理
- **[使用指南](../../docs/usage_guide.md)**: 使用示例和最佳实践

## 📄 许可证

MIT License - 详见 [LICENSE](../../LICENSE) 文件