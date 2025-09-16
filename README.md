# gochat-kit: 现代化 Go 微服务基础设施组件库

`gochat-kit` 是一个为构建现代化 Go 微服务而设计的高性能、可扩展基础设施组件库。它提供了一套经过生产验证的、遵循统一设计规范的基础设施组件，让开发者可以专注于业务逻辑创新，而非重复搭建底层设施。

## 🎯 项目愿景 (Vision)

在现代分布式系统开发中，我们经常面临以下挑战：
- 基础设施组件重复建设，缺乏统一标准
- 组件间集成复杂，维护成本高昂
- 缺乏生产级的可靠性保障
- 可观测性和服务治理能力不足

`gochat-kit` 旨在解决这些痛点，提供一个：
- **生产级可靠**：每个组件都经过生产环境验证，具备高可用性和高性能
- **架构一致**：所有组件遵循统一的设计规范和接口契约
- **易于集成**：标准化接口和依赖注入，降低集成复杂度
- **可观测性原生**：深度集成日志、指标和链路追踪能力
- **渐进式采用**：组件可独立使用，也支持完整基础设施栈集成

## 🏗️ 核心设计原则 (Core Design Principles)

### 1. Provider 模式与统一接口契约

所有有状态组件都通过 **Provider 模式** 暴露能力，确保接口的一致性和可测试性：

```go
// 标准构造函数签名
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)

// 标准配置获取
func GetDefaultConfig(env string) *Config
```

**设计理念**：
- **配置职责分离**：`config` 负责核心静态配置，`opts` 负责外部依赖注入
- **环境感知**：`GetDefaultConfig` 为 `development` 和 `production` 环境提供优化默认值
- **依赖注入**：通过函数式选项模式注入依赖，如 `clog.Logger`、`coord.Provider`

### 2. 组件自治的动态配置

- **声明式配置**：所有可变配置通过配置中心管理，组件不提供命令式修改 API
- **自动热更新**：组件内部监听配置变更，自动更新内部状态
- **故障隔离**：配置变更失败不影响组件正常运行

### 3. 上下文感知与链路追踪

- **Context 优先**：所有 I/O 操作必须接受 `context.Context` 作为首参数
- **TraceID 自动传播**：通过上下文实现完整的请求链路追踪
- **层次化命名空间**：支持链式调用构建清晰的日志标识体系

### 4. 双模式运行支持

关键组件支持 **分布式** 和 **单机** 两种运行模式：
- **分布式模式**：适用于集群部署，依赖外部基础设施（如 Redis、etcd）
- **单机模式**：适用于独立部署，无外部依赖，性能更高

## 📊 组件依赖关系与实现路线图

### 依赖层次图

```
┌─────────────────────────────────────────────────────────────┐
│                    应用层 (Your Services)                     │
└─────────────────────────┬───────────────────────────────────┘
                         │
              ┌──────────▼───────────┐
              │    可观测性层         │
              │  • metrics           │
              └───────────────────────┘
                         │
┌─────────────────────────┼───────────────────────────────────┐
│                    服务治理层                               │
│  • ratelimit  • once  • breaker  • es                      │
└─────────────────────────┼───────────────────────────────────┘
                         │
              ┌──────────▼───────────┐
              │    核心基础设施层     │
              │  • coord  • cache   │
              │  • db     • mq      │
└─────────────┬───────────────────────┘
              │
      ┌───────▼───────┐
      │   基础设施层   │
      │ • clog • uid  │
      └───────────────┘
```

### 实现路线图

#### 🎯 阶段 0：基础组件（可独立使用）
1. **clog** - 结构化日志
   - **依赖**：无
   - **特点**：基于 uber-go/zap，支持层次化命名空间和上下文感知
   - **适用场景**：所有需要日志记录的应用

2. **uid** - 唯一 ID 生成
   - **依赖**：无（Snowflake 模式需要 coord，但 UUIDv7 模式可独立）
   - **特点**：支持 Snowflake 和 UUID v7 两种生成算法
   - **适用场景**：需要分布式唯一 ID 的场景

#### 🚀 阶段 1：核心基础设施（构建微服务基石）
3. **coord** - 分布式协调
   - **依赖**：clog
   - **特点**：基于 etcd，提供服务发现、配置中心、分布式锁
   - **适用场景**：需要分布式协调能力的微服务集群

