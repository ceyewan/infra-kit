# RateLimit 组件

RateLimit 组件提供了统一的分布式限流接口，支持基于令牌桶算法的精确流量控制，确保系统稳定性和可用性。

## 1. 概述

RateLimit 组件实现了基于 Provider 模式的统一限流接口，支持单机和分布式两种运行模式：

- **单机模式**：基于 Go 官方 `golang.org/x/time/rate` 库，适用于单实例部署
- **分布式模式**：基于 Redis 的令牌桶算法，适用于多实例部署的微服务环境

## 2. 核心接口

### 2.1 Provider 接口

```go
// Provider 定义了限流组件的核心接口
type Provider interface {
    // Allow 检查给定资源的单个请求是否被允许
    // resource: 被限流的唯一标识，如 "user:123" 或 "ip:1.2.3.4"
    // ruleName: 要应用的规则名
    // 返回值: bool-是否允许，error-错误信息
    Allow(ctx context.Context, resource, ruleName string) (bool, error)

    // Close 关闭限流器，释放资源
    Close() error
}
```

### 2.2 构造函数和配置

```go
// Config 限流组件配置
type Config struct {
    // Mode 限流模式：local 或 distributed
    Mode string `json:"mode"`

    // ServiceName 服务名称，用于日志和监控
    ServiceName string `json:"serviceName"`

    // RulesPath 限流规则在配置中心的路径
    RulesPath string `json:"rulesPath"`

    // DefaultMode 默认限流模式
    DefaultMode string `json:"defaultMode"`

    // LocalConfig 单机限流配置
    LocalConfig LocalConfig `json:"localConfig"`

    // DistributedConfig 分布式限流配置
    DistributedConfig DistributedConfig `json:"distributedConfig"`
}

// LocalConfig 单机限流配置
type LocalConfig struct {
    // DefaultRate 默认令牌生成速率（每秒）
    DefaultRate float64 `json:"defaultRate"`

    // DefaultCapacity 默认令牌桶容量
    DefaultCapacity int64 `json:"defaultCapacity"`

    // CleanupInterval 清理间隔（秒）
    CleanupInterval int64 `json:"cleanupInterval"`
}

// DistributedConfig 分布式限流配置
type DistributedConfig struct {
    // DefaultRate 默认令牌生成速率（每秒）
    DefaultRate float64 `json:"defaultRate"`

    // DefaultCapacity 默认令牌桶容量
    DefaultCapacity int64 `json:"defaultCapacity"`

    // RedisKeyPrefix Redis键前缀
    RedisKeyPrefix string `json:"redisKeyPrefix"`

    // ScriptLua Lua脚本内容
    ScriptLua string `json:"scriptLua"`
}

// Rule 限流规则定义
type Rule struct {
    // Mode 限流模式
    Mode string `json:"mode"`

    // Rate 令牌生成速率（每秒）
    Rate float64 `json:"rate"`

    // Capacity 令牌桶容量
    Capacity int64 `json:"capacity"`

    // Description 规则描述
    Description string `json:"description"`

    // KeyFormat 键格式模板
    KeyFormat string `json:"keyFormat"`
}

// GetDefaultConfig 返回默认配置
func GetDefaultConfig(env string) *Config

// Option 定义了用于定制限流Provider的函数
type Option func(*options)

// WithLogger 注入日志组件
func WithLogger(logger clog.Logger) Option

// WithCoordProvider 注入配置中心组件
func WithCoordProvider(provider coord.Provider) Option

// WithCacheProvider 注入缓存组件（分布式模式必需）
func WithCacheProvider(provider cache.Provider) Option

// WithMetricsProvider 注入监控组件
func WithMetricsProvider(provider metrics.Provider) Option

// New 创建限流Provider实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
```

## 3. 实现细节

### 3.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                    RateLimit Provider                        │
├─────────────────────────────────────────────────────────────┤
│                    Core Interface                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Allow     │  │   Close     │  │  GetRules   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                    Implementation                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Local     │  │ Distributed │  │   Manager   │          │
│  │   Limiter   │  │   Limiter   │  │   Manager   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                  Dependencies                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   clog      │  │   coord     │  │   cache     │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 核心组件

**RateLimitProvider**
- 实现Provider接口
- 管理限流规则和限流器实例
- 提供统一的限流接口

**LocalLimiter**
- 基于Go官方rate库实现
- 高性能单机限流
- 支持自动清理过期限流器

**DistributedLimiter**
- 基于Redis实现
- 支持集群级别限流
- 使用Lua脚本保证原子性

**RuleManager**
- 管理限流规则
- 监听配置变化
- 动态更新限流器

### 3.3 限流算法

**令牌桶算法**:
- 支持平滑限流和突发流量
- 精确控制请求速率
- 支持动态配置更新

**分布式实现**:
- 基于Redis原子操作
- 使用Lua脚本保证一致性
- 支持集群级限流

### 3.4 动态配置

**配置热更新**:
- 监听配置中心变化
- 自动加载新规则
- 平滑切换限流策略

**规则管理**:
- 支持CRUD操作
- 规则验证和冲突检测
- 版本控制和回滚

## 4. 高级功能

### 4.1 多维度限流

```go
// 用户维度限流
allowed, err := limiter.Allow(ctx, "user:123", "user_api_limit")

// IP维度限流
allowed, err := limiter.Allow(ctx, "ip:192.168.1.100", "ip_request_limit")

// 接口维度限流
allowed, err := limiter.Allow(ctx, "api:/user/profile", "api_rate_limit")
```

### 4.2 降级策略

