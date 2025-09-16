# 基础设施: mq 消息队列

## 1. 设计理念

`mq` 是 `infra-kit` 项目的消息队列组件，基于 `Kafka` 构建。它为微服务架构提供了统一、可靠的消息传递解决方案。

### 核心设计原则

- **极简API**: 提供简洁的生产者和消费者接口，降低使用门槛
- **配置驱动**: 通过配置中心管理 Kafka 配置，支持动态热更新
- **自动管理**: 自动处理偏移量提交、重试、连接管理等复杂逻辑
- **链路追踪**: 自动在消息中传递 trace_id，实现分布式调用链追踪
- **高可靠性**: 支持多种确认级别和重试机制，确保消息传递可靠性

### 组件价值

- **异步通信**: 支持服务间的异步通信，提高系统响应速度
- **解耦合**: 通过消息队列实现服务间的松耦合
- **流量削峰**: 缓冲突发流量，保护下游服务
- **数据分发**: 支持一对多的消息分发模式

## 2. 核心 API 契约

### 2.1 构造函数与配置

```go
// Config 是 mq 组件的配置结构体
type Config struct {
    Brokers          []string        `json:"brokers"`           // Kafka 集群地址列表
    SecurityProtocol string          `json:"securityProtocol"`  // 安全协议
    SASLMechanism    string          `json:"saslMechanism"`    // SASL 认证机制
    SASLUsername     string          `json:"saslUsername"`     // SASL 用户名
    SASLPassword     string          `json:"saslPassword"`     // SASL 密码
    ProducerConfig   *ProducerConfig `json:"producerConfig"`   // 生产者配置
    ConsumerConfig   *ConsumerConfig `json:"consumerConfig"`   // 消费者配置
}

// ProducerConfig 生产者配置
type ProducerConfig struct {
    Acks        int `json:"acks"`         // 确认级别: 0, 1, -1(all)
    RetryMax    int `json:"retryMax"`     // 最大重试次数
    BatchSize   int `json:"batchSize"`    // 批处理大小
    LingerMs    int `json:"lingerMs"`     // 延迟发送时间
    Compression int `json:"compression"`  // 压缩类型
}

// ConsumerConfig 消费者配置
type ConsumerConfig struct {
    AutoOffsetReset      string `json:"autoOffsetReset"`      // 偏移量重置策略
    EnableAutoCommit     bool   `json:"enableAutoCommit"`     // 启用自动提交
    AutoCommitIntervalMs int    `json:"autoCommitIntervalMs"` // 自动提交间隔
    SessionTimeoutMs     int    `json:"sessionTimeoutMs"`     // 会话超时时间
    MaxPollRecords       int    `json:"maxPollRecords"`       // 最大拉取记录数
    MaxPollIntervalMs    int    `json:"maxPollIntervalMs"`    // 最大拉取间隔
}

// GetDefaultConfig 返回环境相关的默认配置
func GetDefaultConfig(env string) *Config

// Option 功能选项
type Option func(*options)

// WithLogger 注入日志依赖
func WithLogger(logger clog.Logger) Option

// WithCoordProvider 注入配置中心依赖
func WithCoordProvider(coord coord.Provider) Option

// WithMetricsProvider 注入监控依赖
func WithMetricsProvider(metrics metrics.Provider) Option

// NewProducer 创建消息生产者
func NewProducer(ctx context.Context, config *Config, opts ...Option) (Producer, error)

// NewConsumer 创建消息消费者
func NewConsumer(ctx context.Context, config *Config, groupID string, opts ...Option) (Consumer, error)
```

### 2.2 消息结构

