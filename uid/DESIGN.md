# uid 设计文档

## 🎯 设计目标

`uid` 是 infra-kit 项目的唯一标识符生成组件，提供 Snowflake 和 UUID v7 两种生成算法，满足不同业务场景的需求。

### 核心设计原则

1. **多算法支持**: 同时支持 Snowflake 和 UUID v7 两种生成算法
2. **场景适配**: 为不同的使用场景提供最合适的 ID 类型
3. **高性能**: Snowflake ID 生成速度极快，适合高并发场景
4. **分布式安全**: 通过协调服务保证实例 ID 的唯一性
5. **易于使用**: 统一的 API 接口，简化使用复杂度

### 应用场景

- **Snowflake ID**: 数据库主键、消息 ID、订单号等需要排序和高性能的场景
- **UUID v7**: 请求 ID、会话 ID、外部资源 ID 等需要全局唯一性和可读性的场景

## 🏗️ 架构概览

### 高层架构

```
公共 API 层
├── New/Init (Provider 模式)
├── GetUUIDV7() (UUID v7 生成)
├── GenerateSnowflake() (Snowflake 生成)
├── IsValidUUID() (UUID 验证)
├── ParseSnowflake() (Snowflake 解析)
└── Close() (资源释放)

配置层
├── Config 结构 (ServiceName, MaxInstanceID)
├── GetDefaultConfig() (环境相关默认值)
└── Validate() (配置验证)

核心算法层
├── Snowflake 算法 (时间戳 + 实例 ID + 序列号)
├── UUID v7 算法 (时间戳 + 随机数)
└── 实例 ID 管理 (协调服务集成)

内部实现层
├── snowflakeGenerator (Snowflake 生成器)
├── uuidGenerator (UUID 生成器)
└── 协议适配层 (coord.Provider 接口)
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
- 时间戳: 42 位 (69 年可用)
- 实例 ID: 10 位 (最多 1024 个实例)
- 序列号: 12 位 (每毫秒 4096 个 ID)

**特性**:
- 时钟回拨检测
- 序列号溢出保护
- 高并发安全
- 批量生成支持

#### 3. UUID v7 算法实现

**格式**:
- 前 6 字节: 时间戳 (48 位)
- 第 7 字节: 版本号 (0111)
- 第 8 字节: 变体 (10xx)
- 后 10 字节: 随机数

**特性**:
- 时间有序
- 全局唯一
- 高性能生成
- 标准格式

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
    snowflakeEpoch = 1609459200000 // 2021-01-01 00:00:00 UTC
    instanceIDBits = 10
    sequenceBits  = 12
    maxInstanceID = (1 << instanceIDBits) - 1 // 1023
    maxSequence   = (1 << sequenceBits) - 1  // 4095
)
```

**关键特性**:
- **线程安全**: 使用互斥锁保护共享状态
- **时钟回拨检测**: 防止时间回溯导致 ID 重复
- **序列号管理**: 同一毫秒内递增序列号，溢出时等待下一毫秒
- **批量生成**: 支持一次性生成多个 ID，提高性能

### 2. UUID v7 生成器

```go
func generateUUIDV7() string {
    // 获取当前时间戳（毫秒级）
    timestamp := time.Now().UnixMilli()
    
    // 创建 16 字节的 UUID
    uuid := make([]byte, 16)
    
    // 前 6 字节为时间戳
    binary.BigEndian.PutUint64(uuid[0:6], timestamp)
    
    // 设置版本号和变体
    uuid[6] = (uuid[6] & 0x0F) | 0x70  // 版本 7
    uuid[8] = (uuid[8] & 0x3F) | 0x80  // 变体 10
    
    // 剩余字节为随机数
    rand.Read(uuid[6:16])
    
    // 格式化为标准 UUID 字符串
    return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", ...)
}
```

**关键特性**:
- **时间有序**: 基于时间戳，保证大致按时间排序
- **随机性**: 后 10 字节随机数，保证唯一性
- **性能优化**: 使用对象池减少内存分配
- **标准格式**: 符合 RFC 4122 规范

### 3. 实例 ID 管理

```go
type uidProvider struct {
    config       *Config
    logger       clog.Logger
    coord        coord.Provider
    snowflake    *snowflakeGenerator
    instanceID   int64
    leaseID      string
    closeChan    chan struct{}
}
```

**分布式模式**:
- 通过 `coord.Provider` 分配全局唯一的实例 ID
- 定期续期租约，确保实例 ID 的有效性
- 优雅释放资源，避免实例 ID 泄漏

**单机模式**:
- 使用随机实例 ID，适用于单机部署
- 无需协调服务，降低系统复杂度

### 4. 配置系统

```go
type Config struct {
    ServiceName   string `json:"serviceName"`   // 服务名称
    MaxInstanceID int    `json:"maxInstanceID"` // 最大实例 ID
}
```

