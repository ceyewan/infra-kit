# 基础设施: coord 分布式协调

## 1. 设计理念

`coord` 是 `infra-kit` 项目的分布式协调核心，基于 `etcd` 构建。它为整个微服务集群提供了一个统一的、可靠的协调层，封装了服务发现、配置中心、分布式锁等复杂性。

### 核心设计原则

- **统一接口**: 通过 Provider 模式提供一致的协调服务体验，所有功能都通过统一入口访问
- **组件自治**: 内部自动监听配置变更，实现配置的热更新，无需外部干预
- **高可用性**: 基于 etcd 的强一致性保证，提供可靠的分布式协调服务
- **类型安全**: 使用泛型提供类型安全的配置监听，避免运行时错误
- **上下文感知**: 所有操作都接受 `context.Context`，支持链路追踪和超时控制

### 组件价值

- **服务治理**: 提供服务注册与发现，支持动态服务实例管理
- **配置中心**: 统一的配置管理，支持动态配置热更新
- **分布式锁**: 提供可靠的分布式锁机制，保证操作的原子性
- **实例管理**: 自动分配和管理服务实例 ID，支持故障自动回收

## 2. 核心 API 契约

### 2.1 构造函数与配置

```go
// Config 是 coord 组件的配置结构体
type Config struct {
    Endpoints        []string        `json:"endpoints"`         // etcd 集群地址列表
    DialTimeout      time.Duration   `json:"dialTimeout"`      // 连接超时
    KeepAliveTime    time.Duration   `json:"keepAliveTime"`    // 保活心跳间隔
    KeepAliveTimeout time.Duration   `json:"keepAliveTimeout"` // 保活超时时间
    Username         string          `json:"username,omitempty"` // 认证用户名
    Password         string          `json:"password,omitempty"` // 认证密码
    TLS              *TLSConfig      `json:"tls,omitempty"`     // TLS 配置
    SyncInterval     time.Duration   `json:"syncInterval"`     // 配置同步间隔
    CacheTTL         time.Duration   `json:"cacheTTL"`         // 配置缓存时间
}

// TLSConfig 定义 TLS 连接配置
type TLSConfig struct {
    CertFile string `json:"certFile,omitempty"`
    KeyFile  string `json:"keyFile,omitempty"`
    CAFile   string `json:"caFile,omitempty"`
}

// GetDefaultConfig 返回环境相关的默认配置
func GetDefaultConfig(env string) *Config

// Option 功能选项
type Option func(*options)

// WithLogger 注入日志依赖
func WithLogger(logger clog.Logger) Option

// New 创建 coord Provider 实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
```

### 2.2 Provider 接口设计

```go
// Provider 是 coord 组件的主接口
type Provider interface {
    // Registry 返回服务注册与发现客户端
    Registry() ServiceRegistry
    // Config 返回配置中心客户端
    Config() ConfigCenter
    // Lock 返回分布式锁客户端
    Lock() DistributedLock
    // InstanceIDAllocator 获取实例ID分配器
    InstanceIDAllocator(serviceName string, maxID int) (InstanceIDAllocator, error)
    // Close 关闭所有连接和资源
    Close() error
}

// ServiceInfo 定义服务信息
type ServiceInfo struct {
    ID        string            `json:"id"`         // 服务实例唯一标识
    Name      string            `json:"name"`       // 服务名称
    Address   string            `json:"address"`    // 服务地址
    Metadata  map[string]string `json:"metadata"`   // 服务元数据
    Version   string            `json:"version"`    // 服务版本
    StartedAt time.Time         `json:"startedAt"`  // 启动时间
}

// ServiceEvent 定义服务事件
type ServiceEvent struct {
    Type    EventType   `json:"type"`    // 事件类型: REGISTER, UNREGISTER
    Service ServiceInfo `json:"service"` // 服务信息
}
```

### 2.3 服务注册与发现接口

```go
// ServiceRegistry 服务注册与发现接口
type ServiceRegistry interface {
    // Register 注册服务实例
    Register(ctx context.Context, service ServiceInfo, ttl time.Duration) error
    // Unregister 注销服务实例
    Unregister(ctx context.Context, serviceID string) error
    // Discover 发现服务实例
    Discover(ctx context.Context, serviceName string) ([]ServiceInfo, error)
    // Watch 监听服务变更
    Watch(ctx context.Context, serviceName string) (<-chan ServiceEvent, error)
    // GetConnection 获取服务连接
    GetConnection(ctx context.Context, serviceName string) (*grpc.ClientConn, error)
}
```

