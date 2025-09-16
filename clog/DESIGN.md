# clog 设计文档

## 🎯 设计目标

clog 是 infra-kit 项目的官方结构化日志组件，基于 uber-go/zap 构建。旨在提供一个**简洁、高性能、上下文感知**的日志解决方案，完全符合 infra-kit 的开发标准。

### 核心设计原则

1. **标准优先**: 严格遵循 infra-kit 组件设计规范，使用标准的 Provider 模式
2. **上下文感知**: 自动从 `context.Context` 中提取 `trace_id`，支持分布式链路追踪
3. **层次化命名空间**: 统一的命名空间系统，支持链式调用，清晰的模块边界
4. **类型安全**: 封装上下文键，避免冲突，提供编译时类型检查
5. **环境感知**: 为开发和生产环境提供优化的默认配置
6. **高性能**: 利用 zap 的零内存分配日志引擎，最小化性能开销
7. **强可观测性**: 完整的命名空间路径和结构化字段，支持精确过滤、分析和请求链可视化

这些原则确保 clog 简单易用、高性能，并能无缝集成到微服务架构中。

## 🏗️ 架构概览

架构采用分层设计，分离关注点：公共 API、配置、核心逻辑、内部实现和 zap 基础。

### 高层架构

```
公共 API 层
├── clog.Info/Warn/Error/Fatal (全局方法)
├── clog.Namespace() (层次化命名空间)
├── clog.WithContext() / C() (上下文感知日志器)
├── clog.WithTraceID() (TraceID 注入)
└── clog.New/Init (Provider 模式)

配置层
├── Config 结构 (Level, Format, Output 等)
├── GetDefaultConfig(env) (环境相关默认值)
├── Option 模式 (如 WithNamespace)
└── ParseOptions() (选项解析)

核心层
├── getDefaultLogger() (单例，支持原子替换)
├── TraceID 管理 (类型安全的上下文键)
└── 全局原子日志器替换

内部层
├── internal.Logger 接口
├── zapLogger 实现 (zap 包装器)
└── 层次化命名空间处理

Zap 基础
├── zap.Logger
├── zapcore.Core
└── zapcore.Encoder (JSON/Console)
```

这种分层设计促进了模块化、可测试性和可扩展性，同时保持了清晰的公共 API。

## 🔧 核心组件

### 1. Provider 模式实现

**目的**: 启用依赖注入，遵循 infra-kit 规范进行组件初始化。

**核心函数**:
- `New(ctx context.Context, config *Config, opts ...Option) (Logger, error)`: 创建独立的日志器实例。`ctx` 控制初始化（日志器不持有 ctx）。选项可自定义行为（如命名空间）。
- `Init(ctx context.Context, config *Config, opts ...Option) error`: 初始化全局默认日志器。如果已初始化则失败（使用替换进行热更新）。
- `GetDefaultConfig(env string) *Config`: 返回优化默认值：
  - "development": 调试级别，控制台格式，启用颜色
  - "production": 信息级别，JSON 格式，禁用颜色

**实现亮点**:
- 将选项解析为结构体用于命名空间注入
- 使用配置创建 zap 日志器（编码器、输出、级别）
- 出错时回退到无操作日志器，实现优雅降级
- 使用 `sync.Once` 和 `atomic.Value` 实现线程安全的全局日志器单例

**设计理念**: Provider 模式确保可配置性，避免全局状态污染。环境默认值减少样板代码，防止开发/生产环境配置错误。

### 2. 层次化命名空间系统

**目的**: 提供统一的方式为日志标记服务/模块/组件路径，替代碎片化的服务/模块 API。

**核心函数**:
- `Namespace(name string) Logger`: 返回子日志器，命名空间追加（如根 "im-gateway" + "user" → "im-gateway.user"）。可链式调用构建深层路径如 "im-gateway.payment.processor.stripe"。

**实现**:
- 命名空间是在日志器创建时添加一次的 zap 字段（`zap.String("namespace", fullPath)`）
- 根通过 init/New 时的 `WithNamespace` 选项设置
- 链式调用创建包装父日志器的新日志器，避免重复字符串操作

**示例输出**:
```json
{"namespace": "im-gateway.user.auth", "trace_id": "abc123", "msg": "密码验证"}
```

**设计理念**:
- **统一概念**: 消除"服务"和"模块"的混淆；一切都是命名空间层
- **灵活性**: 任意深度 vs 固定两层结构
- **可观测性**: 完整路径支持如"过滤 payment.processor.* 的日志"等查询
- **一致性**: 单一 `Namespace()` 方法处理所有层级，减少 API 表面

与传统系统对比：

