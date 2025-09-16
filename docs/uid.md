# 唯一 ID 生成组件实现指南

## 1. 设计理念

`uid` 组件是一个统一的唯一标识符生成组件，提供两种不同的 ID 生成方案来满足各种业务场景的需求。

### 核心设计原则

- **多算法支持**: 同时支持 Snowflake 和 UUID v7 两种生成算法
- **场景适配**: 为不同的使用场景提供最合适的 ID 类型
- **高性能**: Snowflake ID 生成速度极快，适合高并发场景
- **分布式安全**: 通过协调服务保证实例 ID 的唯一性
- **易于使用**: 统一的 API 接口，简化使用复杂度

### 应用场景

- **Snowflake ID**: 数据库主键、消息 ID、订单号等需要排序和高性能的场景
- **UUID v7**: 请求 ID、会话 ID、外部资源 ID 等需要全局唯一性和可读性的场景

## 2. 核心 API 契约

### 2.1 构造函数与配置

```go
// Config 组件配置结构
type Config struct {
    ServiceName   string `json:"serviceName"`   // 服务名称，用于实例ID分配
    MaxInstanceID int    `json:"maxInstanceID"` // 最大实例ID，默认 1023
}

// GetDefaultConfig 返回默认配置
func GetDefaultConfig(env string) *Config

// Option 功能选项
type Option func(*options)

// WithLogger 注入日志依赖
func WithLogger(logger clog.Logger) Option

// WithCoordProvider 注入协调服务依赖
func WithCoordProvider(provider coord.Provider) Option

// New 创建 ID 生成组件实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
```

### 2.2 Provider 接口设计

```go
// Provider ID 生成组件主接口
type Provider interface {
    GetUUIDV7() string                             // 生成 UUID v7
    GenerateSnowflake() (int64, error)            // 生成 Snowflake ID
    IsValidUUID(s string) bool                     // 验证 UUID 格式
    ParseSnowflake(id int64) (timestamp, instanceID, sequence int64) // 解析 Snowflake ID
    Close() error                                  // 释放资源
}
```

## 3. 实现要点

### 3.1 Snowflake 算法实现

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

    maxInstanceID = (1 << instanceIDBits) - 1
    maxSequence   = (1 << sequenceBits) - 1

    instanceIDShift = sequenceBits
    timestampShift  = instanceIDBits + sequenceBits
)

func (g *snowflakeGenerator) Generate() (int64, error) {
    g.mu.Lock()
    defer g.mu.Unlock()

    currentTime := time.Now().UnixMilli() - g.epoch

    if currentTime < g.lastTime {
        return 0, fmt.Errorf("时钟回拨，无法生成 ID")
    }

    if currentTime == g.lastTime {
        g.sequence = (g.sequence + 1) & maxSequence
        if g.sequence == 0 {
            // 序列号用完，等待下一毫秒
            for currentTime <= g.lastTime {
                currentTime = time.Now().UnixMilli() - g.epoch
            }
        }
    } else {
        g.sequence = 0
    }

    g.lastTime = currentTime

    return (currentTime << timestampShift) |
           (g.instanceID << instanceIDShift) |
           g.sequence, nil
}
```

### 3.2 UUID v7 实现

```go
func generateUUIDV7() string {
    // 获取当前时间戳（毫秒级）
    timestamp := time.Now().UnixMilli()

    // 转换为字节（大端序）
    timestampBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(timestampBytes, uint64(timestamp))

    // 创建 16 字节的 UUID
    uuid := make([]byte, 16)

    // 前 6 字节为时间戳（48位）
    copy(uuid[0:6], timestampBytes[2:8])

    // 第 7 字节的高 4 位为版本号（0111）
    uuid[6] = (timestampBytes[8] & 0x0F) | 0x70

    // 第 8 字节的高 2 位为变体（10）
    uuid[8] = (timestampBytes[9] & 0x3F) | 0x80

    // 剩余字节为随机数
    randomBytes := make([]byte, 6)
    if _, err := rand.Read(randomBytes); err != nil {
        // 如果随机数生成失败，使用时间戳的低 6 字节
        copy(randomBytes, timestampBytes[10:16])
    }
    copy(uuid[10:16], randomBytes)

    // 格式化为标准 UUID 字符串
    return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
        binary.BigEndian.Uint32(uuid[0:4]),
        binary.BigEndian.Uint16(uuid[4:6]),
        binary.BigEndian.Uint16(uuid[6:8]),
        binary.BigEndian.Uint16(uuid[8:10]),
        binary.BigEndian.Uint64(uuid[10:16]),
    )
}
```

### 3.3 实例 ID 管理

```go
type uidProvider struct {
    config       *Config
    logger       clog.Logger
    coord        coord.Provider
    snowflake    *snowflakeGenerator
    instanceID   int64
    leaseID      string
}