### 2.4 配置中心接口

```go
// ConfigCenter 配置中心接口
type ConfigCenter interface {
    // Get 获取配置值
    Get(ctx context.Context, key string, value interface{}) error
    // List 列出指定前缀的配置键
    List(ctx context.Context, prefix string) ([]string, error)
    // Set 设置配置值
    Set(ctx context.Context, key string, value interface{}) error
    // Delete 删除配置键
    Delete(ctx context.Context, key string) error

    // Watch 监听单个配置变更
    Watch(ctx context.Context, key string, value interface{}) (Watcher[any], error)
    // WatchPrefix 监听指定前缀的配置变更
    WatchPrefix(ctx context.Context, prefix string, value interface{}) (Watcher[any], error)

    // GetWithVersion 获取配置值和版本号
    GetWithVersion(ctx context.Context, key string, value interface{}) (int64, error)
    // CompareAndSet 原子比较并设置配置值
    CompareAndSet(ctx context.Context, key string, value interface{}, expectedVersion int64) error
}

// Watcher 配置监听器
type Watcher[T any] interface {
    // Chan 返回配置事件通道
    Chan() <-chan ConfigEvent[T]
    // Close 关闭监听器
    Close()
}

// ConfigEvent 配置事件
type ConfigEvent[T any] struct {
    Type  EventType `json:"type"`  // 事件类型: PUT, DELETE
    Key   string    `json:"key"`   // 配置键
    Value T         `json:"value"` // 配置值
}
```

### 2.5 分布式锁接口

```go
// DistributedLock 分布式锁接口
type DistributedLock interface {
    // Acquire 获取分布式锁
    Acquire(ctx context.Context, key string, ttl time.Duration) (Lock, error)
    // TryAcquire 尝试获取锁（非阻塞）
    TryAcquire(ctx context.Context, key string, ttl time.Duration) (Lock, error)
}

// Lock 锁实例
type Lock interface {
    // Unlock 释放锁
    Unlock(ctx context.Context) error
    // TTL 获取锁的剩余生存时间
    TTL(ctx context.Context) (time.Duration, error)
    // Refresh 刷新锁的生存时间
    Refresh(ctx context.Context, ttl time.Duration) error
    // Key 获取锁的键
    Key() string
}
```

### 2.6 实例ID分配器接口

```go
// InstanceIDAllocator 实例ID分配器
type InstanceIDAllocator interface {
    // AcquireID 获取一个唯一的实例ID
    AcquireID(ctx context.Context) (AllocatedID, error)
}

// AllocatedID 已分配的实例ID
type AllocatedID interface {
    // ID 返回实例ID值
    ID() int
    // Close 释放实例ID
    Close(ctx context.Context) error
}
```

## 3. 实现要点

### 3.1 连接管理与重试

```go
type coordProvider struct {
    client    *clientv3.Client
    config    *Config
    logger    clog.Logger

    // 服务注册相关
    session   *concurrency.Session
    leases    map[string]*clientv3.LeaseGrantResponse

    // 配置缓存
    configCache *sync.Map

    // 实例ID分配器
    allocators map[string]*instanceIDAllocator
}

func (p *coordProvider) createClient(config *Config) (*clientv3.Client, error) {
    clientConfig := clientv3.Config{
        Endpoints:            config.Endpoints,
        DialTimeout:          config.DialTimeout,
        DialKeepAliveTime:    config.KeepAliveTime,
        DialKeepAliveTimeout: config.KeepAliveTimeout,
        Username:             config.Username,
        Password:             config.Password,
    }

    // TLS 配置
    if config.TLS != nil {
        tlsConfig, err := createTLSConfig(config.TLS)
        if err != nil {
            return nil, fmt.Errorf("创建 TLS 配置失败: %w", err)
        }
        clientConfig.TLS = tlsConfig
    }

    return clientv3.New(clientConfig)
}
```

### 3.2 配置中心实现