```go
// Message 消息结构
type Message struct {
    Topic   string            `json:"topic"`   // 主题
    Key     []byte            `json:"key"`     // 消息键
    Value   []byte            `json:"value"`   // 消息值
    Headers map[string][]byte `json:"headers"` // 消息头
    Time    time.Time         `json:"time"`    // 消息时间
}

// MessageAck 消息确认
type MessageAck struct {
    Topic     string    `json:"topic"`     // 主题
    Partition int32     `json:"partition"` // 分区
    Offset    int64     `json:"offset"`    // 偏移量
    Error     error     `json:"error"`     // 错误信息
    Timestamp time.Time `json:"timestamp"` // 处理时间
}
```

### 2.3 生产者接口

```go
// Producer 消息生产者接口
type Producer interface {
    // Send 异步发送消息
    Send(ctx context.Context, msg *Message, callback func(*MessageAck)) error
    // SendSync 同步发送消息
    SendSync(ctx context.Context, msg *Message) (*MessageAck, error)
    // SendBatch 批量发送消息
    SendBatch(ctx context.Context, messages []*Message, callback func([]*MessageAck)) error
    // Flush 刷新缓冲区，确保所有消息都已发送
    Flush(ctx context.Context) error
    // Close 关闭生产者
    Close() error
}

// ProducerStats 生产者统计信息
type ProducerStats struct {
    MessagesSent     int64         `json:"messagesSent"`     // 发送消息数
    BytesSent        int64         `json:"bytesSent"`        // 发送字节数
    SendErrors       int64         `json:"sendErrors"`       // 发送错误数
    RetryCount       int64         `json:"retryCount"`       // 重试次数
    AverageLatency   time.Duration `json:"averageLatency"`   // 平均延迟
    CurrentQueueSize int           `json:"currentQueueSize"` // 当前队列大小
}
```

### 2.4 消费者接口

```go
// Consumer 消息消费者接口
type Consumer interface {
    // Subscribe 订阅主题
    Subscribe(ctx context.Context, topics []string, handler MessageHandler) error
    // SubscribeWithMetadata 订阅主题并获取元数据
    SubscribeWithMetadata(ctx context.Context, topics []string, handler MessageMetadataHandler) error
    // Commit 手动提交偏移量
    Commit(ctx context.Context) error
    // Seek 重置偏移量
    Seek(ctx context.Context, topic string, partition int32, offset int64) error
    // Pause 暂停消费
    Pause(ctx context.Context, topicPartitions map[string][]int32) error
    // Resume 恢复消费
    Resume(ctx context.Context, topicPartitions map[string][]int32) error
    // Close 关闭消费者
    Close() error
}

// MessageHandler 消息处理函数
type MessageHandler func(ctx context.Context, msg *Message) error

// MessageMetadataHandler 带元数据的消息处理函数
type MessageMetadataHandler func(ctx context.Context, msg *Message, metadata *ConsumerMetadata) error

// ConsumerMetadata 消费者元数据
type ConsumerMetadata struct {
    Topic     string    `json:"topic"`     // 主题
    Partition int32     `json:"partition"` // 分区
    Offset    int64     `json:"offset"`    // 偏移量
    Timestamp time.Time `json:"timestamp"` // 消息时间
    Headers   map[string][]byte `json:"headers"` // 消息头
}

// ConsumerStats 消费者统计信息
type ConsumerStats struct {
    MessagesConsumed    int64         `json:"messagesConsumed"`    // 消费消息数
    BytesConsumed       int64         `json:"bytesConsumed"`       // 消费字节数
    ProcessingErrors    int64         `json:"processingErrors"`    // 处理错误数
    CommitErrors        int64         `json:"commitErrors"`        // 提交错误数
    AverageProcessTime  time.Duration `json:"averageProcessTime"`  // 平均处理时间
    Lag                int64         `json:"lag"`                // 消费延迟
}
```

## 3. 实现要点

### 3.1 生产者实现