func (p *uidProvider) acquireInstanceID(ctx context.Context) error {
    if p.coord == nil {
        return fmt.Errorf("需要 coord.Provider 来分配实例 ID")
    }

    // 通过协调服务分配实例 ID
    instanceID, leaseID, err := p.coord.Instance().Allocate(ctx, p.config.ServiceName, p.config.MaxInstanceID)
    if err != nil {
        return fmt.Errorf("分配实例 ID 失败: %w", err)
    }

    p.instanceID = instanceID
    p.leaseID = leaseID

    // 初始化 Snowflake 生成器
    p.snowflake = &snowflakeGenerator{
        instanceID: instanceID,
        epoch:      snowflakeEpoch,
    }

    // 启动租约续期协程
    go p.renewInstanceLease(ctx)

    return nil
}

func (p *uidProvider) renewInstanceLease(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := p.coord.Instance().Renew(ctx, p.leaseID); err != nil {
                p.logger.Error("实例 ID 租约续期失败", clog.Err(err))
            }
        }
    }
}
```

### 3.4 资源清理

```go
func (p *uidProvider) Close() error {
    if p.leaseID != "" && p.coord != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        // 释放实例 ID
        if err := p.coord.Instance().Release(ctx, p.leaseID); err != nil {
            p.logger.Error("释放实例 ID 失败", clog.Err(err))
            return err
        }
    }

    return nil
}
```

## 4. 标准用法示例

### 4.1 基础初始化

```go
func main() {
    ctx := context.Background()

    // 1. 初始化日志
    clog.Init(ctx, clog.GetDefaultConfig("production"),
        clog.WithNamespace("uid-service"))

    // 2. 创建 ID 生成组件
    config := uid.GetDefaultConfig("production")
    config.ServiceName = "order-service"

    uidProvider, err := uid.New(ctx, config,
        uid.WithLogger(clog.Namespace("uid")),
        uid.WithCoordProvider(coordProvider), // Snowflake 需要 coord
    )
    if err != nil {
        clog.Fatal("UID 组件初始化失败", clog.Err(err))
    }
    defer uidProvider.Close()
}
```

### 4.2 数据库主键生成

```go
type OrderService struct {
    uid uid.Provider
    db  db.Provider
}

func (s *OrderService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    logger := clog.WithContext(ctx)

    // 生成订单 ID
    orderID, err := s.uid.GenerateSnowflake()
    if err != nil {
        return nil, fmt.Errorf("生成订单 ID 失败: %w", err)
    }

    order := &Order{
        ID:        orderID,
        UserID:    req.UserID,
        ProductID: req.ProductID,
        Quantity:  req.Quantity,
        Amount:    req.Amount,
        Status:    "pending",
        CreatedAt: time.Now(),
    }

    // 保存到数据库
    if err := s.db.DB(ctx).Create(order).Error; err != nil {
        return nil, fmt.Errorf("保存订单失败: %w", err)
    }

    logger.Info("订单创建成功", clog.Int64("order_id", orderID))
    return order, nil
}
```

### 4.3 请求 ID 生成

```go
func RequestIDMiddleware(uidProvider uid.Provider) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 检查请求头中是否已有 Request-ID
        requestID := c.GetHeader("X-Request-ID")

        if requestID == "" || !uidProvider.IsValidUUID(requestID) {
            // 生成新的请求 ID
            requestID = uidProvider.GetUUIDV7()
        }

        // 设置到响应头
        c.Header("X-Request-ID", requestID)

        // 注入到日志上下文
        ctx := clog.WithTraceID(c.Request.Context(), requestID)
        c.Request = c.Request.WithContext(ctx)

        c.Next()
    }
}
```

### 4.4 会话 ID 生成

```go
type SessionService struct {
    uid   uid.Provider
    cache cache.Provider
}

