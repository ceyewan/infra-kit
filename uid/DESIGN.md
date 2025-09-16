# uid 设计文档

## 🎯 设计目标

`uid` 是 infra-kit 项目的唯一标识符生成组件，提供 Snowflake 和 UUID v7 两种生成算法，满足不同业务场景的需求。

### 核心设计原则

1. **多算法支持**: 同时支持 Snowflake 和 UUID v7 两种生成算法
2. **场景适配**: 为不同的使用场景提供最合适的 ID 类型
3. **高性能**: Snowflake ID 生成速度极快，适合高并发场景
4. **实例安全**: 通过实例 ID 管理保证多实例环境下的唯一性
5. **易于使用**: 统一的 API 接口，简化使用复杂度
6. **无外部依赖**: 当前实现无需协调服务，降低部署复杂度

### 应用场景

- **Snowflake ID**: 数据库主键、消息 ID、订单号等需要排序和高性能的场景
- **UUID v7**: 请求 ID、会话 ID、外部资源 ID 等需要全局唯一性和可读性的场景

## 🏗️ 架构概览

### 高层架构

```
公共 API 层
├── New (Provider 模式)
├── GetUUIDV7() (UUID v7 生成)
├── GenerateSnowflake() (Snowflake 生成)
├── IsValidUUID() (UUID 验证)
├── ParseSnowflake() (Snowflake 解析)
└── Close() (资源释放)

配置层
├── Config 结构 (ServiceName, MaxInstanceID, InstanceID)
├── GetDefaultConfig() (环境相关默认值)
└── Validate() (配置验证)

核心算法层
├── Snowflake 算法 (时间戳 + 实例 ID + 序列号)
├── UUID v7 算法 (基于 Google UUID 库)
└── 实例 ID 管理 (配置/环境变量/随机分配)

内部实现层
├── snowflakeGenerator (Snowflake 生成器)
└── uuidGenerator (UUID 生成器)
```

### 核心组件

#### 1. Provider 接口设计

```go
type Provider interface {
    GetUUIDV7() string                             // 生成 UUID v7
    GenerateSnowflake() (int64, error)            // 生成 Snowflake ID
    IsValidUUID(s string) bool                     // 验证 UUID 格式
    ParseSnowflake(id int64) (timestamp, instanceID, sequence int64) // 解析 Snowflake ID
    Close() error                                  // 释放资源
}
```

#### 2. Snowflake 算法实现

**位分配**:
- 时间戳: 42 位 (69 年可用，从 2021-01-01 开始)
- 实例 ID: 10 位 (最多 1024 个实例)
- 序列号: 12 位 (每毫秒 4096 个 ID)

**特性**:
- 时钟回拨检测和错误处理
- 序列号溢出保护
- 高并发安全 (互斥锁保护)
- 线程安全的 ID 生成

#### 3. UUID v7 算法实现

**实现**:
- 基于 Google UUID 库，确保标准兼容性
- RFC 4122 规范的 UUID v7 格式
- 时间有序的全局唯一标识符

**特性**:
- 时间有序，便于索引和排序
- 全局唯一性保证
- 高性能生成 (无状态设计)
- 标准格式验证

## 🔧 核心实现

### 1. Snowflake 生成器

```go
type snowflakeGenerator struct {
    mu         sync.Mutex
    instanceID int64
    sequence   int64
    lastTime   int64
    epoch      int64
}

const (
    SnowflakeEpoch = 1609459200000 // 2021-01-01 00:00:00 UTC
    InstanceIDBits = 10
    SequenceBits   = 12
    MaxInstanceID = (1 << InstanceIDBits) - 1 // 1023
    MaxSequence   = (1 << SequenceBits) - 1   // 4095
)
```

**关键特性**:
- **线程安全**: 使用互斥锁保护共享状态
- **时钟回拨检测**: 防止时间回溯导致 ID 重复
- **序列号管理**: 同一毫秒内递增序列号，溢出时等待下一毫秒
- **错误处理**: 明确的错误类型和错误信息

### 2. UUID v7 生成器

```go
// 使用 Google UUID 库生成 UUID v7
func GenerateUUIDV7() string {
    u, err := uuid.NewV7()
    if err != nil {
        // 备选方案
        return uuid.New().String()
    }
    return u.String()
}
```