```go
type kafkaProducer struct {
    producer   sarama.SyncProducer
    asyncProd  sarama.AsyncProducer
    config     *Config
    logger     clog.Logger
    metrics    metrics.Provider
    stats      *ProducerStats
    statsMu    sync.RWMutex
}

func (p *kafkaProducer) Send(ctx context.Context, msg *Message, callback func(*MessageAck)) error {
    // 自动添加 trace_id 到消息头
    if msg.Headers == nil {
        msg.Headers = make(map[string][]byte)
    }

    if traceID := getTraceID(ctx); traceID != "" {
        msg.Headers["X-Trace-ID"] = []byte(traceID)
    }

    // 创建 Kafka 消息
    kafkaMsg := &sarama.ProducerMessage{
        Topic:   msg.Topic,
        Key:     sarama.ByteEncoder(msg.Key),
        Value:   sarama.ByteEncoder(msg.Value),
        Headers: convertHeaders(msg.Headers),
    }

    // 异步发送
    p.asyncProd.SendMessage(kafkaMsg, func(message *sarama.ProducerMessage, err error) {
        ack := &MessageAck{
            Topic:     message.Topic,
            Partition: message.Partition,
            Offset:    message.Offset,
            Timestamp: time.Now(),
        }

        if err != nil {
            ack.Error = err
            p.incrementError()
        } else {
            p.incrementSuccess(message)
        }

        callback(ack)
    })

    return nil
}

func (p *kafkaProducer) SendSync(ctx context.Context, msg *Message) (*MessageAck, error) {
    startTime := time.Now()

    // 添加 trace_id
    if msg.Headers == nil {
        msg.Headers = make(map[string][]byte)
    }

    if traceID := getTraceID(ctx); traceID != "" {
        msg.Headers["X-Trace-ID"] = []byte(traceID)
    }

    kafkaMsg := &sarama.ProducerMessage{
        Topic:   msg.Topic,
        Key:     sarama.ByteEncoder(msg.Key),
        Value:   sarama.ByteEncoder(msg.Value),
        Headers: convertHeaders(msg.Headers),
    }

    // 同步发送
    partition, offset, err := p.producer.SendMessage(kafkaMsg)
    if err != nil {
        p.incrementError()
        return nil, err
    }

    p.incrementSuccess(kafkaMsg)

    return &MessageAck{
        Topic:     msg.Topic,
        Partition: partition,
        Offset:    offset,
        Timestamp: time.Now(),
    }, nil
}
```

### 3.2 消费者实现

```go
type kafkaConsumer struct {
    consumer   sarama.ConsumerGroup
    handler    MessageHandler
    config     *Config
    groupID    string
    logger     clog.Logger
    metrics    metrics.Provider
    stats      *ConsumerStats
    statsMu    sync.RWMutex
}

type consumerHandler struct {
    handler MessageHandler
    logger  clog.Logger
    stats   *ConsumerStats
}

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    for {
        select {
        case msg, ok := <-claim.Messages():
            if !ok {
                return nil
            }

            startTime := time.Now()

            // 构造消息对象
            message := &Message{
                Topic:   msg.Topic,
                Key:     msg.Key,
                Value:   msg.Value,
                Headers: convertKafkaHeaders(msg.Headers),
                Time:    msg.Timestamp,
            }

            // 从消息头提取 trace_id
            ctx := context.Background()
            if traceID := getTraceIDFromHeaders(msg.Headers); traceID != "" {
                ctx = clog.WithTraceID(ctx, traceID)
            }

            // 处理消息
            err := h.handler(ctx, message)

            // 更新统计信息
            processTime := time.Since(startTime)
            h.stats.mu.Lock()
            h.stats.MessagesConsumed++
            h.stats.BytesConsumed += int64(len(msg.Value))
            h.stats.AverageProcessTime = time.Duration(
                (int64(h.stats.AverageProcessTime)*(h.stats.MessagesConsumed-1) + int64(processTime)) /
                h.stats.MessagesConsumed,
            )

            if err != nil {
                h.stats.ProcessingErrors++
            }
            h.stats.mu.Unlock()

            // 手动提交偏移量（如果配置为手动提交）
            if err == nil {
                session.MarkMessage(msg, "")
            }

        case <-session.Context().Done():
            return nil
        }
    }
}
```

