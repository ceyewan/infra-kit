# Once 组件

Once 组件提供了统一的分布式幂等操作接口，确保关键操作在分布式环境中的幂等性和结果一致性。

## 1. 概述

Once 组件实现了基于 Provider 模式的统一幂等操作接口，支持单机和分布式两种运行模式：

- **单机模式**：基于内存的幂等控制，适用于单实例部署
- **分布式模式**：基于 Redis 的分布式幂等，适用于多实例部署的微服务环境

## 2. 核心接口

### 2.1 Provider 接口

```go
// Provider 定义了幂等操作的核心接口
type Provider interface {
    // Do 执行一个幂等操作，无返回值
    // 如果key对应的操作已经成功执行过，则直接返回nil
    // 否则，执行函数f。如果f返回错误，幂等标记不会被持久化，允许重试
    Do(ctx context.Context, key string, ttl time.Duration, f func() error) error
    
    // Execute 执行一个带返回值的幂等操作
    // 如果操作已执行过，它会直接返回缓存的结果
    // 否则，执行callback，缓存其结果，并返回
    Execute(ctx context.Context, key string, ttl time.Duration, callback func() (any, error)) (any, error)
    
    // Clear 主动清除指定key的幂等标记和缓存结果
    Clear(ctx context.Context, key string) error
    
    // Close 关闭Provider并释放相关资源
    Close() error
}
```

### 2.2 构造函数和配置

```go
// Config 幂等组件配置
type Config struct {
    // Mode 幂等模式：local 或 distributed
    Mode string `json:"mode"`
    
    // ServiceName 服务名称，用于日志和监控
    ServiceName string `json:"serviceName"`
    
    // KeyPrefix 为所有幂等key添加前缀，用于命名空间隔离
    KeyPrefix string `json:"keyPrefix"`
    
    // DefaultTTL 默认过期时间
    DefaultTTL time.Duration `json:"defaultTTL"`
    
    // LocalConfig 单机幂等配置
    LocalConfig LocalConfig `json:"localConfig"`
    
    // DistributedConfig 分布式幂等配置
    DistributedConfig DistributedConfig `json:"distributedConfig"`
}

// LocalConfig 单机幂等配置
type LocalConfig struct {
    // CleanupInterval 清理间隔
    CleanupInterval time.Duration `json:"cleanupInterval"`
    
    // MaxEntries 最大缓存条目数
    MaxEntries int `json:"maxEntries"`
}

// DistributedConfig 分布式幂等配置
type DistributedConfig struct {
    // RedisKeyPrefix Redis键前缀
    RedisKeyPrefix string `json:"redisKeyPrefix"`
    
    // ResultKeyPrefix 结果缓存键前缀
    ResultKeyPrefix string `json:"resultKeyPrefix"`
    
    // LockTimeout 锁超时时间
    LockTimeout time.Duration `json:"lockTimeout"`
    
    // ScriptLua Lua脚本内容
    ScriptLua string `json:"scriptLua"`
}

// GetDefaultConfig 返回默认配置
func GetDefaultConfig(env string) *Config

// Option 定义了用于定制幂等Provider的函数
type Option func(*options)

// WithLogger 注入日志组件
func WithLogger(logger clog.Logger) Option

// WithCacheProvider 注入缓存组件（分布式模式必需）
func WithCacheProvider(provider cache.Provider) Option

// WithMetricsProvider 注入监控组件
func WithMetricsProvider(provider metrics.Provider) Option

// WithCoordProvider 注入配置中心组件
func WithCoordProvider(provider coord.Provider) Option

// New 创建幂等Provider实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
```

## 3. 实现细节

### 3.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      Once Provider                           │
├─────────────────────────────────────────────────────────────┤
│                    Core Interface                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │     Do      │  │  Execute    │  │   Clear     │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                    Implementation                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │    Local    │  │ Distributed│  │   Manager   │          │
│  │   Executor  │  │   Executor  │  │   Manager   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                  Dependencies                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   clog      │  │   cache     │  │   metrics   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 核心组件

**OnceProvider**
- 实现Provider接口
- 管理幂等操作和状态
- 提供统一的幂等接口

**LocalExecutor**
- 基于内存实现单机幂等
- 高性能幂等控制
- 支持自动清理过期状态

**DistributedExecutor**
- 基于Redis实现分布式幂等
- 支持集群级别幂等
- 使用Lua脚本保证原子性

**StateManager**
- 管理幂等状态
- 处理TTL过期
- 支持状态清理和恢复

### 3.3 幂等算法

**状态机设计**:
- 初始状态：未执行
- 执行中状态：正在执行
- 成功状态：执行成功
- 失败状态：执行失败

**分布式实现**:
- 基于Redis原子操作
- 使用Lua脚本保证一致性
- 支持结果缓存和状态管理