**关键特性**:
- **标准兼容**: 严格遵循 RFC 4122 规范
- **版本验证**: 验证 UUID 版本号为 7
- **变体验证**: 验证变体为 RFC 4122
- **错误处理**: 生成失败时的备选方案

### 3. 实例 ID 管理

```go
type uidProvider struct {
    config     *Config
    logger     clog.Logger
    snowflake  *internal.SnowflakeGenerator
    instanceID int64
    closeOnce  sync.Once
}
```

**实例 ID 分配策略**:
- **配置指定**: 通过 Config.InstanceID 直接指定
- **环境变量**: 通过 INSTANCE_ID 环境变量设置
- **随机分配**: 当 InstanceID=0 时随机分配 (0-MaxInstanceID)

**TODO: 未来扩展**:
- 添加 coord.Provider 集成支持分布式实例 ID 管理
- 实现实例 ID 租约机制
- 添加实例 ID 动态分配和释放

### 4. 配置系统

```go
type Config struct {
    ServiceName   string `json:"serviceName"`   // 服务名称
    MaxInstanceID int    `json:"maxInstanceID"` // 最大实例 ID (1-1023)
    InstanceID    int    `json:"instanceId"`    // 实例 ID (0=自动分配)
}
```

**配置验证**:
- 服务名称不能为空
- 实例 ID 范围验证 (0-MaxInstanceID)
- 最大实例 ID 范围验证 (1-1023)
- 环境变量支持

## 🔄 错误处理策略

### 1. 时钟回拨处理

```go
func (g *snowflakeGenerator) Generate() (int64, error) {
    currentTime := time.Now().UnixMilli() - g.epoch
    
    if currentTime < g.lastTime {
        return 0, fmt.Errorf("时钟回拨检测：上次时间 %d，当前时间 %d", g.lastTime, currentTime)
    }
    // ...
}
```

**处理策略**:
- 检测到时钟回拨时立即返回错误
- 避免生成重复的 ID
- 提供明确的错误信息便于排查

### 2. 配置错误处理

```go
func (c *Config) Validate() error {
    if c.ServiceName == "" {
        return fmt.Errorf("服务名称不能为空")
    }
    // ... 其他验证
}
```

**处理策略**:
- 启动时快速失败
- 提供清晰的错误信息
- 支持环境变量配置

### 3. 资源清理

```go
func (p *uidProvider) Close() error {
    p.closeOnce.Do(func() {
        if p.logger != nil {
            p.logger.Info("uid 组件已关闭")
        }
    })
    return nil
}
```

**处理策略**:
- 使用 `sync.Once` 确保只执行一次
- 记录关闭日志
- 支持优雅关闭

## 🎨 性能优化

### 1. Snowflake 性能特性

**生成性能**:
- 单次生成: 微秒级响应
- 支持高并发: 互斥锁保护
- 内存效率: 使用 int64 类型，避免内存分配

**优化策略**:
- 减少锁持有时间
- 使用高效的位运算
- 避免不必要的内存分配

### 2. UUID v7 性能特性

**生成性能**:
- 无状态设计，天然支持高并发
- 基于 Google UUID 库优化
- 无锁并发，支持极高吞吐量

**优化策略**:
- 使用成熟的第三方库
- 避免重复实现
- 保证标准兼容性

## 📊 并发安全性

### 1. Snowflake 生成器

**并发控制**:
```go
func (g *snowflakeGenerator) Generate() (int64, error) {
    g.mu.Lock()
    defer g.mu.Unlock()
    // ...
}
```

**安全保证**:
- 互斥锁保护所有共享状态
- 原子操作更新时间戳和序列号
- 避免竞态条件

### 2. UUID v7 生成器

**并发安全**:
- 每次生成都是独立的操作
- 不需要锁保护
- 天然支持高并发

## 🔄 生命周期管理

### 1. 初始化流程

```
配置验证 → 选项解析 → 实例 ID 分配 → 生成器初始化
```

### 2. 运行时管理

```
ID 生成请求 → 算法处理 → 结果返回
```

### 3. 关闭流程

```
Close() 调用 → 资源清理 → 日志记录
```