```go
type configCenter struct {
    client   *clientv3.Client
    provider *coordProvider
    cache    *sync.Map
    watchers map[string][]*configWatcher
}

func (c *configCenter) WatchPrefix(ctx context.Context, prefix string, value interface{}) (Watcher[any], error) {
    watcher := &configWatcher{
        prefix:  prefix,
        value:   value,
        channel: make(chan ConfigEvent[any], 100),
        done:    make(chan struct{}),
    }

    // 启动监听协程
    go watcher.watch(ctx, c.client)

    // 注册监听器
    c.watchersMu.Lock()
    c.watchers[prefix] = append(c.watchers[prefix], watcher)
    c.watchersMu.Unlock()

    return watcher, nil
}

type configWatcher struct {
    prefix  string
    value   interface{}
    channel chan ConfigEvent[any]
    done    chan struct{}
}

func (w *configWatcher) watch(ctx context.Context, client *clientv3.Client) {
    watchCh := client.Watch(ctx, w.prefix, clientv3.WithPrefix())

    for {
        select {
        case <-ctx.Done():
            return
        case <-w.done:
            return
        case resp, ok := <-watchCh:
            if !ok {
                return
            }

            for _, event := range resp.Events {
                configEvent := w.convertEvent(event)
                if configEvent != nil {
                    w.channel <- *configEvent
                }
            }
        }
    }
}
```

### 3.3 分布式锁实现

```go
type distributedLock struct {
    client   *clientv3.Client
    session  *concurrency.Session
}

func (d *distributedLock) Acquire(ctx context.Context, key string, ttl time.Duration) (Lock, error) {
    // 创建会话（如果不存在）
    if d.session == nil {
        session, err := concurrency.NewSession(d.client,
            concurrency.WithTTL(int(ttl.Seconds())))
        if err != nil {
            return nil, fmt.Errorf("创建会话失败: %w", err)
        }
        d.session = session
    }

    // 创建互斥锁
    mutex := concurrency.NewMutex(d.session, key)

    // 尝试获取锁
    if err := mutex.Lock(ctx); err != nil {
        return nil, fmt.Errorf("获取锁失败: %w", err)
    }

    return &etcdLock{
        mutex:  mutex,
        key:    key,
        client: d.client,
    }, nil
}

type etcdLock struct {
    mutex  *concurrency.Mutex
    key    string
    client *clientv3.Client
}

func (l *etcdLock) Unlock(ctx context.Context) error {
    // 使用事务确保原子性
    txn := l.client.Txn(ctx)

    // 检查锁是否仍然存在
    cmp := clientv3.Compare(clientv3.ModRevision(l.key), ">", 0)

    // 删除锁
    del := clientv3.OpDelete(l.key)

    // 执行事务
    resp, err := txn.If(cmp).Then(del).Commit()
    if err != nil {
        return fmt.Errorf("释放锁失败: %w", err)
    }

    if !resp.Succeeded {
        return fmt.Errorf("锁不存在或已被其他客户端持有")
    }

    return nil
}
```

### 3.4 实例ID分配器实现

```go
type instanceIDAllocator struct {
    serviceName string
    maxID       int
    client      *clientv3.Client
    session     *concurrency.Session
    allocated   map[int]bool
    mu          sync.Mutex
}

func (a *instanceIDAllocator) AcquireID(ctx context.Context) (AllocatedID, error) {
    a.mu.Lock()
    defer a.mu.Unlock()

    // 创建会话（如果不存在）
    if a.session == nil {
        session, err := concurrency.NewSession(a.client)
        if err != nil {
            return nil, fmt.Errorf("创建会话失败: %w", err)
        }
        a.session = session
    }

    // 尝试分配ID
    for i := 0; i < a.maxID; i++ {
        if !a.allocated[i] {
            // 检查是否已被其他实例占用
            key := fmt.Sprintf("/instances/%s/%d", a.serviceName, i)
            resp, err := a.client.Grant(ctx, 30) // 30秒租约
            if err != nil {
                continue
            }

            // 尝试创建临时节点
            _, err = a.client.Put(ctx, key, "", clientv3.WithLease(resp.ID))
            if err == nil {
                a.allocated[i] = true
                return &allocatedID{
                    id:      i,
                    leaseID: resp.ID,
                    key:     key,
                    client:  a.client,
                    alloc:   a,
                }, nil
            }
        }
    }

    return nil, fmt.Errorf("无可用的实例ID")
}

type allocatedID struct {
    id      int
    leaseID clientv3.LeaseID
    key     string
    client  *clientv3.Client
    alloc   *instanceIDAllocator
}

func (a *allocatedID) Close(ctx context.Context) error {
    // 释放租约
    _, err := a.client.Revoke(ctx, a.leaseID)
    if err != nil {
        return fmt.Errorf("释放租约失败: %w", err)
    }

    // 从分配器中移除
    a.alloc.mu.Lock()
    a.alloc.allocated[a.id] = false
    a.alloc.mu.Unlock()

    return nil
}
```