4. **cache** - 分布式缓存
   - **依赖**：clog（可选：coord 用于配置管理）
   - **特点**：支持 Redis，提供字符串、哈希、集合、有序集合等操作
   - **适用场景**：需要高性能缓存和分布式锁的场景

5. **db** - 数据库访问
   - **依赖**：clog
   - **特点**：基于 GORM，支持分库分表和连接池优化
   - **适用场景**：需要数据库访问的应用

6. **mq** - 消息队列
   - **依赖**：clog（可选：coord 用于配置管理）
   - **特点**：基于 Kafka，提供生产者-消费者模式
   - **适用场景**：需要异步消息处理的应用

#### 🛡️ 阶段 2：服务治理（提升系统可靠性）
7. **ratelimit** - 分布式限流
   - **依赖**：clog, coord, cache
   - **特点**：支持令牌桶算法，可配置分布式/单机模式
   - **适用场景**：需要流量控制和 API 保护的场景

8. **once** - 分布式幂等
   - **依赖**：clog, cache
   - **特点**：支持操作幂等性和结果缓存
   - **适用场景**：需要保证操作唯一性的场景（如支付、订单处理）

9. **breaker** - 熔断器
   - **依赖**：clog, coord
   - **特点**：基于熔断模式，防止系统雪崩
   - **适用场景**：需要保护下游服务依赖的场景

10. **es** - 搜索引擎
    - **依赖**：clog
    - **特点**：基于 Elasticsearch，支持泛型索引
    - **适用场景**：需要全文搜索和数据分析的场景

#### 📈 阶段 3：可观测性（完善监控体系）
11. **metrics** - 监控指标
    - **依赖**：clog
    - **特点**：基于 OpenTelemetry，提供自动化指标收集
    - **适用场景**：需要监控和链路追踪的应用

## 📁 组件详细说明

### 📝 结构化日志 (clog)

**核心特性**：
- 基于 uber-go/zap 的高性能日志记录
- 层次化命名空间支持
- 自动 TraceID 传播
- 上下文感知日志记录

**使用场景**：
```go
// 初始化
clog.Init(ctx, clog.GetDefaultConfig("production"), 
    clog.WithNamespace("my-service"))

// 业务代码中使用
logger := clog.WithContext(ctx)
logger.Info("处理用户请求", 
    clog.String("user_id", userID),
    clog.String("operation", "create_profile"))
```

### 🔗 分布式协调 (coord)

**核心特性**：
- 服务注册与发现
- 配置中心与动态配置
- 分布式锁机制
- 实例 ID 自动分配

**使用场景**：
```go
// 初始化
coordProvider, _ := coord.New(ctx, coordConfig, 
    coord.WithLogger(clog.Namespace("coord")))

// 服务注册
serviceInfo := coord.ServiceInfo{
    Name:    "user-service",
    Address: "localhost:8080",
}
coordProvider.Registry().Register(ctx, serviceInfo, 30*time.Second)

// 配置监听
watcher, _ := coordProvider.Config().WatchPrefix(ctx, "/config/ratelimit/", &rules)
```

### 🗄️ 数据库访问 (db)

**核心特性**：
- 基于 GORM 的 ORM 操作
- 自动分库分表支持
- 连接池优化
- 简化事务管理

**使用场景**：
```go
// 初始化
dbProvider, _ := db.New(ctx, dbConfig,
    db.WithLogger(clog.Namespace("db")))

// 基本查询
var user User
err := dbProvider.DB(ctx).Where("id = ?", userID).First(&user).Error

// 事务操作
err := dbProvider.Transaction(ctx, func(tx *gorm.DB) error {
    // 事务逻辑
    return tx.Model(&Account{}).Update("balance", gorm.Expr("balance - ?", amount)).Error
})
```

### 🚀 消息队列 (mq)

**核心特性**：
- 基于 Kafka 的消息生产消费
- 自动偏移量管理
- TraceID 自动传播
- 支持同步和异步发送

**使用场景**：
```go
// 生产者初始化
producer, _ := mq.NewProducer(ctx, mqConfig,
    mq.WithLogger(clog.Namespace("mq-producer")))

// 发送消息
msg := &mq.Message{
    Topic: "user.events",
    Key:   []byte(userID),
    Value: eventData,
}
producer.Send(ctx, msg, func(err error) {
    // 处理发送结果
})

// 消费者
consumer, _ := mq.NewConsumer(ctx, mqConfig, "user-service-group")
consumer.Subscribe(ctx, []string{"user.events"}, func(ctx context.Context, msg *mq.Message) error {
    // 处理消息
    return nil // 成功处理，自动提交偏移量
})
```

