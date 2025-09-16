# CLAUDE.md

此文件为 Claude Code (claude.ai/code) 在此代码仓库中工作时提供指导。确保思考和输出全程使用中文，包括日志信息，遵循编程版八荣八耻原则。

## 项目概览

`infra-kit` 是一个现代化的 Go 微服务基础设施组件库，专为构建高性能、可扩展的分布式系统而设计。它提供经过生产验证的基础设施组件，遵循统一的设计模式。

## 核心架构原则

### 编程核心哲学 - 编程版八荣八耻
1. **以暗猜接口为耻，以认真查阅为荣** - 在使用任何接口前，必须仔细阅读文档和源码，理解其行为和约束
2. **以模糊执行为耻，以寻求确认为荣** - 遇到不确定的操作时，主动验证和确认，避免模糊执行导致的问题
3. **以盲想业务为耻，以人类确认为荣** - 对业务逻辑的理解必须与人类确认，避免基于假设的开发
4. **以创造接口为耻，以复用现有为荣** - 优先使用现有接口和模式，避免重复造轮子和创造不必要的抽象
5. **以跳过验证为耻，以主动测试为荣** - 所有修改都必须经过充分测试，包括单元测试、集成测试和边界测试
6. **以破坏架构为耻，以遵循规范为荣** - 严格遵循项目架构规范，维护代码结构的一致性和可维护性
7. **以假装理解为耻，以诚实无知为荣** - 承认知识盲点，主动学习和请教，避免基于错误理解的实现
8. **以盲目修改为耻，以谨慎重构为荣** - 修改代码前深入理解现有逻辑，重构时确保功能完整性

### 设计原则

本项目的架构设计遵循业界成熟的设计模式，结合 `infra-kit` 的实际情况，强调高内聚低耦合的架构原则，通过依赖倒置实现组件间的松耦合。

#### SOLID 原则

**S - 单一职责原则 (Single Responsibility Principle)**
- 每个组件只负责一个明确的功能领域
- `clog` 专注于结构化日志，`cache` 专注于缓存操作
- 避免组件功能过于复杂，保持职责清晰

**O - 开闭原则 (Open/Closed Principle)**
- 对扩展开放，对修改关闭
- 通过接口和组合实现功能扩展
- `Provider` 模式允许组件功能扩展而无需修改现有代码

**L - 里氏替换原则 (Liskov Substitution Principle)**
- 所有组件实现都遵循统一的接口契约
- `Provider` 接口确保不同实现可以互相替换
- 单机和分布式模式可以在运行时切换

**I - 接口隔离原则 (Interface Segregation Principle)**
- 客户端不应该被迫依赖它们不使用的方法
- 每个组件只暴露必要的接口方法
- 通过函数选项模式注入依赖，避免大而全的接口

**D - 依赖倒置原则 (Dependency Inversion Principle)**
- 高层模块不应该依赖低层模块，两者都应该依赖抽象
- 抽象不应该依赖细节，细节应该依赖抽象
- 通过 `Provider` 接口和依赖注入实现依赖倒置

#### KISS 原则 (Keep It Simple, Stupid)

- **API 简洁性**：所有组件提供简单易用的 API
- **配置简化**：使用 `GetDefaultConfig()` 提供环境相关默认配置
- **实现直观**：代码实现保持简单直观，避免过度设计
- **文档清晰**：提供清晰的使用文档和示例

#### DRY 原则 (Don't Repeat Yourself)

- **通用模式**：所有组件遵循统一的 `Provider` 模式
- **代码复用**：通过接口和组合实现代码复用
- **配置统一**：统一的配置管理和依赖注入模式
- **错误处理**：统一的错误处理和日志记录模式

#### 高内聚低耦合

**高内聚**：
- 每个组件内部功能高度相关
- `clog` 组件内部集中处理所有日志相关功能
- `cache` 组件内部集中处理所有缓存相关操作
- 组件内部通过 `internal` 包隐藏实现细节

**低耦合**：
- 组件间通过接口依赖，而非具体实现
- 使用函数选项模式注入外部依赖
- 通过配置中心实现动态配置，避免硬编码依赖
- 支持组件的独立测试和部署

#### 依赖倒置实现