## 4. 标准用法示例

### 4.1 基础初始化

```go
func main() {
    ctx := context.Background()

    // 1. 初始化协调组件
    config := coord.GetDefaultConfig("production")
    config.Endpoints = []string{"etcd1:2379", "etcd2:2379", "etcd3:2379"}

    coordProvider, err := coord.New(ctx, config,
        coord.WithLogger(clog.Namespace("coord")),
    )
    if err != nil {
        log.Fatal("coord 初始化失败:", err)
    }
    defer coordProvider.Close()
}
```

### 4.2 服务注册与发现

```go
func (s *UserService) Start(ctx context.Context) error {
    // 注册服务
    serviceInfo := coord.ServiceInfo{
        ID:       fmt.Sprintf("user-service-%s", uuid.New().String()),
        Name:     "user-service",
        Address:  "0.0.0.0:8080",
        Metadata: map[string]string{
            "version": "1.0.0",
            "region":  "cn-north-1",
        },
    }

    err := s.coord.Registry().Register(ctx, serviceInfo, 30*time.Second)
    if err != nil {
        return fmt.Errorf("服务注册失败: %w", err)
    }

    // 启动服务
    go s.startServer()

    // 监听服务变更
    events, err := s.coord.Registry().Watch(ctx, "payment-service")
    if err != nil {
        return fmt.Errorf("监听服务变更失败: %w", err)
    }

    go s.handleServiceEvents(events)

    return nil
}

func (s *PaymentService) CallUserService(ctx context.Context, userID string) (*User, error) {
    // 获取用户服务连接
    conn, err := s.coord.Registry().GetConnection(ctx, "user-service")
    if err != nil {
        return nil, fmt.Errorf("获取用户服务连接失败: %w", err)
    }

    client := pb.NewUserServiceClient(conn)
    return client.GetUser(ctx, &pb.GetUserRequest{UserId: userID})
}
```

### 4.3 配置中心使用

```go
func (s *ConfigService) WatchRateLimitRules(ctx context.Context) {
    // 监听限流规则配置
    watcher, err := s.coord.Config().WatchPrefix(ctx, "/config/ratelimit/", &[]RateLimitRule{})
    if err != nil {
        s.logger.Error("监听限流规则失败", clog.Err(err))
        return
    }
    defer watcher.Close()

    for event := range watcher.Chan() {
        switch event.Type {
        case coord.PUT:
            s.logger.Info("限流规则更新", clog.String("key", event.Key))
            s.updateRateLimitRules(event.Value.([]RateLimitRule))
        case coord.DELETE:
            s.logger.Info("限流规则删除", clog.String("key", event.Key))
            s.removeRateLimitRule(event.Key)
        }
    }
}

func (s *ConfigService) UpdateFeatureFlag(ctx context.Context, flagName string, enabled bool) error {
    key := fmt.Sprintf("/features/%s", flagName)

    // 获取当前版本
    current := &FeatureFlag{}
    version, err := s.coord.Config().GetWithVersion(ctx, key, current)
    if err != nil && !errors.Is(err, coord.ErrNotFound) {
        return fmt.Errorf("获取当前配置失败: %w", err)
    }

    // 更新配置
    newFlag := &FeatureFlag{
        Name:     flagName,
        Enabled:  enabled,
        UpdateAt: time.Now(),
    }

    return s.coord.Config().CompareAndSet(ctx, key, newFlag, version)
}
```

### 4.4 分布式锁使用

