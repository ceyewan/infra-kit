# infra-kit: 现代化 Go 微服务基础设施组件库

`infra-kit` 是一个为构建现代化 Go 微服务而设计的高性能、可扩展基础设施组件库。它提供了一套经过生产验证的、遵循统一设计规范的基础设施组件。

## 🎯 核心设计原则

### Provider 模式
所有有状态组件都通过 Provider 模式暴露能力：
```go
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
func GetDefaultConfig(env string) *Config
```

### 组件自治
- **声明式配置**: 通过配置中心管理所有可变配置
- **自动热更新**: 组件内部监听配置变更并自动更新
- **故障隔离**: 配置变更失败不影响组件正常运行

### 上下文感知
- **Context 优先**: 所有 I/O 操作必须接受 `context.Context` 作为首参数
- **TraceID 自动传播**: 通过上下文实现完整的请求链路追踪
- **层次化命名空间**: 支持链式调用构建清晰的日志标识体系

## 📊 组件层次

```
应用层
├── 可观测性层 (metrics)
├── 服务治理层 (ratelimit, once, breaker, es)
├── 核心基础设施层 (coord, cache, db, mq)
└── 基础设施层 (clog, uid)
```

## 📁 核心组件

### 🎯 阶段 0：基础组件
- **clog** - 结构化日志（基于 uber-go/zap）
- **uid** - 唯一 ID 生成（Snowflake/UUIDv7）

### 🚀 阶段 1：核心基础设施
- **coord** - 分布式协调（基于 etcd）
- **cache** - 分布式缓存（基于 Redis）
- **db** - 数据库访问（基于 GORM）
- **mq** - 消息队列（基于 Kafka）

### 🛡️ 阶段 2：服务治理
- **ratelimit** - 分布式限流（令牌桶算法）
- **once** - 分布式幂等（操作唯一性保证）
- **breaker** - 熔断器（系统保护）
- **es** - 搜索引擎（基于 Elasticsearch）

### 📈 阶段 3：可观测性
- **metrics** - 监控指标（基于 OpenTelemetry）

## 🚀 快速开始

```go
package main

import (
    "context"
    "time"
    "github.com/ceyewan/infra-kit/clog"
    "github.com/ceyewan/infra-kit/cache"
)

func main() {
    ctx := context.Background()
    
    // 初始化日志
    clog.Init(ctx, clog.GetDefaultConfig("development"),
        clog.WithNamespace("demo"))
    
    // 初始化缓存
    cacheProvider, err := cache.New(ctx, cache.GetDefaultConfig("development"),
        cache.WithLogger(clog.Namespace("cache")))
    if err != nil {
        clog.Fatal("缓存初始化失败", clog.Err(err))
    }
    defer cacheProvider.Close()
    
    // 使用组件
    cacheProvider.String().Set(ctx, "hello", "world", time.Hour)
    
    value, _ := cacheProvider.String().Get(ctx, "hello")
    clog.Info("缓存值", clog.String("value", value))
}
```

## 🛠️ 组件选择指南

- **简单应用**: `clog` + `cache` + `db`
- **微服务集群**: 增加 `coord` + `mq` + `ratelimit`
- **高可靠性**: 增加 `once` + `breaker` + `metrics`
- **搜索需求**: 增加 `es`

## 📚 文档导航

- **[使用指南](docs/usage_guide.md)**: 详细的使用示例和最佳实践
- **[设计规范](docs/README.md)**: 核心设计原则和接口契约
- **[组件文档](docs/)**: 每个组件的详细设计说明

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件