### 3.3 连接管理和配置更新

```go
func (p *kafkaProducer) setupConfig(config *Config) *sarama.Config {
    saramaConfig := sarama.NewConfig()

    // 生产者配置
    saramaConfig.Producer.RequiredAcks = sarama.RequiredAcks(config.ProducerConfig.Acks)
    saramaConfig.Producer.Retry.Max = config.ProducerConfig.RetryMax
    saramaConfig.Producer.Retry.Backoff = 100 * time.Millisecond
    saramaConfig.Producer.Flush.MaxMessages = config.ProducerConfig.BatchSize
    saramaConfig.Producer.Flush.Frequency = time.Duration(config.ProducerConfig.LingerMs) * time.Millisecond
    saramaConfig.Producer.Compression = sarama.CompressionCodec(config.ProducerConfig.Compression)

    // 消费者配置
    saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
    if config.ConsumerConfig.AutoOffsetReset == "earliest" {
        saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
    }
    saramaConfig.Consumer.Offsets.AutoCommit.Enable = config.ConsumerConfig.EnableAutoCommit
    saramaConfig.Consumer.Offsets.AutoCommit.Interval = time.Duration(config.ConsumerConfig.AutoCommitIntervalMs) * time.Millisecond
    saramaConfig.Consumer.Group.Session.Timeout = time.Duration(config.ConsumerConfig.SessionTimeoutMs) * time.Millisecond
    saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin

    // 安全配置
    if config.SecurityProtocol != "PLAINTEXT" {
        saramaConfig.Net.SASL.Enable = true
        saramaConfig.Net.SASL.Mechanism = sarama.SASLMechanism(config.SASLMechanism)
        saramaConfig.Net.SASL.User = config.SASLUsername
        saramaConfig.Net.SASL.Password = config.SASLPassword
    }

    return saramaConfig
}

func (p *kafkaProducer) watchConfigChanges(ctx context.Context) {
    if p.coord == nil {
        return
    }

    configPath := "/config/mq/"
    watcher, err := p.coord.Config().WatchPrefix(ctx, configPath, &p.config)
    if err != nil {
        p.logger.Error("监听 MQ 配置失败", clog.Err(err))
        return
    }

    go func() {
        for range watcher.Changes() {
            p.updateConfig()
        }
    }()
}
```

### 3.4 监控和统计

```go
func (p *kafkaProducer) startMetricsCollection(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            p.reportMetrics()
        }
    }
}

func (p *kafkaProducer) reportMetrics() {
    p.statsMu.RLock()
    defer p.statsMu.RUnlock()

    if p.metrics != nil {
        p.metrics.Gauge("mq.producer.messages_sent", float64(p.stats.MessagesSent))
        p.metrics.Gauge("mq.producer.bytes_sent", float64(p.stats.BytesSent))
        p.metrics.Gauge("mq.producer.send_errors", float64(p.stats.SendErrors))
        p.metrics.Gauge("mq.producer.retry_count", float64(p.stats.RetryCount))
        p.metrics.Gauge("mq.producer.average_latency_ms", float64(p.stats.AverageLatency.Milliseconds()))
        p.metrics.Gauge("mq.producer.queue_size", float64(p.stats.CurrentQueueSize))
    }
}
```

## 4. 标准用法示例

### 4.1 基础初始化