### 3.4 错误处理和恢复

**业务逻辑失败**:
- 如果回调函数返回错误，幂等标记不会被持久化
- 允许后续重试操作
- 支持重试次数限制

**幂等器异常**:
- 记录异常日志
- 支持降级策略
- 提供监控和告警

## 4. 高级功能

### 4.1 结果缓存

```go
// 使用Execute缓存操作结果
result, err := onceProvider.Execute(ctx, "calc:complex", time.Hour, func() (any, error) {
    // 执行复杂计算
    return complexCalculation()
})
```

### 4.2 状态管理

```go
// 手动清除幂等状态
err := onceProvider.Clear(ctx, "payment:order:123")

// 批量清除状态
err := onceProvider.ClearByPrefix(ctx, "payment:order:")
```

### 4.3 监控和指标

```go
// 监控指标
metrics := map[string]string{
    "operations_total":     "总操作数",
    "cache_hits_total":    "缓存命中数",
    "cache_misses_total":  "缓存未命中数",
    "errors_total":        "错误总数",
    "executions_total":    "执行总数",
}
```

## 5. 使用示例

### 5.1 基本使用

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/gochat-kit/once"
    "github.com/gochat-kit/clog"
    "github.com/gochat-kit/cache"
)

func main() {
    ctx := context.Background()
    
    // 初始化依赖组件
    logger := clog.New(ctx, &clog.Config{})
    cacheProvider := cache.New(ctx, &cache.Config{})
    
    // 获取默认配置
    config := once.GetDefaultConfig("production")
    config.ServiceName = "payment-service"
    config.KeyPrefix = "idempotent:"
    
    // 创建幂等Provider
    opts := []once.Option{
        once.WithLogger(logger),
        once.WithCacheProvider(cacheProvider),
    }
    
    onceProvider, err := once.New(ctx, config, opts...)
    if err != nil {
        logger.Fatal("创建幂等器失败", clog.Err(err))
    }
    defer onceProvider.Close()
    
    // 使用幂等器
    orderID := "order123"
    err = onceProvider.Do(ctx, fmt.Sprintf("payment:process:%s", orderID), 24*time.Hour, func() error {
        // 执行支付处理逻辑
        return processPayment(ctx, orderID)
    })
    
    if err != nil {
        logger.Error("支付处理失败", clog.Err(err))
    } else {
        logger.Info("支付处理完成")
    }
}

func processPayment(ctx context.Context, orderID string) error {
    // 支付处理逻辑
    return nil
}
```

### 5.2 消息队列消费者

```go
package consumer

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/gochat-kit/once"
    "github.com/gochat-kit/clog"
    "github.com/gochat-kit/mq"
)

type PaymentConsumer struct {
    onceProvider once.Provider
    mqConsumer   mq.Consumer
}

func NewPaymentConsumer(onceProvider once.Provider, mqConsumer mq.Consumer) *PaymentConsumer {
    return &PaymentConsumer{
        onceProvider: onceProvider,
        mqConsumer:   mqConsumer,
    }
}

func (c *PaymentConsumer) Start(ctx context.Context) error {
    return c.mqConsumer.Consume(ctx, "payment-topic", c.handlePaymentMessage)
}

func (c *PaymentConsumer) handlePaymentMessage(ctx context.Context, msg *mq.Message) error {
    logger := clog.WithContext(ctx)
    
    // 解析消息
    var paymentMessage PaymentMessage
    if err := json.Unmarshal(msg.Value, &paymentMessage); err != nil {
        logger.Error("解析支付消息失败", clog.Err(err))
        return err
    }
    
    // 构建幂等键
    idempotencyKey := fmt.Sprintf("payment:process:%s", paymentMessage.OrderID)
    
    // 使用Execute保证幂等性和结果缓存
    result, err := c.onceProvider.Execute(ctx, idempotencyKey, 24*time.Hour, func() (any, error) {
        logger.Info("开始处理支付", clog.String("order_id", paymentMessage.OrderID))
        
        // 执行支付逻辑
        err := c.processPayment(ctx, &paymentMessage)
        if err != nil {
            return nil, err
        }
        
        // 返回处理结果
        return &PaymentResult{
            OrderID:    paymentMessage.OrderID,
            Status:     "success",
            ProcessedAt: time.Now(),
        }, nil
    })
    
    if err != nil {
        logger.Error("支付处理失败", 
            clog.Err(err), 
            clog.String("order_id", paymentMessage.OrderID))
        return err
    }
    
    // 处理结果
    paymentResult := result.(*PaymentResult)
    logger.Info("支付处理完成", 
        clog.String("order_id", paymentResult.OrderID),
        clog.String("status", paymentResult.Status))
    
    return nil
}

