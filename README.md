# infra-kit: A Go Foundational Infrastructure Kit

`infra-kit` 是一个为现代化 Go 应用和服务设计的高性能、可扩展的基础设施组件库。它的名字寓意着“基石”，旨在为上层业务逻辑提供稳定、可靠且易于使用的底层能力支持。

## 愿景 (Vision)

在构建复杂的分布式系统时，开发者往往需要重复实现日志、数据库、缓存、消息队列等基础功能。`infra-kit` 的目标是将这些通用能力标准化、模块化，提供一套“开箱即用”的最佳实践，让开发者可以更专注于业务逻辑的创新，而非底层设施的搭建。

我们致力于：
*   **生产力优先**: 提供简洁、直观的 API，大幅提升开发效率。
*   **高性能**: 底层实现优选社区广泛验证的高性能库。
*   **可观测性**: 所有组件都将原生集成日志、指标和追踪能力（规划中）。
*   **高可扩展性**: 遵循面向接口和依赖注入的设计，轻松替换或扩展组件实现。

## 项目结构与命名约定 (Structure & Naming)

`infra-kit` 遵循 Go 社区的最佳实践，采用清晰、独立的包结构。

### 根目录结构

```plaintext
infra-kit/
├── clog/            # 日志
├── database/        # 数据库
├── cache/           # 缓存
├── mq/              # 消息队列
├── elasticsearch/   # Elasticsearch
├── coordinator/     # 分布式协调
├── metrics/         # 监控指标
├── idempotent/      # 幂等处理
├── uid/             # 唯一 ID 生成
├── ratelimit/       # 限流
├── breaker/         # 熔断器
└── internal/        # 内部共享代码，不对外暴露
```

### 命名哲学

1.  **清晰胜于简洁**: 我们选用能够准确描述其功能的标准单词（如 `database` 而非 `db`），因为 Go 的包路径本身提供了唯一的命名空间。
2.  **避免缩写**: 除非是业界公认的缩写（如 `mq`），否则使用全称（如 `coordinator` 而非 `coord`），以增强代码的可读性。

## 核心设计原则 (Core Design Principles)

`infra-kit` 的所有组件都遵循以下核心设计原则，以保证整个库的一致性、健壮性和可维护性。

### 1. 面向接口编程 (Interface-Oriented Programming)

**这是 `infra-kit` 最核心的原则。**

每个组件都对外暴露一个或多个接口，作为其功能的唯一契约。外部调用者只与接口交互，而不关心其背后的具体实现。

**示例 (`cache` 包):**
```go
// cache.go
package cache

type Cache interface {
    Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
    Get(ctx context.Context, key string) (string, error)
}

// redis.go (实现)
type redisCache struct { /* ... */ }
func (r *redisCache) Set(...) error { /* ... */ }
func (r *redisCache) Get(...) (string, error) { /* ... */ }
```
**好处**:
*   **低耦合**: 使用者代码与具体技术（如 Redis, MySQL）解耦。
*   **可测试性**: 在单元测试中，可以轻松地注入一个内存实现的 Mock 对象，而无需启动重量级的外部服务。
*   **灵活性**: 未来可以无缝地增加新的实现（如 `memcached`），而无需改动任何上层业务代码。

### 2. 函数式选项模式 (Functional Options Pattern)

所有组件的初始化都采用函数式选项模式，以提供灵活、可读且向后兼容的配置方式。

**示例 (`database` 包):**
```go
package database

// 初始化函数
func New(dsn string, opts ...Option) (DB, error) {
    // ...
}

// 配置项
type Option func(*Options)

// 具体的配置函数
func WithMaxOpenConns(n int) Option {
    return func(o *Options) {
        o.MaxOpenConns = n
    }
}

// 使用方式
db, err := database.New("user:pass@...", database.WithMaxOpenConns(100))
```

### 3. 依赖倒置原则 (Dependency Inversion Principle)

为了构建清晰的依赖关系并避免循环引用，我们遵循依赖倒置原则：**高层模块不应依赖于底层模块，两者都应依赖于抽象（接口）**。

当组件 A 需要组件 B 的功能，但从逻辑上讲 B 比 A 更“高层”或存在循环依赖风险时：
1.  在 A 包中定义一个描述其需求的接口。
2.  让 A 的构造函数依赖于这个接口。
3.  让 B 包（或其使用者）去实现这个接口。
4.  在服务的最终组装点（如 `main.go`），将 B 的实例注入到 A 中。

这个原则是解决复杂系统中依赖管理问题的关键。

## 建议开发顺序 (Suggested Development Order)

我们推荐按照组件的依赖层次进行开发，以确保每一步的产出都可以被后续工作立即使用。

1.  **Tier 0: 核心基石 (Foundation)** - 无任何内部依赖
    *   `log` (日志): 所有组件都需要日志记录。
    *   `uid` (ID 生成器): 分布式系统基础。

2.  **Tier 1: 核心能力 (Core Capabilities)** - 依赖 Tier 0
    *   `database` (数据库)
    *   `cache` (缓存)
    *   `mq` (消息队列)
    *   `elasticsearch` (搜索)

3.  **Tier 2: 服务治理 (Service Governance)** - 依赖 Tier 0/1
    *   `metrics` (监控指标)
    *   `ratelimit` (限流器)
    *   `breaker` (熔断器)
    *   `idempotent` (幂等处理器)

4.  **Tier 3: 分布式协调 (Distributed Coordination)** - 按需开发
    *   `coordinator` (协调器)

## 如何使用 (How to Use)

所有组件都应通过其 `NewXxx` 构造函数进行实例化，并通过依赖注入的方式在你的服务中使用。

```go
package main

import (
    "infra-kit/log"
    "infra-kit/cache"
)

func main() {
    // 1. 初始化基础组件
    logger := log.New(/* ... */)

    // 使用函数式选项配置 Redis 缓存
    cache, err := cache.New(
        cache.WithDSN("redis://localhost:6379/0"),
        cache.WithPassword("your-password"),
    )
    if err != nil {
        logger.Fatal("failed to init cache", "error", err)
    }

    // 2. 将组件实例注入到业务服务中
    // myService := service.NewMyService(logger, cache)

    // 3. 启动应用
    // ...
}
```