```go
// 正确的依赖倒置示例
type Service struct {
    logger clog.Logger    // 依赖接口而非具体实现
    cache  cache.Provider // 依赖接口而非具体实现
}

// 通过构造函数注入依赖
func NewService(logger clog.Logger, cache cache.Provider) *Service {
    return &Service{
        logger: logger,
        cache:  cache,
    }
}

// 支持不同的实现，便于测试和替换
service := NewService(
    clog.New(logConfig),      // 可以替换为任何实现 clog.Logger 的组件
    cache.New(cacheConfig),   // 可以替换为任何实现 cache.Provider 的组件
)
```

### Provider 模式
所有有状态组件都遵循 Provider 模式，具有标准签名：
```go
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
func GetDefaultConfig(env string) *Config
```

### 组件层次结构
```
应用层
├── 可观测性层 (metrics)
├── 服务治理层 (ratelimit, once, breaker, es)
├── 核心基础设施层 (coord, cache, db, mq)
└── 基础设施层 (clog, uid)
```

## 开发命令

### 组件开发
```bash
# 构建特定组件
cd clog && go build ./...
# 测试特定组件
cd cache && go test -v ./...
# 运行组件代码检查
cd ratelimit && golangci-lint run
```

## 组件架构

### 基础组件 (阶段 0)
- **clog**: 结构化日志 (无依赖)
- **uid**: 唯一 ID 生成 (Snowflake/UUIDv7)

### 核心基础设施 (阶段 1)
- **coord**: 分布式协调 (基于 etcd)
- **cache**: 分布式缓存 (基于 Redis)
- **db**: 数据库访问 (基于 GORM)
- **mq**: 消息队列 (基于 Kafka)

### 服务治理 (阶段 2)
- **ratelimit**: 分布式限流 (令牌桶算法) 或者单机版（标准 rate 库）
- **once**: 分布式幂等，单机版（标准 sync.Map）
- **breaker**: 熔断器
- **es**: Elasticsearch 集成

### 可观测性 (阶段 3)
- **metrics**: 监控和链路追踪 (基于 OpenTelemetry)

## 代码模式

### 标准初始化
```go
// 始终按此顺序初始化
clog.Init(ctx, clog.GetDefaultConfig("production"), opts...)

coordProvider, _ := coord.New(ctx, coordConfig,
    coord.WithLogger(clog.Namespace("coord")))

cacheProvider, _ := cache.New(ctx, cacheConfig,
    cache.WithLogger(clog.Namespace("cache")),
    cache.WithCoordProvider(coordProvider))
```

### 配置管理
- 使用 `GetDefaultConfig("development")` 或 `GetDefaultConfig("production")`
- 核心静态配置通过 `config` 参数
- 外部依赖通过 `opts...Option`
- 通过 coord 配置中心进行动态配置

### 错误处理
- 网络错误：组件包含内置重试机制
- 配置错误：启动时快速失败
- 业务异常：提供明确的错误类型

### Context 使用
- 所有 I/O 操作必须接受 `context.Context` 作为第一个参数
- 通过上下文自动传播 TraceID
- 使用 `clog.WithContext(ctx)` 进行请求范围的日志记录

## 组件开发指南

### 新组件结构
```
component-name/
├── component.go         # 主要实现
├── component_test.go    # 测试
├── config.go           # 配置结构
├── options.go          # 函数式选项
└── internal/           # 内部实现
```

### 模块管理
- 每个组件都是独立的 Go 模块
- 使用 `go.work` 进行本地开发
- 遵循语义化版本控制
- 保持向后兼容性

### 测试要求
- 最低 80% 的测试覆盖率
- 包含单元测试和集成测试
- 为性能关键组件提供基准测试

## 最佳实践

### 组件选择
- **简单应用**: clog + cache + db
- **微服务**: 增加 coord + mq + ratelimit
- **高可靠性**: 增加 once + breaker + metrics
- **搜索需求**: 增加 es

### 依赖注入
- 使用函数式选项模式进行依赖注入
- 注入 clog.Logger 进行结构化日志记录
- 注入 coord.Provider 进行配置管理

### 性能考虑
- 适当配置连接池
- 使用多级缓存策略
- 尽可能使用批量操作
- 使用异步消息处理

## 配置

### 环境特定默认值
- 开发环境：控制台格式，调试级别，启用颜色
- 生产环境：JSON 格式，信息级别，禁用颜色

### 动态配置
组件通过 coord 提供者监听配置变更：
```go
watcher, _ := coordProvider.Config().WatchPrefix(ctx, "/config/component/", &config)
```

## 重要说明

- 本项目遵循中文文档标准
- 所有组件支持分布式和单机两种模式
- 组件设计支持渐进式采用
- 仅关注防御性安全模式
- 代码中无硬编码的密钥或凭据