func (c *PaymentConsumer) processPayment(ctx context.Context, msg *PaymentMessage) error {
    // 实际的支付处理逻辑
    return nil
}
```

### 5.3 HTTP接口幂等

```go
package handler

import (
    "context"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/gochat-kit/once"
    "github.com/gochat-kit/clog"
)

type OrderHandler struct {
    onceProvider once.Provider
}

func NewOrderHandler(onceProvider once.Provider) *OrderHandler {
    return &OrderHandler{
        onceProvider: onceProvider,
    }
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
    ctx := c.Request.Context()
    logger := clog.WithContext(ctx)
    
    // 获取幂等键
    idempotencyKey := c.GetHeader("X-Idempotency-Key")
    if idempotencyKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "缺少幂等键"})
        return
    }
    
    // 构建幂等键
    key := "order:create:" + idempotencyKey
    
    // 使用Execute保证创建操作的幂等性
    result, err := h.onceProvider.Execute(ctx, key, 24*time.Hour, func() (any, error) {
        logger.Info("开始创建订单", clog.String("idempotency_key", idempotencyKey))
        
        // 解析请求参数
        var req CreateOrderRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            return nil, err
        }
        
        // 创建订单
        order, err := h.createOrder(ctx, req)
        if err != nil {
            return nil, err
        }
        
        logger.Info("订单创建成功", 
            clog.String("order_id", order.ID),
            clog.String("idempotency_key", idempotencyKey))
        
        return order, nil
    })
    
    if err != nil {
        logger.Error("创建订单失败", 
            clog.Err(err), 
            clog.String("idempotency_key", idempotencyKey))
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // 返回结果
    order := result.(*Order)
    c.JSON(http.StatusOK, gin.H{"order": order})
}

func (h *OrderHandler) createOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
    // 实际的订单创建逻辑
    return &Order{}, nil
}
```

## 6. 最佳实践

### 6.1 幂等键设计

1. **分层命名**：使用分层命名空间，如 `业务域:操作类型:业务ID`
2. **唯一性保证**：确保幂等键的唯一性和可重复性
3. **TTL设置**：根据业务特点合理设置过期时间
4. **前缀隔离**：通过前缀实现不同服务和环境的隔离

### 6.2 性能优化

1. **结果缓存**：对计算密集型操作启用结果缓存
2. **本地缓存**：对频繁访问的键进行本地缓存
3. **批量操作**：支持批量幂等检查，减少网络开销
4. **异步清理**：异步清理过期状态，减少阻塞

### 6.3 错误处理

1. **重试机制**：对临时性错误实现自动重试
2. **降级策略**：幂等器异常时的降级处理
3. **监控告警**：对异常情况进行监控和告警
4. **日志记录**：记录幂等操作的详细信息

### 6.4 分布式环境

1. **Redis集群**：使用Redis集群提高可用性
2. **脚本优化**：优化Lua脚本性能
3. **连接池**：合理配置Redis连接池
4. **超时设置**：设置合理的超时时间

## 7. 监控和运维

### 7.1 关键指标

- **操作次数**：总操作数和成功率
- **缓存命中率**：结果缓存的命中率
- **响应时间**：幂等检查的响应时间
- **错误率**：各种错误的统计

### 7.2 日志规范

- 使用clog组件记录幂等操作日志
- 记录操作结果和执行时间
- 支持链路追踪集成

### 7.3 故障排除

1. **幂等失效**：检查键设计和TTL设置
2. **性能问题**：检查Redis配置和网络状况
3. **状态不一致**：检查Redis数据一致性
4. **内存泄漏**：检查本地缓存和状态管理

## 8. 配置示例

### 8.1 基础配置

```go
// 开发环境配置
config := &once.Config{
    Mode:        "local",
    ServiceName: "payment-service-dev",
    KeyPrefix:   "idempotent:",
    DefaultTTL:  24 * time.Hour,
    LocalConfig: once.LocalConfig{
        CleanupInterval: 1 * time.Hour,
        MaxEntries:     10000,
    },
}

// 生产环境配置
config := &once.Config{
    Mode:        "distributed",
    ServiceName: "payment-service-prod",
    KeyPrefix:   "idempotent:",
    DefaultTTL:  24 * time.Hour,
    DistributedConfig: once.DistributedConfig{
        RedisKeyPrefix:   "idempotent:",
        ResultKeyPrefix:  "result:",
        LockTimeout:      30 * time.Second,
    },
}
```

### 8.2 高级配置

```go
// 启用监控和配置中心
opts := []once.Option{
    once.WithLogger(logger),
    once.WithCacheProvider(cacheProvider),
    once.WithMetricsProvider(metricsProvider),
    once.WithCoordProvider(coordProvider),
    once.WithTTL(48 * time.Hour),
    once.WithRetryCount(3),
    once.WithRetryInterval(1 * time.Second),
}
```