```go
func main() {
    ctx := context.Background()

    // 1. 初始化 MQ 组件
    config := mq.GetDefaultConfig("production")
    config.Brokers = []string{"kafka1:9092", "kafka2:9092", "kafka3:9092"}
    config.SecurityProtocol = "SASL_SSL"

    // 生产者配置
    config.ProducerConfig = &mq.ProducerConfig{
        Acks:        -1,   // 等待所有副本确认
        RetryMax:    10,   // 最大重试次数
        BatchSize:   65536, // 批处理大小
        LingerMs:    10,   // 延迟发送时间
        Compression: 2,    // 使用 gzip 压缩
    }

    // 消费者配置
    config.ConsumerConfig = &mq.ConsumerConfig{
        AutoOffsetReset:      "earliest",
        EnableAutoCommit:     false,
        AutoCommitIntervalMs: 5000,
        SessionTimeoutMs:     30000,
        MaxPollRecords:       500,
        MaxPollIntervalMs:    300000,
    }

    // 创建生产者
    producer, err := mq.NewProducer(ctx, config,
        mq.WithLogger(clog.Namespace("mq-producer")),
        mq.WithCoordProvider(coordProvider),
        mq.WithMetricsProvider(metricsProvider),
    )
    if err != nil {
        log.Fatal("创建 MQ 生产者失败:", err)
    }
    defer producer.Close()

    // 创建消费者
    consumer, err := mq.NewConsumer(ctx, config, "user-service-group",
        mq.WithLogger(clog.Namespace("mq-consumer")),
    )
    if err != nil {
        log.Fatal("创建 MQ 消费者失败:", err)
    }
    defer consumer.Close()
}
```

### 4.2 发送消息

```go
type UserService struct {
    producer mq.Producer
}

func (s *UserService) UserRegistered(ctx context.Context, user *User) error {
    logger := clog.WithContext(ctx)

    // 构造事件消息
    event := &UserRegisteredEvent{
        UserID:    user.ID,
        Username:  user.Username,
        Email:     user.Email,
        Timestamp: time.Now(),
    }

    eventData, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("序列化事件失败: %w", err)
    }

    message := &mq.Message{
        Topic: "user-events.registered",
        Key:   []byte(user.ID),
        Value: eventData,
        Headers: map[string][]byte{
            "Event-Type": []byte("UserRegistered"),
            "Version":    []byte("1.0"),
        },
    }

    // 异步发送消息
    err = s.producer.Send(ctx, message, func(ack *mq.MessageAck) {
        if ack.Error != nil {
            logger.Error("发送用户注册事件失败",
                clog.String("user_id", user.ID),
                clog.Err(ack.Error))
        } else {
            logger.Info("用户注册事件发送成功",
                clog.String("user_id", user.ID),
                clog.Int32("partition", ack.Partition),
                clog.Int64("offset", ack.Offset))
        }
    })

    if err != nil {
        return fmt.Errorf("发送消息失败: %w", err)
    }

    return nil
}
```

### 4.3 接收消息

```go
type NotificationService struct {
    consumer mq.Consumer
}

func (s *NotificationService) Start(ctx context.Context) error {
    logger := clog.WithContext(ctx)

    // 订阅用户事件主题
    topics := []string{
        "user-events.registered",
        "user-events.updated",
        "user-events.deleted",
    }

    handler := func(ctx context.Context, msg *mq.Message) error {
        // 处理消息
        return s.handleUserEvent(ctx, msg)
    }

    // 启动消费者
    if err := s.consumer.Subscribe(ctx, topics, handler); err != nil {
        return fmt.Errorf("订阅主题失败: %w", err)
    }

    logger.Info("消息消费者启动成功", clog.Strings("topics", topics))
    return nil
}

func (s *NotificationService) handleUserEvent(ctx context.Context, msg *mq.Message) error {
    logger := clog.WithContext(ctx)

    // 根据主题处理不同事件
    switch msg.Topic {
    case "user-events.registered":
        return s.handleUserRegistered(ctx, msg)
    case "user-events.updated":
        return s.handleUserUpdated(ctx, msg)
    case "user-events.deleted":
        return s.handleUserDeleted(ctx, msg)
    default:
        logger.Warn("未知的用户事件类型", clog.String("topic", msg.Topic))
        return nil
    }
}

func (s *NotificationService) handleUserRegistered(ctx context.Context, msg *mq.Message) error {
    logger := clog.WithContext(ctx)

    var event UserRegisteredEvent
    if err := json.Unmarshal(msg.Value, &event); err != nil {
        logger.Error("解析用户注册事件失败", clog.Err(err))
        return err
    }

    logger.Info("处理用户注册事件",
        clog.String("user_id", event.UserID),
        clog.String("username", event.Username))

    // 发送欢迎邮件
    if err := s.sendWelcomeEmail(ctx, event); err != nil {
        logger.Error("发送欢迎邮件失败",
            clog.String("user_id", event.UserID),
            clog.Err(err))
        return err
    }

    // 提交偏移量
    if err := s.consumer.Commit(ctx); err != nil {
        logger.Error("提交偏移量失败", clog.Err(err))
        return err
    }

    return nil
}
```