### ⚡ 分布式限流 (ratelimit)

**核心特性**：
- 支持分布式（Redis）和单机（内存）模式
- 令牌桶算法实现
- 动态配置热更新
- 多维度限流策略

**使用场景**：
```go
// 初始化
rateLimitProvider, _ := ratelimit.New(ctx, rateLimitConfig,
    ratelimit.WithCoordProvider(coordProvider),
    ratelimit.WithCacheProvider(cacheProvider))

// 限流检查
allowed, err := rateLimitProvider.Allow(ctx, "user:123", "api_call_limit")
if !allowed {
    return fmt.Errorf("请求过于频繁，请稍后重试")
}
```

### 🔄 分布式幂等 (once)

**核心特性**：
- 支持分布式和单机模式
- 操作幂等性保证
- 结果缓存机制
- 自动状态管理

**使用场景**：
```go
// 无返回值幂等
err := onceProvider.Do(ctx, "payment:order-123", 24*time.Hour, func() error {
    return processPayment(orderData)
})

// 带结果缓存
result, err := onceProvider.Execute(ctx, "doc:create:xyz", 48*time.Hour, func() (any, error) {
    return createDocument(docData)
})
```

## 🎨 统一初始化模式

```go
func main() {
    // 阶段 0：基础组件
    clog.Init(ctx, clog.GetDefaultConfig("production"), 
        clog.WithNamespace("my-service"))
    
    // 阶段 1：核心基础设施
    coordProvider, _ := coord.New(ctx, coordConfig,
        coord.WithLogger(clog.Namespace("coord")))
    
    cacheProvider, _ := cache.New(ctx, cacheConfig,
        cache.WithLogger(clog.Namespace("cache")),
        cache.WithCoordProvider(coordProvider))
    
    // 阶段 2：服务治理
    rateLimitProvider, _ := ratelimit.New(ctx, rateLimitConfig,
        ratelimit.WithCoordProvider(coordProvider),
        ratelimit.WithCacheProvider(cacheProvider))
    
    // 启动应用服务
    service := NewMyService(coordProvider, cacheProvider, rateLimitProvider)
    service.Run()
}
```

## 🚀 快速开始

### 最小化使用示例

```go
package main

import (
    "context"
    "github.com/ceyewan/gochat-kit/clog"
    "github.com/ceyewan/gochat-kit/cache"
)

func main() {
    ctx := context.Background()
    
    // 1. 初始化日志
    clog.Init(ctx, clog.GetDefaultConfig("development"), 
        clog.WithNamespace("demo"))
    
    // 2. 初始化缓存
    cacheProvider, err := cache.New(ctx, cache.GetDefaultConfig("development"),
        cache.WithLogger(clog.Namespace("cache")))
    if err != nil {
        clog.Fatal("缓存初始化失败", clog.Err(err))
    }
    defer cacheProvider.Close()
    
    // 3. 使用组件
    err = cacheProvider.String().Set(ctx, "hello", "world", time.Hour)
    if err != nil {
        clog.Error("设置缓存失败", clog.Err(err))
    }
    
    value, err := cacheProvider.String().Get(ctx, "hello")
    if err == nil {
        clog.Info("缓存值", clog.String("value", value))
    }
}
```

## 📚 文档导航

- **[使用指南](docs/usage_guide.md)**：详细的使用示例和最佳实践
- **[设计规范](docs/README.md)**：核心设计原则和接口契约
- **[组件文档](docs/)**：每个组件的详细设计说明

## 🛠️ 开发建议

### 组件选择指南

- **简单应用**：从 `clog` + `cache` + `db` 开始
- **微服务集群**：增加 `coord` + `mq` + `ratelimit`
- **高可靠性要求**：增加 `once` + `breaker` + `metrics`
- **搜索需求**：按需增加 `es`

### 配置管理策略

1. **开发环境**：使用 `GetDefaultConfig("development")` 获取默认配置
2. **生产环境**：使用 `GetDefaultConfig("production")` 并覆盖关键配置
3. **动态配置**：通过 `coord` 配置中心管理运行时配置

### 错误处理最佳实践

- 网络异常：组件内置重试机制，业务代码关注核心逻辑
- 配置错误：启动时快速失败，避免运行时问题
- 业务异常：提供明确的错误类型，便于业务代码处理

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件