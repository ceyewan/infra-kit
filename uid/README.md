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
// 输出: 019952f1-9079-771c-831b-f88b1189e4b6

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
// 输出: Order ID: 623164467712724992

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
    MaxInstanceID int    `json:"maxInstanceID"` // 最大实例 ID (1-1023)
    InstanceID    int    `json:"instanceId"`    // 实例 ID (0=自动分配)
}

// 获取环境相关默认配置
func GetDefaultConfig(env string) *Config

// 验证配置
func (c *Config) Validate() error

// 配置设置方法
func (c *Config) SetServiceName(name string) *Config
func (c *Config) SetMaxInstanceID(maxID int) *Config
func (c *Config) SetInstanceID(instanceID int) *Config
```

### 函数式选项

```go
// 注入日志依赖
func WithLogger(logger clog.Logger) Option
```

## ⚙️ 配置方式

### 1. 代码配置

```go
// 指定实例 ID
config := &uid.Config{
    ServiceName:   "order-service",
    MaxInstanceID: 100,
    InstanceID:    5, // 指定实例 ID
}

// 自动分配实例 ID
config := &uid.Config{
    ServiceName:   "order-service",
    MaxInstanceID: 100,
    InstanceID:    0, // 0 表示自动分配
}
```

### 2. 环境变量配置

```bash
# 设置环境变量
export SERVICE_NAME=order-service
export MAX_INSTANCE_ID=100
export INSTANCE_ID=5

# 在代码中使用
config := uid.GetDefaultConfig("production")
// config.ServiceName = "order-service" (来自环境变量)
// config.InstanceID = 5 (来自环境变量)
```

### 3. 容器化部署

```yaml
# docker-compose.yml
services:
  order-service:
    image: order-service:latest
    environment:
      - SERVICE_NAME=order-service
      - MAX_INSTANCE_ID=100
      # 为每个实例分配不同的 INSTANCE_ID
      - INSTANCE_ID=${INSTANCE_ID:-0}
    deploy:
      replicas: 3
```

## 🏗️ 部署模式

### 单机模式

```go
// 单机模式，自动分配实例 ID
config := &uid.Config{
    ServiceName:   "standalone-service",
    MaxInstanceID: 10,
    InstanceID:    0, // 自动分配
}

provider, err := uid.New(ctx, config)
if err != nil {
    log.Fatal(err)
}
```

### 多实例模式

```go
// 方法 1: 通过配置文件分配
config := &uid.Config{
    ServiceName:   "multi-instance-service",
    MaxInstanceID: 100,
    InstanceID:    getInstanceIDFromConfig(), // 从配置读取
}

// 方法 2: 通过环境变量分配
config := uid.GetDefaultConfig("production")
// 实例 ID 从环境变量读取

provider, err := uid.New(ctx, config)
if err != nil {
    log.Fatal(err)
}
```

### Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: order-service
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: order-service
        env:
        - name: SERVICE_NAME
          value: "order-service"
        - name: MAX_INSTANCE_ID
          value: "100"
        - name: INSTANCE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.uid
```

## 🎯 使用场景

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

## 📊 性能特性

### Snowflake 算法

- **生成速度**: 每秒可生成数十万个 ID
- **时间排序**: ID 按时间大致排序
- **实例唯一性**: 通过实例 ID 保证多实例环境下的唯一性
- **时钟容错**: 检测时钟回拨，避免 ID 重复

**位分配**:
- 时间戳: 42 位 (69 年可用)
- 实例 ID: 10 位 (最多 1024 个实例)
- 序列号: 12 位 (每毫秒 4096 个 ID)

### UUID v7 算法

- **全局唯一**: 基于时间戳和随机数，保证唯一性
- **时间有序**: 大致按时间排序，便于索引
- **标准格式**: 符合 RFC 4122 规范
- **高性能**: 无状态设计，支持高并发

**格式**:
- 前 6 字节: 时间戳 (48 位)
- 第 7 字节: 版本号 (0111)
- 第 8 字节: 变体 (10xx)
- 后 10 字节: 随机数

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
        // 等待时钟同步或使用备用策略
        time.Sleep(time.Second)
        snowflakeID, err = provider.GenerateSnowflake()
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

### 2. 实例 ID 规划

```go
// 单实例服务
config.MaxInstanceID = 1
config.InstanceID = 1

// 小型集群 (3-5 实例)
config.MaxInstanceID = 10
config.InstanceID = getInstanceID() // 1-10

// 中型集群 (10-100 实例)
config.MaxInstanceID = 100
config.InstanceID = getInstanceID() // 1-100

// 大型集群 (100-1024 实例)
config.MaxInstanceID = 1023
config.InstanceID = getInstanceID() // 1-1023
```

### 3. 容器化最佳实践

```yaml
# docker-compose.yml 示例
version: '3.8'
services:
  order-service-1:
    image: order-service:latest
    environment:
      - SERVICE_NAME=order-service
      - MAX_INSTANCE_ID=100
      - INSTANCE_ID=1
  
  order-service-2:
    image: order-service:latest
    environment:
      - SERVICE_NAME=order-service
      - MAX_INSTANCE_ID=100
      - INSTANCE_ID=2
```

### 4. 资源管理

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

## 📈 监控和可观测性

### 关键指标

- **ID 生成速率**: 每秒生成的 ID 数量
- **错误率**: 生成失败的比率
- **延迟分布**: ID 生成耗时分布
- **实例 ID 使用率**: 已分配实例 ID 的比例

### 日志记录示例

```go
clog.Info("ID 生成统计",
    clog.String("service", config.ServiceName),
    clog.Int64("generated_count", totalCount),
    clog.Float64("error_rate", errorRate),
    clog.Int64("instance_id", instanceID),
)
```

### 健康检查

- 实例 ID 配置状态
- 时钟同步状态
- 组件初始化状态

## 🧪 测试

```bash
# 运行所有测试
go test -v ./...

# 运行基准测试
go test -bench=. -benchmem ./...

# 运行特定测试
go test -v -run=TestSnowflakeGeneration ./...
```

## 📚 相关文档

- **[设计文档](DESIGN.md)**: 详细的架构设计和实现原理
- **[使用示例](examples/main.go)**: 实际使用场景的代码示例

## 🔄 版本兼容性

- **Go 1.18+**: 需要 Go 1.18 或更高版本
- **infra-kit**: 与 infra-kit 其他组件兼容
- **向后兼容**: 保持 API 的向后兼容性

## 🔮 未来规划

### 已知限制

- 当前实现不支持分布式实例 ID 管理
- 批量生成功能暂未提供 (存在并发安全问题)
- 缺少动态配置更新支持

### 计划功能

- **分布式支持**: 集成 coord 组件实现分布式实例 ID 管理
- **批量生成**: 解决并发安全问题，支持批量 ID 生成
- **动态配置**: 支持运行时配置更新
- **更多算法**: 支持 UUID v8 等新算法

## 📄 许可证

MIT License - 详见 [LICENSE](../../LICENSE) 文件