## 🔧 配置最佳实践

### 1. 服务名称设置

```go
config := &uid.Config{
    ServiceName:   "order-service",      // 清晰的服务标识
    MaxInstanceID: 100,                  // 根据集群规模设置
    InstanceID:    5,                    // 指定实例 ID
}
```

### 2. 环境相关配置

```go
// 开发环境
devConfig := uid.GetDefaultConfig("development")
devConfig.ServiceName = "dev-order-service"

// 生产环境
prodConfig := uid.GetDefaultConfig("production")
prodConfig.ServiceName = "prod-order-service"
```

### 3. 实例 ID 规划

```go
// 单实例部署
config := &uid.Config{
    ServiceName:   "single-service",
    MaxInstanceID: 1,
    InstanceID:    1,
}

// 小型集群 (3-5 实例)
config := &uid.Config{
    ServiceName:   "small-cluster-service",
    MaxInstanceID: 10,
    InstanceID:    getInstanceID(), // 1-10
}

// 中型集群 (10-100 实例)
config := &uid.Config{
    ServiceName:   "medium-cluster-service",
    MaxInstanceID: 100,
    InstanceID:    getInstanceID(), // 1-100
}

// 大型集群 (100-1024 实例)
config := &uid.Config{
    ServiceName:   "large-cluster-service",
    MaxInstanceID: 1023,
    InstanceID:    getInstanceID(), // 1-1023
}
```

## 📈 监控和可观测性

### 1. 关键指标

- **ID 生成速率**: 每秒生成的 ID 数量
- **错误率**: 生成失败的比率
- **延迟分布**: ID 生成耗时分布
- **实例 ID 使用率**: 已分配实例 ID 的比例

### 2. 日志记录

```go
clog.Info("uid 组件初始化成功",
    clog.String("service_name", config.ServiceName),
    clog.Int64("instance_id", instanceID),
    clog.Int("max_instance_id", config.MaxInstanceID),
)
```

### 3. 健康检查

- 实例 ID 配置状态
- 组件初始化状态
- 配置验证状态

## 🔮 未来扩展

### 1. 分布式支持

**TODO: coord 组件集成**
- 添加 `WithCoordProvider()` 选项
- 实现分布式实例 ID 分配
- 支持实例 ID 租约管理
- 添加优雅的资源释放机制

```go
// 未来扩展设计
type uidProvider struct {
    config     *Config
    logger     clog.Logger
    coord      coord.Provider  // 协调服务依赖
    snowflake  *internal.SnowflakeGenerator
    instanceID int64
    leaseID    string          // 租约 ID
    closeChan  chan struct{}   // 关闭信号
    closeOnce  sync.Once
}
```

### 2. 算法扩展

- **UUID v8**: 支持自定义哈希算法
- **雪花 ID 变体**: 支持不同的位分配方案
- **分段 ID**: 支持业务相关的分段生成

### 3. 功能扩展

- **ID 模板**: 支持业务定制的 ID 格式
- **批量导入**: 支持外部 ID 批量导入
- **ID 查询**: 支持基于时间戳的 ID 查询

### 4. 性能优化

- **批量生成**: 解决并发安全问题，支持批量 ID 生成
- **本地缓存**: 缓存生成的 ID 提高性能
- **异步生成**: 支持异步 ID 生成队列

## 🎯 设计总结

`uid` 组件通过提供 Snowflake 和 UUID v7 两种生成算法，满足了不同业务场景的需求。当前实现具有以下特点：

1. **简洁高效**: 无需外部依赖，部署简单
2. **高性能**: 支持高并发 ID 生成
3. **灵活配置**: 支持多种实例 ID 分配策略
4. **易于使用**: 统一的 API 接口，简化使用复杂度
5. **可扩展**: 为未来的分布式支持预留了扩展点
6. **生产就绪**: 包含完整的错误处理和资源管理

**当前限制**:
- 不支持分布式实例 ID 管理 (待 coord 组件实现)
- 批量生成功能存在并发安全问题，暂时不提供
- 缺少动态配置更新支持

这个设计使 `uid` 成为 infra-kit 项目中重要的基础设施组件，能够满足大多数场景的唯一 ID 生成需求。