### 4.4 批量发送

```go
func (s *MessageService) BatchSendMessages(ctx context.Context, messages []*Message) error {
    logger := clog.WithContext(ctx)

    // 批量发送消息
    err := s.producer.SendBatch(ctx, messages, func(acks []*mq.MessageAck) {
        successCount := 0
        errorCount := 0

        for _, ack := range acks {
            if ack.Error != nil {
                errorCount++
                logger.Error("消息发送失败",
                    clog.String("topic", ack.Topic),
                    clog.Err(ack.Error))
            } else {
                successCount++
            }
        }

        logger.Info("批量消息发送完成",
            clog.Int("total", len(messages)),
            clog.Int("success", successCount),
            clog.Int("errors", errorCount))
    })

    if err != nil {
        return fmt.Errorf("批量发送消息失败: %w", err)
    }

    return nil
}
```

## 5. 高级特性

### 5.1 事务消息

```go
func (s *OrderService) CreateOrderWithTransaction(ctx context.Context, order *Order) error {
    // 在数据库事务中发送消息
    err := s.db.Transaction(ctx, func(tx *gorm.DB) error {
        // 1. 创建订单
        if err := tx.Create(order).Error; err != nil {
            return fmt.Errorf("创建订单失败: %w", err)
        }

        // 2. 发送订单创建事件
        event := &OrderCreatedEvent{
            OrderID:  order.ID,
            UserID:   order.UserID,
            Amount:   order.Amount,
            Status:   order.Status,
        }

        eventData, _ := json.Marshal(event)
        message := &mq.Message{
            Topic: "order-events.created",
            Key:   []byte(order.ID),
            Value: eventData,
        }

        // 使用同步发送确保消息发送成功
        ack, err := s.producer.SendSync(ctx, message)
        if err != nil {
            return fmt.Errorf("发送订单事件失败: %w", err)
        }

        // 记录消息偏移量
        if err := tx.Create(&MessageLog{
            OrderID:   order.ID,
            Topic:     ack.Topic,
            Partition: ack.Partition,
            Offset:    ack.Offset,
            Status:    "sent",
        }).Error; err != nil {
            return fmt.Errorf("记录消息日志失败: %w", err)
        }

        return nil
    })

    return err
}
```

### 5.2 消息重试和死信队列