**配置验证**:
- 服务名称不能为空
- 实例 ID 范围在 1-1023 之间
- 环境相关默认值

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
- 记录错误日志，便于排查问题

### 2. 实例 ID 分配失败

```go
func (p *uidProvider) acquireInstanceID(ctx context.Context) error {
    instanceID, leaseID, err := p.coord.Instance().Allocate(ctx, p.config.ServiceName, p.config.MaxInstanceID)
    if err != nil {
        return fmt.Errorf("分配实例 ID 失败: %w", err)
    }
    // ...
}
```

**处理策略**:
- 重试机制（最多 3 次）
- 错误日志记录
- 优雅降级（单机模式）

### 3. 资源清理

```go
func (p *uidProvider) Close() error {
    p.closeOnce.Do(func() {
        close(p.closeChan)
        if p.leaseID != "" && p.coord != nil {
            p.coord.Instance().Release(ctx, p.leaseID)
        }
    })
    return nil
}
```

**处理策略**:
- 使用 `sync.Once` 确保只执行一次
- 释放实例 ID 租约
- 关闭后台协程

## 🎨 性能优化

### 1. Snowflake 性能优化

**批量生成**:
```go
func (g *snowflakeGenerator) GenerateBatch(count int) ([]int64, error) {
    // 减少锁竞争，一次性生成多个 ID
    // 适用于需要大量 ID 的场景
}
```

**内存对齐**:
- 使用 int64 类型确保原子操作
- 避免伪共享，提高缓存命中率

### 2. UUID v7 性能优化

**对象池**:
```go
var uuidPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 16)
    },
}
```

**预分配缓冲区**:
- 减少内存分配开销
- 提高并发性能

## 📊 并发安全性

### 1. Snowflake 生成器

**互斥锁保护**:
```go
func (g *snowflakeGenerator) Generate() (int64, error) {
    g.mu.Lock()
    defer g.mu.Unlock()
    // ...
}
```

**原子操作**:
- 时间戳和序列号的更新是原子的
- 避免竞态条件

### 2. UUID v7 生成器

**无状态设计**:
- 每次生成都是独立的
- 不需要锁保护
- 天然支持高并发

## 🔄 生命周期管理

### 1. 初始化流程

```
配置验证 → 选项解析 → 实例 ID 分配 → 生成器初始化 → 后台协程启动
```

### 2. 运行时管理

```
ID 生成请求 → 算法处理 → 结果返回
                  ↓
            租约续期协程
```

### 3. 关闭流程

```
关闭信号 → 停止后台协程 → 释放实例 ID → 资源清理
```

## 🔧 配置最佳实践

### 1. 服务名称设置

```go
config := &uid.Config{
    ServiceName:   "order-service",      // 清晰的服务标识
    MaxInstanceID: 100,                  // 根据集群规模设置
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
// 小型集群（< 100 实例）
config.MaxInstanceID = 100

// 中型集群（< 500 实例）
config.MaxInstanceID = 500

// 大型集群（< 1024 实例）
config.MaxInstanceID = 1023
```

## 📈 监控和可观测性

### 1. 关键指标

- **ID 生成速率**: 每秒生成的 ID 数量
- **错误率**: 生成失败的比率
- **延迟分布**: ID 生成耗时分布
- **实例 ID 使用率**: 已分配实例 ID 的比例

### 2. 日志记录

```go
clog.Info("ID 生成统计",
    clog.String("service", config.ServiceName),
    clog.Int64("generated_count", totalCount),
    clog.Float64("error_rate", errorRate),
    clog.Int64("instance_id", instanceID),
)
```

### 3. 健康检查

- 实例 ID 租约状态
- 协调服务连接状态
- 时钟同步状态

## 🔮 未来扩展

### 1. 算法扩展

- **UUID v8**: 支持自定义哈希算法
- **雪花 ID 变体**: 支持不同的位分配方案
- **分段 ID**: 支持业务相关的分段生成

### 2. 功能扩展

- **ID 模板**: 支持业务定制的 ID 格式
- **批量导入**: 支持外部 ID 批量导入
- **ID 查询**: 支持基于时间戳的 ID 查询

### 3. 性能优化

- **异步生成**: 支持异步 ID 生成队列
- **本地缓存**: 缓存生成的 ID 提高性能
- **分布式缓存**: 支持跨实例的 ID 缓存

## 🎯 设计总结

`uid` 组件通过提供 Snowflake 和 UUID v7 两种生成算法，满足了不同业务场景的需求。其设计具有以下特点：

1. **高可用性**: 支持单机和分布式两种部署模式
2. **高性能**: Snowflake 算法支持每秒数十万 ID 生成
3. **易使用**: 统一的 API 接口，简化使用复杂度
4. **可扩展**: 支持未来算法和功能的扩展
5. **可观测**: 完整的监控和日志支持

这个设计使 `uid` 成为 infra-kit 项目中不可或缺的基础设施组件。