# uid - infra-kit 唯一标识符生成组件

`uid` 是 infra-kit 项目的官方唯一标识符生成组件，提供 Snowflake 和 UUID v7 两种生成算法，满足不同业务场景的需求。

## 🚀 快速开始

### 基础初始化

```go
import (
    "context"
    "github.com/ceyewan/infra-kit/uid"
)

// 创建配置
config := uid.GetDefaultConfig("production")
config.ServiceName = "my-service"

// 创建 uid Provider
provider, err := uid.New(context.Background(), config)
if err != nil {
    log.Fatal(err)
}
defer provider.Close()
```

### 生成 UUID v7

```go
// 生成 UUID v7，适用于请求 ID、会话 ID 等场景
requestID := provider.GetUUIDV7()
fmt.Printf("Request ID: %s\n", requestID)
// 输出: 0189d1b0-7a7e-7b3e-8c4d-123456789012

// 验证 UUID 格式
isValid := provider.IsValidUUID(requestID)
fmt.Printf("Valid UUID: %t\n", isValid)
// 输出: Valid UUID: true
```

### 生成 Snowflake ID

```go
// 生成 Snowflake ID，适用于数据库主键、消息 ID 等场景
orderID, err := provider.GenerateSnowflake()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Order ID: %d\n", orderID)
// 输出: Order ID: 1234567890123456789

// 解析 Snowflake ID
timestamp, instanceID, sequence := provider.ParseSnowflake(orderID)
fmt.Printf("Timestamp: %d, InstanceID: %d, Sequence: %d\n", 
    timestamp, instanceID, sequence)
```

## 📋 API 参考

### Provider 接口

```go
type Provider interface {
    // 生成 UUID v7 格式的唯一标识符
    GetUUIDV7() string
    
    // 生成 Snowflake 格式的唯一标识符
    GenerateSnowflake() (int64, error)
    
    // 验证 UUID 格式
    IsValidUUID(s string) bool
    
    // 解析 Snowflake ID
    ParseSnowflake(id int64) (timestamp, instanceID, sequence int64)
    
    // 释放资源
    Close() error
}
```

### 配置结构

```go
type Config struct {
    ServiceName   string `json:"serviceName"`   // 服务名称
    MaxInstanceID int    `json:"maxInstanceID"` // 最大实例 ID，默认 1023
}

// 获取环境相关默认配置
func GetDefaultConfig(env string) *Config

// 验证配置
func (c *Config) Validate() error
```

### 函数式选项

```go
// 注入日志依赖
func WithLogger(logger clog.Logger) Option

// 注入协调服务依赖
func WithCoordProvider(provider coord.Provider) Option
```

## ⚙️ 使用场景

### 1. 数据库主键生成

```go
type OrderService struct {
    uidProvider uid.Provider
    db          sql.DB
}

func (s *OrderService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    // 生成订单 ID
    orderID, err := s.uidProvider.GenerateSnowflake()
    if err != nil {
        return nil, fmt.Errorf("生成订单 ID 失败: %w", err)
    }

    order := &Order{
        ID:        orderID,
        UserID:    req.UserID,
        Amount:    req.Amount,
        Status:    "pending",
        CreatedAt: time.Now(),
    }

    // 保存到数据库
    result := s.db.ExecContext(ctx, 
        "INSERT INTO orders (id, user_id, amount, status, created_at) VALUES (?, ?, ?, ?, ?)",
        order.ID, order.UserID, order.Amount, order.Status, order.CreatedAt)
    
    if result.Error != nil {
        return nil, result.Error
    }

    return order, nil
}
```

### 2. HTTP 请求追踪

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

### 3. 会话管理

```go
type SessionService struct {
    uidProvider uid.Provider
    cache       cache.Provider
}

func (s *SessionService) CreateSession(ctx context.Context, userID string) (*Session, error) {
    // 生成会话 ID
    sessionID := s.uidProvider.GetUUIDV7()

    session := &Session{
        ID:        sessionID,
        UserID:    userID,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }

    // 存储会话信息
    sessionData, _ := json.Marshal(session)
    if err := s.cache.Set(ctx, fmt.Sprintf("session:%s", sessionID),
        sessionData, 24*time.Hour); err != nil {
        return nil, fmt.Errorf("存储会话失败: %w", err)
    }

    return session, nil
}
```

### 4. 消息队列 ID 生成

```go
type MessageProducer struct {
    uidProvider uid.Provider
    mq          mq.Provider
}

func (p *MessageProducer) SendMessage(ctx context.Context, payload interface{}) error {
    // 生成消息 ID
    messageID, err := p.uidProvider.GenerateSnowflake()
    if err != nil {
        return fmt.Errorf("生成消息 ID 失败: %w", err)
    }

    message := &Message{
        ID:      messageID,
        Payload: payload,
        Created: time.Now(),
    }

    // 发送到消息队列
    if err := p.mq.Publish(ctx, "orders", message); err != nil {
        return fmt.Errorf("发送消息失败: %w", err)
    }

    return nil
}
```