| 方面          | 传统 (Service + Module) | 层次化命名空间 |
|---------------|------------------------|----------------|
| API 数量     | 2 (WithService + Module) | 1 (WithNamespace + Namespace) |
| 概念复杂度   | 高 (边界模糊)           | 低 (统一)     |
| 可扩展性     | 差 (固定层级)           | 强 (任意深度) |
| 可读性       | 中等                   | 高 (路径类似)  |

### 3. 类型安全的 TraceID 管理

**目的**: 通过链接跨服务的日志实现分布式追踪，无需手动传播。

**核心函数**:
- `WithTraceID(ctx context.Context, traceID string) context.Context`: 使用私有结构键将 traceID 注入 ctx（避免字符串键冲突）
- `WithContext(ctx context.Context) Logger`: 如果存在 traceID 则提取，返回包含该字段的日志器（`zap.String("trace_id", id)`）。如果不存在则回退到默认值
- `C(ctx context.Context) Logger`: `WithContext` 的简写别名

**实现**:
- 私有键: `var traceIDKey struct{}` 确保类型安全
- 提取使用直接类型断言（无反射）保证性能
- 创建新 ctx（不可变，并发安全）

**工作流**:
1. 中间件/拦截器: `ctx = WithTraceID(originalCtx, traceID)`
2. 业务代码: `logger = WithContext(ctx)` (自动添加 trace_id)

**设计理念**:
- **封装**: 隐藏键细节；用户不手动管理上下文值
- **类型安全**: 编译时检查防止错误类型等问题
- **API 对称性**: 注入 (WithTraceID) + 提取 (WithContext) 形成完整直观的对
- **隔离性**: 每请求 ctx 确保无跨请求泄漏
- **性能**: 除了 zap 字段添加外零运行时开销

这遵循了 Go 上下文最佳实践（如无全局变量、不可变传播）。

### 4. 配置系统

**目的**: 集中所有配置逻辑以提高可维护性。

**核心结构**:
```go
type Config struct {
    Level       string           `json:"level"`      // 日志级别
    Format      string           `json:"format"`     // "json" 或 "console"
    Output      string           `json:"output"`     // 目标 (stdout/文件)
    AddSource   bool             `json:"add_source"` // 包含文件:行
    EnableColor bool             `json:"enable_color"`
    RootPath    string           `json:"root_path"`  // 相对路径用
    Rotation    *RotationConfig  `json:"rotation"`   // 文件轮转
}

type RotationConfig struct {
    MaxSize    int  `json:"maxSize"`    // MB
    MaxBackups int  `json:"maxBackups"` // 备份文件数
    MaxAge     int  `json:"maxAge"`     // 天数
    Compress   bool `json:"compress"`   // 压缩轮转文件
}
```

**实现**:
- 通过函数式模式解析选项: `type Option func(*options)`
- 加载时验证（如无效级别 → 错误）
- 如果 Output 是文件，轮转使用 lumberjack 进行文件管理
- 具有可配置大小限制、备份计数和压缩的自动日志轮转

**轮转特性**:
- **MaxSize**: 轮转前最大文件大小（MB，默认: 100MB）
- **MaxBackups**: 保留的旧日志文件最大数量（默认: 3）
- **MaxAge**: 旧日志文件最大保留天数（默认: 7 天）
- **Compress**: 是否使用 gzip 压缩轮转文件（默认: false）
- **Local Time**: 使用本地时间进行文件轮转时间戳
- **文件命名**: 自动管理带时间戳的轮转文件名

**设计理念**: 在 `config.go` 中集中配置分离关注点，简化测试，支持热重载等未来扩展。

## 🔄 Log Rotation Architecture

### Overview
clog provides built-in log rotation functionality using the lumberjack library, enabling automatic log file management without external dependencies. This feature is essential for production environments where log files need to be managed automatically to prevent disk space exhaustion.

### Rotation Implementation

#### Core Components
- **RotationConfig**: Configuration structure for rotation parameters
- **lumberjack.Logger**: Underlying rotation engine
- **buildLoggerWithRotation()**: Internal function that creates rotating log writers
- **Automatic Rotation**: Seamlessly integrated with zap's WriteSyncer interface

#### Configuration Parameters
```go
type RotationConfig struct {
    MaxSize    int  `json:"maxSize"`    // Maximum file size in MB before rotation
    MaxBackups int  `json:"maxBackups"` // Maximum number of backup files to retain
    MaxAge     int  `json:"maxAge"`     // Maximum age of backup files in days
    Compress   bool `json:"compress"`   // Whether to compress rotated files
}
```