```go
func (s *PaymentService) handlePaymentMessage(ctx context.Context, msg *mq.Message) error {
    logger := clog.WithContext(ctx)

    var payment Payment
    if err := json.Unmarshal(msg.Value, &payment); err != nil {
        return fmt.Errorf("解析支付消息失败: %w", err)
    }

    // 获取重试次数
    retryCount := getRetryCountFromHeaders(msg.Headers)

    // 处理支付
    err := s.processPayment(ctx, &payment)
    if err != nil {
        logger.Error("支付处理失败",
            clog.String("payment_id", payment.ID),
            clog.Int("retry_count", retryCount),
            clog.Err(err))

        // 如果重试次数超过限制，发送到死信队列
        if retryCount >= 3 {
            return s.sendToDeadLetterQueue(ctx, msg, err)
        }

        // 增加重试次数并重新发送
        return s.retryMessage(ctx, msg, retryCount+1)
    }

    logger.Info("支付处理成功", clog.String("payment_id", payment.ID))
    return nil
}

func (s *PaymentService) retryMessage(ctx context.Context, msg *mq.Message, retryCount int) error {
    // 更新重试次数
    if msg.Headers == nil {
        msg.Headers = make(map[string][]byte)
    }
    msg.Headers["Retry-Count"] = []byte(strconv.Itoa(retryCount))

    // 延迟重试
    time.AfterFunc(time.Duration(retryCount)*time.Second, func() {
        ack, err := s.producer.SendSync(ctx, msg)
        if err != nil {
            s.logger.Error("重试消息发送失败", clog.Err(err))
        } else {
            s.logger.Info("重试消息发送成功",
                clog.String("topic", ack.Topic),
                clog.Int32("partition", ack.Partition))
        }
    })

    return nil
}
```

### 5.3 消费者组管理

```go
func (s *ConsumerManager) Rebalance(ctx context.Context) error {
    // 获取当前分区分配
    assignments, err := s.getCurrentAssignments(ctx)
    if err != nil {
        return fmt.Errorf("获取分区分配失败: %w", err)
    }

    // 计算新的分区分配
    newAssignments := s.calculateOptimalAssignments(assignments)

    // 应用新的分配
    if err := s.applyAssignments(ctx, newAssignments); err != nil {
        return fmt.Errorf("应用分区分配失败: %w", err)
    }

    // 更新消费者组状态
    s.updateConsumerGroupStatus(newAssignments)

    return nil
}

func (s *ConsumerManager) monitorConsumerLag(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.checkConsumerLag(ctx)
        }
    }
}

func (s *ConsumerManager) checkConsumerLag(ctx context.Context) {
    // 获取所有消费者组
    groups, err := s.getConsumerGroups(ctx)
    if err != nil {
        s.logger.Error("获取消费者组失败", clog.Err(err))
        return
    }

    // 检查每个组的消费延迟
    for _, group := range groups {
        lag, err := s.getConsumerLag(ctx, group)
        if err != nil {
            s.logger.Error("获取消费延迟失败",
                clog.String("group", group),
                clog.Err(err))
            continue
        }

        // 如果延迟超过阈值，发出告警
        if lag > s.config.LagThreshold {
            s.logger.Warn("消费延迟过高",
                clog.String("group", group),
                clog.Int64("lag", lag))

            // 触发自动扩容
            s.triggerAutoScaling(ctx, group, lag)
        }
    }
}
```

## 6. 最佳实践

### 6.1 消息设计

- **消息格式**: 使用 JSON 或 Protobuf 进行序列化
- **消息大小**: 控制单个消息大小，避免过大消息
- **消息版本**: 在消息头中包含版本信息，便于兼容性处理
- **消息标识**: 使用有意义的键值，便于分区路由

### 6.2 性能优化

- **批量发送**: 使用批量发送减少网络开销
- **压缩**: 启用消息压缩减少传输数据量
- **连接池**: 合理配置连接池大小
- **异步处理**: 使用异步发送提高吞吐量

### 6.3 可靠性保证

- **确认级别**: 根据业务需求选择合适的确认级别
- **重试机制**: 实现合理的重试策略
- **死信队列**: 处理无法正常消费的消息
- **监控告警**: 监控消息队列的关键指标

### 6.4 运维管理

- **主题管理**: 合理设计主题数量和分区数
- **消费者组**: 合理设置消费者组，避免重复消费
- **配置管理**: 使用配置中心管理 Kafka 配置
- **监控指标**: 监控生产者、消费者的关键指标

---

*遵循这些指南可以确保消息队列组件的高质量实现和稳定运行。*