func (s *SessionService) CreateSession(ctx context.Context, userID string) (*Session, error) {
    // 生成会话 ID
    sessionID := s.uid.GetUUIDV7()

    session := &Session{
        ID:        sessionID,
        UserID:    userID,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }

    // 存储会话信息
    sessionData, _ := json.Marshal(session)
    if err := s.cache.String().Set(ctx, fmt.Sprintf("session:%s", sessionID),
        sessionData, 24*time.Hour); err != nil {
        return nil, fmt.Errorf("存储会话失败: %w", err)
    }

    return session, nil
}
```

## 5. 测试策略

### 5.1 单元测试

```go
func TestSnowflakeGeneration(t *testing.T) {
    // 创建测试用的 uidProvider
    provider := &uidProvider{
        snowflake: &snowflakeGenerator{
            instanceID: 1,
            epoch:      snowflakeEpoch,
        },
    }

    // 生成 1000 个 ID 验证唯一性
    ids := make(map[int64]bool)
    for i := 0; i < 1000; i++ {
        id, err := provider.GenerateSnowflake()
        assert.NoError(t, err)
        assert.False(t, ids[id])
        ids[id] = true
    }
}

func TestUUIDV7Generation(t *testing.T) {
    provider := &uidProvider{}

    // 生成多个 UUID 验证格式
    for i := 0; i < 100; i++ {
        uuid := provider.GetUUIDV7()
        assert.True(t, provider.IsValidUUID(uuid))

        // 验证版本号为 7
        assert.Equal(t, '7', uuid[14])
    }
}
```

### 5.2 并发测试

```go
func TestConcurrentSnowflakeGeneration(t *testing.T) {
    provider := &uidProvider{
        snowflake: &snowflakeGenerator{
            instanceID: 1,
            epoch:      snowflakeEpoch,
        },
    }

    var wg sync.WaitGroup
    ids := make(chan int64, 10000)

    // 启动 10 个 goroutine 并发生成 ID
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                id, err := provider.GenerateSnowflake()
                assert.NoError(t, err)
                ids <- id
            }
        }()
    }

    wg.Wait()
    close(ids)

    // 验证所有 ID 唯一
    idSet := make(map[int64]bool)
    for id := range ids {
        assert.False(t, idSet[id])
        idSet[id] = true
    }
}
```

## 6. 性能优化

### 6.1 Snowflake 性能优化

```go
// 批量生成 Snowflake ID
func (g *snowflakeGenerator) GenerateBatch(count int) ([]int64, error) {
    g.mu.Lock()
    defer g.mu.Unlock()

    ids := make([]int64, count)
    currentTime := time.Now().UnixMilli() - g.epoch

    if currentTime < g.lastTime {
        return nil, fmt.Errorf("时钟回拨，无法生成 ID")
    }

    for i := 0; i < count; i++ {
        if currentTime == g.lastTime {
            g.sequence = (g.sequence + 1) & maxSequence
            if g.sequence == 0 {
                // 序列号用完，等待下一毫秒
                for currentTime <= g.lastTime {
                    currentTime = time.Now().UnixMilli() - g.epoch
                }
            }
        } else {
            g.sequence = 0
            g.lastTime = currentTime
        }

        ids[i] = (currentTime << timestampShift) |
                 (g.instanceID << instanceIDShift) |
                 g.sequence
    }

    return ids, nil
}
```

### 6.2 UUID v7 性能优化

```go
var uuidPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 16)
    },
}