```go
// 降级配置
config := &Config{
    Mode: "distributed",
    DistributedConfig: DistributedConfig{
        FallbackToLocal: true,  // 分布式异常时降级到单机
        FallbackRate:    100.0,  // 降级后的限流速率
    },
}
```

### 4.3 监控和指标

```go
// 监控指标
metrics := map[string]string{
    "requests_total":     "总请求数",
    "allowed_total":      "通过请求数",
    "limited_total":      "被限流请求数",
    "current_tokens":     "当前令牌数",
    "rule_updates":       "规则更新次数",
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

    "github.com/infra-kit/ratelimit"
    "github.com/infra-kit/clog"
    "github.com/infra-kit/coord"
    "github.com/infra-kit/cache"
)

func main() {
    ctx := context.Background()

    // 初始化依赖组件
    logger := clog.New(ctx, &clog.Config{})
    coordProvider := coord.New(ctx, &coord.Config{})
    cacheProvider := cache.New(ctx, &cache.Config{})

    // 获取默认配置
    config := ratelimit.GetDefaultConfig("production")
    config.ServiceName = "message-service"
    config.RulesPath = "/config/prod/message-service/ratelimit/"

    // 创建限流Provider
    opts := []ratelimit.Option{
        ratelimit.WithLogger(logger),
        ratelimit.WithCoordProvider(coordProvider),
        ratelimit.WithCacheProvider(cacheProvider),
    }

    limiter, err := ratelimit.New(ctx, config, opts...)
    if err != nil {
        logger.Fatal("创建限流器失败", clog.Err(err))
    }
    defer limiter.Close()

    // 使用限流器
    userID := "user123"
    for i := 0; i < 10; i++ {
        allowed, err := limiter.Allow(ctx, fmt.Sprintf("user:%s", userID), "user_send_message")
        if err != nil {
            logger.Error("限流检查失败", clog.Err(err))
            continue
        }

        if allowed {
            logger.Info("请求通过限流检查")
            // 执行业务逻辑
        } else {
            logger.Warn("请求被限流")
            // 返回限流错误
        }

        time.Sleep(100 * time.Millisecond)
    }
}
```

### 5.2 中间件集成

```go
package middleware

import (
    "context"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/infra-kit/ratelimit"
    "github.com/infra-kit/clog"
)

func RateLimitMiddleware(limiter ratelimit.Provider) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := c.Request.Context()

        // 构建限流键
        resource := buildResourceKey(c)
        ruleName := getRuleName(c)

        // 检查限流
        allowed, err := limiter.Allow(ctx, resource, ruleName)
        if err != nil {
            clog.WithContext(ctx).Error("限流检查失败", clog.Err(err))
            c.Next() // 降级：限流器异常时放行
            return
        }

        if !allowed {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": "请求过于频繁，请稍后再试",
            })
            return
        }

        c.Next()
    }
}

func buildResourceKey(c *gin.Context) string {
    // 根据业务需求构建限流键
    // 例如：user:123, ip:192.168.1.100, api:/user/profile
    return ""
}

func getRuleName(c *gin.Context) string {
    // 根据接口路径返回对应的规则名
    return ""
}
```

### 5.3 配置管理

```go
// 限流规则配置示例
{
    "user_api_limit": {
        "mode": "distributed",
        "rate": 100.0,
        "capacity": 200,
        "description": "用户API调用限制",
        "keyFormat": "user:%s"
    },
    "ip_request_limit": {
        "mode": "local",
        "rate": 1000.0,
        "capacity": 1500,
        "description": "IP请求限制",
        "keyFormat": "ip:%s"
    },
    "api_rate_limit": {
        "mode": "distributed",
        "rate": 500.0,
        "capacity": 800,
        "description": "API接口限制",
        "keyFormat": "api:%s"
    }
}
```

## 6. 最佳实践

### 6.1 限流策略设计

1. **分层限流**：结合单机和分布式限流
2. **多维度防护**：基于用户、IP、接口等多维度限流
3. **梯度限流**：设置多级限流阈值，逐步收紧

### 6.2 性能优化

1. **本地缓存**：对频繁使用的限流键进行本地缓存
2. **批量操作**：支持批量限流检查，减少网络开销
3. **异步清理**：异步清理过期限流器实例

### 6.3 监控告警

1. **实时监控**：监控限流命中率和系统负载
2. **阈值告警**：设置限流阈值告警
3. **趋势分析**：分析限流趋势，优化限流策略

### 6.4 容错处理

1. **降级策略**：分布式限流异常时降级到单机限流
2. **熔断机制**：持续异常时自动熔断
3. **恢复机制**：异常恢复后自动恢复限流功能

## 7. 监控和运维

### 7.1 关键指标

- **限流命中率**：被限流的请求比例
- **限流器数量**：活跃的限流器实例数
- **规则更新频率**：限流规则更新次数
- **响应时间**：限流检查的响应时间

### 7.2 日志规范

- 使用clog组件记录限流日志
- 记录限流决策和原因
- 支持链路追踪集成

### 7.3 配置管理

- 通过配置中心统一管理限流规则
- 支持配置版本控制和回滚
- 提供配置验证和冲突检测

## 8. 故障排除

### 8.1 常见问题

1. **限流不生效**：检查规则配置和键格式
2. **性能问题**：检查Redis连接和Lua脚本
3. **配置不同步**：检查配置中心连接和监听

### 8.2 调试方法

1. **启用调试日志**：查看限流决策过程
2. **监控指标**：分析限流器状态
3. **配置验证**：验证规则配置的正确性

### 8.3 性能调优

1. **连接池优化**：优化Redis连接池配置
2. **本地缓存**：启用本地缓存减少Redis访问
3. **批量处理**：使用批量API减少网络开销