#### Implementation Details
```go
func buildLoggerWithRotation(config *Config) (zapcore.WriteSyncer, error) {
    rotatingWriter := &lumberjack.Logger{
        Filename:   config.Output,
        MaxSize:    config.Rotation.MaxSize,
        MaxBackups: config.Rotation.MaxBackups,
        MaxAge:     config.Rotation.MaxAge,
        Compress:   config.Rotation.Compress,
        LocalTime:  true,
    }
    return zapcore.AddSync(rotatingWriter), nil
}
```

### Rotation Behavior

#### File Naming Convention
- **Current Log**: Uses the filename specified in `config.Output`
- **Rotated Files**: Appends timestamp to filename: `filename.YYYY-MM-DD-HH-MM-SS`
- **Compressed Files**: Adds `.gz` extension: `filename.YYYY-MM-DD-HH-MM-SS.gz`

#### Rotation Triggers
1. **Size-based**: When current log file exceeds `MaxSize` megabytes
2. **Time-based**: When file age exceeds `MaxAge` days
3. **Manual**: External rotation tools can be used in conjunction

#### Cleanup Process
- **RetentionPolicy**: Keeps maximum of `MaxBackups` files
- **AgeCleanup**: Removes files older than `MaxAge` days
- **Compression**: Optionally compresses rotated files to save space

### Integration with zap Logger

The rotation functionality is transparently integrated with zap's logger architecture:

```
Application Logs
    ↓
zap.Logger (clog wrapper)
    ↓
zapcore.Core (with rotation encoder)
    ↓
lumberjack.Logger (rotation manager)
    ↓
File System
```

### Performance Considerations

1. **Minimal Overhead**: lumberjack uses efficient file operations
2. **Lock-free Rotation**: Atomic file operations prevent log loss
3. **Buffered Writing**: Uses system buffers for optimal performance
4. **Compression**: Background compression to avoid blocking log writes

### Best Practices

1. **Production Configuration**:
   ```go
   config.Rotation = &clog.RotationConfig{
       MaxSize:    100,  // 100MB
       MaxBackups: 5,    // 5 backup files
       MaxAge:     30,   // 30 days retention
       Compress:   true, // Enable compression
   }
   ```

2. **Development Configuration**:
   ```go
   config.Rotation = &clog.RotationConfig{
       MaxSize:    10,   // 10MB
       MaxBackups: 2,    // 2 backup files
       MaxAge:     7,    // 7 days retention
       Compress:   false, // No compression for debugging
   }
   ```

3. **Monitoring**: Monitor disk usage and log rotation frequency
4. **Backup Strategy**: Consider external backup for compliance requirements

## 🔄 日志轮转架构

### 概述
clog 使用 lumberjack 库提供内置日志轮转功能，无需外部依赖即可实现自动日志文件管理。此功能对于需要自动管理日志文件以防止磁盘空间耗尽的生产环境至关重要。

### 轮转实现

#### 核心组件
- **RotationConfig**: 轮转参数的配置结构
- **lumberjack.Logger**: 底层轮转引擎
- **buildLoggerWithRotation()**: 创建轮转日志写入器的内部函数
- **自动轮转**: 与 zap 的 WriteSyncer 接口无缝集成

#### 配置参数
```go
type RotationConfig struct {
    MaxSize    int  `json:"maxSize"`    // 轮转前最大文件大小 (MB)
    MaxBackups int  `json:"maxBackups"` // 保留的最大备份文件数
    MaxAge     int  `json:"maxAge"`     // 备份文件最大天数
    Compress   bool `json:"compress"`   // 是否压缩轮转文件
}
```

#### 实现细节
```go
func buildLoggerWithRotation(config *Config) (zapcore.WriteSyncer, error) {
    rotatingWriter := &lumberjack.Logger{
        Filename:   config.Output,
        MaxSize:    config.Rotation.MaxSize,
        MaxBackups: config.Rotation.MaxBackups,
        MaxAge:     config.Rotation.MaxAge,
        Compress:   config.Rotation.Compress,
        LocalTime:  true,
    }
    return zapcore.AddSync(rotatingWriter), nil
}
```

### 轮转行为

#### 文件命名约定
- **当前日志**: 使用 `config.Output` 中指定的文件名
- **轮转文件**: 附加时间戳到文件名: `filename.YYYY-MM-DD-HH-MM-SS`
- **压缩文件**: 添加 `.gz` 扩展名: `filename.YYYY-MM-DD-HH-MM-SS.gz`

#### 轮转触发条件
1. **基于大小**: 当前日志文件超过 `MaxSize` 兆字节时
2. **基于时间**: 当文件年龄超过 `MaxAge` 天时
3. **手动**: 可结合使用外部轮转工具