```go
func (s *OrderService) ProcessOrder(ctx context.Context, orderID string) error {
    // 获取分布式锁
    lock, err := s.coord.Lock().Acquire(ctx, fmt.Sprintf("lock:order:%s", orderID), 30*time.Second)
    if err != nil {
        return fmt.Errorf("获取订单处理锁失败: %w", err)
    }
    defer lock.Unlock(ctx)

    // 检查锁是否仍然有效
    if ttl, err := lock.TTL(ctx); err != nil {
        return fmt.Errorf("检查锁TTL失败: %w", err)
    } else if ttl <= 0 {
        return fmt.Errorf("锁已过期")
    }

    // 处理订单逻辑
    return s.processOrderLogic(ctx, orderID)
}
```

### 4.5 实例ID分配使用

```go
func (s *UserService) StartWorker(ctx context.Context) error {
    // 获取实例ID分配器
    allocator, err := s.coord.InstanceIDAllocator("user-service-worker", 64)
    if err != nil {
        return fmt.Errorf("获取实例ID分配器失败: %w", err)
    }

    // 分配实例ID
    instanceID, err := allocator.AcquireID(ctx)
    if err != nil {
        return fmt.Errorf("分配实例ID失败: %w", err)
    }
    defer instanceID.Close(ctx)

    s.instanceID = instanceID.ID()
    s.logger.Info("工作实例启动", clog.Int("instance_id", s.instanceID))

    // 启动工作协程
    go s.startWorkerLoop(ctx)

    return nil
}
```

## 5. 高级特性

### 5.1 配置缓存与同步

```go
func (c *configCenter) startConfigSync(ctx context.Context) {
    ticker := time.NewTicker(c.provider.config.SyncInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.syncConfigs(ctx)
        }
    }
}

func (c *configCenter) syncConfigs(ctx context.Context) {
    // 获取所有配置键
    keys, err := c.List(ctx, "/config/")
    if err != nil {
        c.provider.logger.Error("同步配置失败", clog.Err(err))
        return
    }

    // 同步每个配置
    for _, key := range keys {
        var value interface{}
        if err := c.Get(ctx, key, &value); err == nil {
            c.cache.Store(key, value)
        }
    }
}
```

### 5.2 健康检查与监控

```go
func (p *coordProvider) HealthCheck(ctx context.Context) error {
    // 检查 etcd 连接
    if err := p.client.Sync(ctx); err != nil {
        return fmt.Errorf("etcd 连接失败: %w", err)
    }

    // 检查会话状态
    if p.session != nil {
        if err := p.session.Orphan(); err != nil {
            return fmt.Errorf("会话异常: %w", err)
        }
    }

    // 检查租约状态
    for leaseID := range p.leases {
        ttl, err := p.client.TimeToLive(ctx, leaseID)
        if err != nil || ttl.TTL <= 0 {
            return fmt.Errorf("租约 %d 已失效", leaseID)
        }
    }

    return nil
}
```

## 6. 最佳实践

### 6.1 服务注册最佳实践

- **唯一标识**: 使用 UUID 或主机名+端口组合作为服务实例ID
- **元数据丰富**: 在服务信息中包含版本、区域、健康状态等元数据
- **定期续约**: 设置合理的 TTL 并定期续约，确保故障检测的及时性
- **优雅下线**: 服务关闭时主动注销，避免残留服务信息

### 6.2 配置管理最佳实践

- **层次化结构**: 使用层次化的配置键结构，如 `/config/{service}/{module}/`
- **版本控制**: 重要配置变更使用 CompareAndSet 确保原子性
- **缓存策略**: 合理设置配置缓存时间，减少 etcd 访问压力
- **变更监听**: 使用 WatchPrefix 监听配置变更，实现热更新

### 6.3 分布式锁最佳实践

- **锁粒度**: 选择合适的锁粒度，避免过大或过小的锁定范围
- **超时设置**: 设置合理的锁超时时间，避免死锁
- **异常处理**: 处理锁获取失败和锁过期的情况
- **资源清理**: 确保锁在使用后正确释放

### 6.4 性能优化

- **连接池**: 合理配置 etcd 连接池大小
- **批量操作**: 使用事务进行批量配置操作
- **缓存机制**: 实现配置缓存，减少 etcd 访问
- **监控告警**: 监控 etcd 性能指标，及时发现性能问题

---

*遵循这些指南可以确保分布式协调组件的高质量实现和稳定运行。*