func generateUUIDV7Optimized() string {
    // 从池中获取字节切片
    uuid := uuidPool.Get().([]byte)
    defer uuidPool.Put(uuid)

    // 获取当前时间戳
    timestamp := time.Now().UnixMilli()

    // 填充时间戳部分
    timestampBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(timestampBytes, uint64(timestamp))

    copy(uuid[0:6], timestampBytes[2:8])
    uuid[6] = (timestampBytes[8] & 0x0F) | 0x70
    uuid[8] = (timestampBytes[9] & 0x3F) | 0x80

    // 填充随机部分
    randomBytes := make([]byte, 6)
    if _, err := rand.Read(randomBytes); err == nil {
        copy(uuid[10:16], randomBytes)
    } else {
        // 降级到时间戳
        copy(uuid[10:16], timestampBytes[10:16])
    }

    // 格式化为字符串
    return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
        binary.BigEndian.Uint32(uuid[0:4]),
        binary.BigEndian.Uint16(uuid[4:6]),
        binary.BigEndian.Uint16(uuid[6:8]),
        binary.BigEndian.Uint16(uuid[8:10]),
        binary.BigEndian.Uint64(uuid[10:16]),
    )
}
```

## 7. 错误处理

### 7.1 时钟回拨处理

```go
func (g *snowflakeGenerator) handleClockBackward(currentTime int64) error {
    // 记录时钟回拨事件
    log.Printf("检测到时钟回拨，上次时间: %d, 当前时间: %d", g.lastTime, currentTime)

    // 如果回拨时间较小，等待时钟追上
    if g.lastTime-currentTime < 1000 { // 1秒内的回拨
        time.Sleep(time.Duration(g.lastTime - currentTime) * time.Millisecond)
        return nil
    }

    // 如果回拨时间较大，使用备用策略
    return fmt.Errorf("时钟回拨时间过长，无法生成 ID")
}
```

### 7.2 实例 ID 分配失败处理

```go
func (p *uidProvider) acquireInstanceIDWithRetry(ctx context.Context) error {
    const maxRetries = 3
    const retryDelay = 2 * time.Second

    var lastErr error

    for i := 0; i < maxRetries; i++ {
        err := p.acquireInstanceID(ctx)
        if err == nil {
            return nil
        }

        lastErr = err
        p.logger.Error("实例 ID 分配失败，准备重试",
            clog.Int("attempt", i+1),
            clog.Err(err))

        if i < maxRetries-1 {
            time.Sleep(retryDelay)
        }
    }

    return fmt.Errorf("实例 ID 分配失败，已重试 %d 次: %w", maxRetries, lastErr)
}
```

## 8. 最佳实践

### 8.1 ID 选择指南

- **数据库主键**: 使用 Snowflake ID，便于排序和索引
- **外部 API ID**: 使用 UUID v7，避免暴露内部信息
- **消息 ID**: 使用 Snowflake ID，便于消息排序和追踪
- **会话 ID**: 使用 UUID v7，保证全局唯一性
- **日志追踪 ID**: 使用 UUID v7，便于跨系统追踪

### 8.2 性能考虑

- **Snowflake**: 单机每秒可生成数十万个 ID，满足大多数业务需求
- **UUID v7**: 生成速度稍慢，但仍然是高性能的
- **批量生成**: 对于需要大量 ID 的场景，使用批量生成接口

### 8.3 部署建议

- **实例 ID 管理**: 使用协调服务进行实例 ID 分配和管理
- **时钟同步**: 确保服务器时钟同步，避免时钟回拨问题
- **监控**: 监控 ID 生成性能和错误率
- **备份**: 准备备用 ID 生成策略，防止服务不可用

---

*遵循这些指南可以确保 ID 生成组件的高质量实现和稳定运行。*