## 🏗️ 部署模式

### 单机模式

```go
// 单机模式，无需协调服务
config := &uid.Config{
    ServiceName:   "standalone-service",
    MaxInstanceID: 10,
}

provider, err := uid.New(ctx, config)
if err != nil {
    log.Fatal(err)
}
```

### 分布式模式

```go
// 分布式模式，需要协调服务
config := &uid.Config{
    ServiceName:   "distributed-service",
    MaxInstanceID: 100,
}

// 注入协调服务
provider, err := uid.New(ctx, config, 
    uid.WithCoordProvider(coordProvider))
if err != nil {
    log.Fatal(err)
}
```

## 📊 性能特性

### Snowflake 算法

- **生成速度**: 每秒可生成数十万个 ID
- **时间排序**: ID 按时间大致排序
- **分布式安全**: 通过实例 ID 保证全局唯一性
- **时钟容错**: 检测时钟回拨，避免 ID 重复

### UUID v7 算法

- **全局唯一**: 基于时间戳和随机数，保证唯一性
- **时间有序**: 大致按时间排序，便于索引
- **标准格式**: 符合 RFC 4122 规范
- **高性能**: 无状态设计，支持高并发

## 🔄 错误处理

### 配置错误

```go
config := &uid.Config{
    ServiceName: "", // 空服务名称
}

provider, err := uid.New(ctx, config)
if err != nil {
    // 处理配置错误
    fmt.Printf("配置错误: %v\n", err)
    // 输出: 配置错误: 服务名称不能为空
}
```

### 生成错误

```go
snowflakeID, err := provider.GenerateSnowflake()
if err != nil {
    // 处理生成错误
    switch {
    case strings.Contains(err.Error(), "时钟回拨"):
        // 时钟回拨错误
        log.Printf("检测到时钟回拨: %v", err)
    case strings.Contains(err.Error(), "实例 ID"):
        // 实例 ID 相关错误
        log.Printf("实例 ID 错误: %v", err)
    default:
        // 其他错误
        log.Printf("生成 ID 失败: %v", err)
    }
}
```

## 🎯 最佳实践

### 1. ID 选择指南

| 场景 | 推荐算法 | 原因 |
|------|----------|------|
| 数据库主键 | Snowflake | 排序性好，索引友好 |
| 请求 ID | UUID v7 | 全局唯一，可读性好 |
| 会话 ID | UUID v7 | 安全性高，不易猜测 |
| 消息 ID | Snowflake | 时间排序，便于追踪 |
| 外部资源 ID | UUID v7 | 不暴露内部信息 |

### 2. 配置建议

```go
// 小型服务（单实例）
config := &uid.Config{
    ServiceName:   "small-service",
    MaxInstanceID: 10,
}

// 中型服务（多实例）
config := &uid.Config{
    ServiceName:   "medium-service",
    MaxInstanceID: 100,
}

// 大型服务（分布式）
config := &uid.Config{
    ServiceName:   "large-service",
    MaxInstanceID: 1023,
}
```

### 3. 资源管理

```go
// 使用 defer 确保资源释放
func createUserHandler(c *gin.Context) {
    provider, err := uid.New(ctx, config)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    defer provider.Close() // 确保释放资源
    
    // 处理业务逻辑
    userID := provider.GetUUIDV7()
    // ...
}
```

## 📝 使用示例

更多使用示例请参考：

- **[基本用法](examples/main.go)**: 基础功能和配置示例
- **[设计文档](DESIGN.md)**: 详细的架构设计和实现原理
- **[使用指南](../../docs/uid.md)**: 完整的使用指南和最佳实践

## 🧪 测试

```bash
# 运行所有测试
go test -v ./...

# 运行基准测试
go test -bench=. -benchmem ./...

# 运行特定测试
go test -v -run=TestSnowflakeGeneration ./...
```

## 📈 监控

建议监控以下指标：

- **ID 生成速率**: 每秒生成的 ID 数量
- **错误率**: 生成失败的比率
- **延迟分布**: ID 生成耗时分布
- **实例 ID 使用率**: 已分配实例 ID 的比例

## 🔄 版本兼容性

- **Go 1.18+**: 需要 Go 1.18 或更高版本
- **infra-kit**: 与 infra-kit 其他组件兼容
- **向后兼容**: 保持 API 的向后兼容性

## 📄 许可证

MIT License - 详见 [LICENSE](../../LICENSE) 文件