# infra-kit

现代化 Go 微服务基础设施组件库。

## 组件架构

```
应用层
├── 可观测性层 (metrics)
├── 服务治理层 (ratelimit, once, breaker, es)
├── 核心基础设施层 (coord, cache, db, mq)
└── 基础设施层 (clog, uid)
```

## 核心组件

### 基础组件 (阶段 0)
- **clog** - 结构化日志 (无依赖)
- **uid** - 唯一 ID 生成 (Snowflake/UUID v7)

### 核心基础设施 (阶段 1)
- **coord** - 分布式协调 (基于 etcd)
- **cache** - 分布式缓存 (基于 Redis)
- **db** - 数据库访问 (基于 GORM)
- **mq** - 消息队列 (基于 Kafka)

### 服务治理 (阶段 2)
- **ratelimit** - 分布式限流 (令牌桶算法)
- **once** - 分布式幂等 (标准 sync.Map)
- **breaker** - 熔断器
- **es** - Elasticsearch 集成

### 可观测性 (阶段 3)
- **metrics** - 监控和链路追踪 (基于 OpenTelemetry)

## 快速开始

### 使用 clog 日志组件

```go
import "github.com/ceyewan/infra-kit/clog"

// 初始化日志
config := clog.GetDefaultConfig("production")
if err := clog.Init(context.Background(), config, clog.WithNamespace("my-service")); err != nil {
    log.Fatal(err)
}

// 记录日志
clog.Info("服务启动成功")
clog.Error("操作失败", clog.Err(err))
```

### 使用 uid 唯一 ID 生成组件

```go
import "github.com/ceyewan/infra-kit/uid"

// 创建 uid Provider
config := uid.GetDefaultConfig("production")
config.ServiceName = "order-service"

provider, err := uid.New(context.Background(), config)
if err != nil {
    log.Fatal(err)
}
defer provider.Close()

// 生成 Snowflake ID (用于数据库主键)
orderID, err := provider.GenerateSnowflake()

// 生成 UUID v7 (用于请求 ID)
requestID := provider.GetUUIDV7()
```

## 开发环境

```bash
# 初始化开发环境
make init

# 运行测试
make test

# 构建所有组件
make build
```

## 设计原则

所有组件遵循以下设计原则：

- **Provider 模式**: 标准化的初始化接口
- **依赖注入**: 通过函数选项注入外部依赖
- **上下文感知**: 支持 context.Context 和链路追踪
- **环境配置**: 提供开发/生产环境默认配置
- **高内聚低耦合**: 清晰的模块边界和接口设计

## 文档

- [模块组织指南](docs/module_organization_guide.md)
- [开发规范](docs/README.md)
- [clog 使用指南](docs/clog.md)
- [uid 使用指南](docs/uid.md)

## 组件状态

| 组件 | 状态 | 文档 | 测试覆盖率 |
|------|------|------|-----------|
| clog | ✅ 已完成 | [链接](docs/clog.md) | >90% |
| uid | ✅ 已完成 | [链接](docs/uid.md) | >90% |
| coord | 🚧 开发中 | - | - |
| cache | 🚧 开发中 | - | - |
| db | 🚧 开发中 | - | - |
| mq | 🚧 开发中 | - | - |

## 许可证

MIT License