#### 清理过程
- **保留策略**: 保留最多 `MaxBackups` 个文件
- **年龄清理**: 删除超过 `MaxAge` 天的文件
- **压缩**: 可选压缩轮转文件以节省空间

### 与 zap 日志器的集成

轮转功能与 zap 的日志器架构透明集成：

```
应用程序日志
    ↓
zap.Logger (clog 包装器)
    ↓
zapcore.Core (带轮转编码器)
    ↓
lumberjack.Logger (轮转管理器)
    ↓
文件系统
```

### 性能考虑

1. **最小开销**: lumberjack 使用高效的文件操作
2. **无锁轮转**: 原子文件操作防止日志丢失
3. **缓冲写入**: 使用系统缓冲区实现最佳性能
4. **压缩**: 后台压缩避免阻塞日志写入

### 最佳实践

1. **生产配置**:
   ```go
   config.Rotation = &clog.RotationConfig{
       MaxSize:    100,  // 100MB
       MaxBackups: 5,    // 5 个备份文件
       MaxAge:     30,   // 30 天保留
       Compress:   true, // 启用压缩
   }
   ```

2. **开发配置**:
   ```go
   config.Rotation = &clog.RotationConfig{
       MaxSize:    10,   // 10MB
       MaxBackups: 2,    // 2 个备份文件
       MaxAge:     7,    // 7 天保留
       Compress:   false, // 调试时不压缩
   }
   ```

3. **监控**: 监控磁盘使用情况和日志轮转频率
4. **备份策略**: 考虑合规要求的外部备份

## 🔑 关键技术决策

### 1. 层次化命名空间 vs 模块系统
- **为什么？** 减少 API 重复，统一概念，支持微服务的灵活深度。传统两层限制复杂应用（如 GoChat）的可扩展性。

### 2. 类型安全的上下文键
- **为什么？** 防止类型不匹配或键冲突的运行时 panic。封装内部细节，符合 Go 对安全性和简洁性的强调。

### 3. 集中配置
- **为什么？** 避免分散的配置代码，提高可维护性，支持统一验证/解析。

### 4. Zap 作为基础
- **为什么？** 经过验证的零分配性能，丰富的生态系统（JSON/控制台编码器），结构化字段。最小包装保持速度。

## 🎨 应用的设计模式

1. **Provider 模式**: 用于初始化（New/Init），确保可测试性和依赖注入
2. **函数式选项**: 可扩展配置，无破坏性更改（如稍后添加 WithEncoder）
3. **单例模式**: 具有原子替换的全局日志器，用于线程安全和热更新
4. **装饰器模式**: Namespace() 包装日志器，添加字段而不改变核心行为
5. **适配器模式**: 包装 zap.Logger 以强制执行 clog 的接口并添加 traceID 等功能

## 🚀 性能策略

1. **零分配日志记录**: 直接使用 zap.Field；无中间结构
2. **延迟初始化**: 单例在首次使用时加载
3. **高效字段**: TraceID/命名空间每日志器添加一次，非每日志添加
4. **无反射**: 上下文提取使用类型断言
5. **基准测试**: 热路径目标 <1% 开销（如 Info 调用）

## 📊 向后兼容性和迁移

### 破坏性更改
- `Module()` → `Namespace()`: 统一 API
- Init/New 签名: 添加 ctx/opts 以符合 Provider 规范
- TraceID: `context.WithValue(..., "traceID", ...)` → `WithTraceID()` 提高安全性
- 移除如 `SetTraceIDHook()` 等钩子: 简化为基于上下文

### 迁移指南
1. **命名空间**: 将 `Module("user")` 替换为 `Namespace("user")`
2. **Init**: 添加 `context.Background()` 和 `&config`；使用 `WithNamespace("service")`
3. **TraceID**: 在中间件中使用 `WithTraceID(ctx, id)`；在处理器中使用 `WithContext(ctx)`
4. **全局变量**: 如果未使用破坏性 API，现有代码可工作；更新为新功能

未更新代码无运行时中断；支持渐进迁移。

## 🔮 未来扩展

1. **配置中心**: etcd 集成实现动态级别/格式
2. **高级选项**: 自定义编码器、输出、钩子
3. **监控**: 指标（日志速率、错误）、自动告警
4. **追踪**: OpenTelemetry spans，自动传播
5. **采样**: 高量日志的速率限制

此设计在简洁性、功能和性能之间取得平衡，使 clog 成为 infra-kit 可观察分布式系统的理想选择。

API 参考和示例见 [README.md](README.md)。