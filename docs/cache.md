# 分布式缓存组件实现指南

## 1. 设计理念

`cache` 组件是一个高性能的分布式缓存组件，基于 Redis 实现。它旨在为微服务架构提供统一、可靠、易用的缓存解决方案。

### 核心设计原则

- **封装但不隐藏**: 将 Redis 的复杂性封装在简洁的 API 后面
- **类型安全**: 提供强类型的操作接口，避免运行时错误
- **功能丰富**: 支持字符串、哈希、集合、有序集合等多种数据结构
- **高可用性**: 内置分布式锁和连接池管理
- **易于测试**: 提供清晰的接口和测试工具

### 组件价值

- **性能提升**: 通过缓存减少数据库访问压力
- **可扩展性**: 支持集群部署和水平扩展
- **一致性**: 提供分布式锁保证数据一致性
- **可观测性**: 集成日志和监控支持

## 2. 核心 API 契约

### 2.1 构造函数与配置

```go
// Config 组件配置结构
type Config struct {
    Addr            string        `json:"addr"`            // Redis 服务器地址
    Password        string        `json:"password"`        // 认证密码
    DB              int           `json:"db"`              // 数据库编号
    PoolSize        int           `json:"poolSize"`        // 连接池大小
    DialTimeout     time.Duration `json:"dialTimeout"`     // 连接超时
    ReadTimeout     time.Duration `json:"readTimeout"`     // 读取超时
    WriteTimeout    time.Duration `json:"writeTimeout"`    // 写入超时
    KeyPrefix       string        `json:"keyPrefix"`       // Key 前缀
    MinIdleConns    int           `json:"minIdleConns"`    // 最小空闲连接
    MaxRetries      int           `json:"maxRetries"`      // 最大重试次数
}

// GetDefaultConfig 返回环境相关默认配置
func GetDefaultConfig(env string) *Config

// Option 功能选项
type Option func(*options)

// WithLogger 注入日志依赖
func WithLogger(logger clog.Logger) Option

// WithCoordProvider 注入配置中心依赖
func WithCoordProvider(coord coord.Provider) Option

// New 创建缓存组件实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
```

### 2.2 Provider 接口设计

```go
// Provider 缓存组件主接口
type Provider interface {
    String() StringOperations      // 字符串操作
    Hash() HashOperations          // 哈希操作
    Set() SetOperations            // 集合操作
    ZSet() ZSetOperations          // 有序集合操作
    Lock() LockOperations          // 分布式锁
    Bloom() BloomFilterOperations  // 布隆过滤器
    Script() ScriptingOperations   // 脚本操作
    
    Ping(ctx context.Context) error   // 连接检查
    Close() error                    // 资源清理
}

// 标准错误定义
var ErrCacheMiss = errors.New("cache: key not found")
```

### 2.3 功能子接口

#### 字符串操作
```go
type StringOperations interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
    Del(ctx context.Context, keys ...string) error
    Incr(ctx context.Context, key string) (int64, error)
    Decr(ctx context.Context, key string) (int64, error)
    Exists(ctx context.Context, keys ...string) (int64, error)
    SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
    GetSet(ctx context.Context, key string, value interface{}) (string, error)
}
```

#### 哈希操作
```go
type HashOperations interface {
    HGet(ctx context.Context, key, field string) (string, error)
    HSet(ctx context.Context, key, field string, value interface{}) error
    HGetAll(ctx context.Context, key string) (map[string]string, error)
    HDel(ctx context.Context, key string, fields ...string) error
    HExists(ctx context.Context, key, field string) (bool, error)
    HLen(ctx context.Context, key string) (int64, error)
}
```

#### 有序集合操作
```go
type ZMember struct {
    Member interface{}
    Score  float64
}

type ZSetOperations interface {
    ZAdd(ctx context.Context, key string, members ...*ZMember) error
    ZRange(ctx context.Context, key string, start, stop int64) ([]*ZMember, error)
    ZRevRange(ctx context.Context, key string, start, stop int64) ([]*ZMember, error)
    ZRangeByScore(ctx context.Context, key string, min, max float64) ([]*ZMember, error)
    ZRem(ctx context.Context, key string, members ...interface{}) error
    ZCard(ctx context.Context, key string) (int64, error)
    ZCount(ctx context.Context, key string, min, max float64) (int64, error)
}
```

#### 分布式锁操作
```go
type LockOperations interface {
    Acquire(ctx context.Context, key string, expiration time.Duration) (Locker, error)
}

type Locker interface {
    Unlock(ctx context.Context) error
    Refresh(ctx context.Context, expiration time.Duration) error
}
```

## 3. 实现要点

### 3.1 连接池管理

```go
type redisProvider struct {
    client   *redis.Client
    config   *Config
    logger   clog.Logger
    coord    coord.Provider
}

func (p *redisProvider) createClient(config *Config) *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr:         config.Addr,
        Password:     config.Password,
        DB:           config.DB,
        PoolSize:     config.PoolSize,
        MinIdleConns: config.MinIdleConns,
        DialTimeout:  config.DialTimeout,
        ReadTimeout:  config.ReadTimeout,
        WriteTimeout: config.WriteTimeout,
        MaxRetries:   config.MaxRetries,
    })
}
```

### 3.2 Key 前缀处理

```go
func (p *redisProvider) addPrefix(key string) string {
    if p.config.KeyPrefix == "" {
        return key
    }
    return p.config.KeyPrefix + key
}

func (p *redisProvider) addPrefixes(keys []string) []string {
    if p.config.KeyPrefix == "" {
        return keys
    }
    
    result := make([]string, len(keys))
    for i, key := range keys {
        result[i] = p.config.KeyPrefix + key
    }
    return result
}
```

### 3.3 分布式锁实现

```go
type redisLocker struct {
    client   *redis.Client
    key      string
    value    string
    logger   clog.Logger
}

func (p *redisProvider) Acquire(ctx context.Context, key string, expiration time.Duration) (Locker, error) {
    fullKey := p.addPrefix(key)
    value := uuid.New().String()
    
    // 使用 SETNX 命令原子性地获取锁
    acquired, err := p.client.SetNX(ctx, fullKey, value, expiration).Result()
    if err != nil {
        return nil, fmt.Errorf("获取锁失败: %w", err)
    }
    
    if !acquired {
        return nil, fmt.Errorf("锁已被占用")
    }
    
    return &redisLocker{
        client: p.client,
        key:    fullKey,
        value:  value,
        logger: p.logger,
    }, nil
}

func (l *redisLocker) Unlock(ctx context.Context) error {
    // 使用 Lua 脚本确保只有锁的持有者才能释放
    script := `
        if redis.call("GET", KEYS[1]) == ARGV[1] then
            return redis.call("DEL", KEYS[1])
        else
            return 0
        end
    `
    
    result, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
    if err != nil {
        return fmt.Errorf("释放锁失败: %w", err)
    }
    
    if result.(int64) == 0 {
        return fmt.Errorf("锁已过期或被其他进程持有")
    }
    
    return nil
}
```

### 3.4 动态配置支持

```go
func (p *redisProvider) watchConfigChanges(ctx context.Context) {
    if p.coord == nil {
        return
    }
    
    configPath := "/config/cache/"
    watcher, err := p.coord.Config().WatchPrefix(ctx, configPath, &p.config)
    if err != nil {
        p.logger.Error("监听缓存配置失败", clog.Err(err))
        return
    }
    
    go func() {
        for range watcher.Changes() {
            p.updateConnection()
        }
    }()
}

func (p *redisProvider) updateConnection() {
    // 重新创建连接
    newClient := p.createClient(p.config)
    
    // 优雅关闭旧连接
    if p.client != nil {
        p.client.Close()
    }
    
    p.client = newClient
    p.logger.Info("缓存配置已更新，重新建立连接")
}
```

## 4. 标准用法示例

### 4.1 基础初始化

```go
func main() {
    ctx := context.Background()
    
    // 1. 初始化日志
    clog.Init(ctx, clog.GetDefaultConfig("production"), 
        clog.WithNamespace("cache-service"))
    
    // 2. 创建缓存组件
    config := cache.GetDefaultConfig("production")
    config.Addr = "redis:6379"
    config.KeyPrefix = "my-service:"
    
    cacheProvider, err := cache.New(ctx, config,
        cache.WithLogger(clog.Namespace("cache")),
    )
    if err != nil {
        clog.Fatal("缓存初始化失败", clog.Err(err))
    }
    defer cacheProvider.Close()
    
    // 3. 验证连接
    if err := cacheProvider.Ping(ctx); err != nil {
        clog.Fatal("Redis 连接失败", clog.Err(err))
    }
}
```

### 4.2 缓存业务数据

```go
type UserService struct {
    cache cache.Provider
    db    db.Provider
}

func (s *UserService) GetUserProfile(ctx context.Context, userID string) (*Profile, error) {
    logger := clog.WithContext(ctx)
    key := fmt.Sprintf("user:%s:profile", userID)
    
    // 尝试从缓存获取
    profileJSON, err := s.cache.String().Get(ctx, key)
    if err == nil {
        var profile Profile
        if json.Unmarshal([]byte(profileJSON), &profile) == nil {
            logger.Info("用户资料缓存命中", clog.String("user_id", userID))
            return &profile, nil
        }
    }
    
    // 缓存未命中，从数据库获取
    profile, err := s.getUserFromDB(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // 写入缓存
    profileData, _ := json.Marshal(profile)
    if err := s.cache.String().Set(ctx, key, profileData, time.Hour); err != nil {
        logger.Error("写入缓存失败", clog.Err(err))
    }
    
    return profile, nil
}
```

### 4.3 分布式锁使用

```go
func (s *OrderService) ProcessOrder(ctx context.Context, orderID string) error {
    logger := clog.WithContext(ctx)
    
    // 获取分布式锁，防止重复处理
    lock, err := s.cache.Lock().Acquire(ctx, fmt.Sprintf("lock:order:%s", orderID), 30*time.Second)
    if err != nil {
        return fmt.Errorf("获取订单处理锁失败: %w", err)
    }
    defer lock.Unlock(ctx)
    
    logger.Info("开始处理订单", clog.String("order_id", orderID))
    
    // 处理订单逻辑
    if err := s.processOrderLogic(ctx, orderID); err != nil {
        return fmt.Errorf("处理订单失败: %w", err)
    }
    
    logger.Info("订单处理完成", clog.String("order_id", orderID))
    return nil
}
```

### 4.4 有序集合使用

```go
func (s *LeaderboardService) UpdateScore(ctx context.Context, userID string, score float64) error {
    key := "leaderboard:global"
    member := &cache.ZMember{
        Member: userID,
        Score:  score,
    }
    
    // 更新用户分数
    if err := s.cache.ZSet().ZAdd(ctx, key, member); err != nil {
        return fmt.Errorf("更新排行榜分数失败: %w", err)
    }
    
    // 保持排行榜前100名
    count, err := s.cache.ZSet().ZCard(ctx, key)
    if err != nil {
        return fmt.Errorf("获取排行榜数量失败: %w", err)
    }
    
    if count > 100 {
        if err := s.cache.ZSet().ZRemRangeByRank(ctx, key, 0, count-101); err != nil {
            return fmt.Errorf("清理排行榜失败: %w", err)
        }
    }
    
    return nil
}
```

## 5. 测试策略

### 5.1 单元测试

```go
func TestCacheStringOperations(t *testing.T) {
    // 创建测试用的 Redis 容器
    ctx := context.Background()
    config := &cache.Config{
        Addr:     "localhost:6379",
        KeyPrefix: "test:",
    }
    
    provider, err := cache.New(ctx, config)
    assert.NoError(t, err)
    defer provider.Close()
    
    // 测试字符串操作
    err = provider.String().Set(ctx, "test_key", "test_value", time.Hour)
    assert.NoError(t, err)
    
    value, err := provider.String().Get(ctx, "test_key")
    assert.NoError(t, err)
    assert.Equal(t, "test_value", value)
}
```

### 5.2 集成测试

```go
func TestCacheWithCoord(t *testing.T) {
    // 模拟配置中心
    coord := mock.NewCoordProvider()
    
    // 创建带配置中心的缓存组件
    config := cache.GetDefaultConfig("test")
    provider, err := cache.New(context.Background(), config,
        cache.WithCoordProvider(coord),
    )
    assert.NoError(t, err)
    
    // 测试动态配置更新
    coord.UpdateConfig("/config/cache/pool_size", "50")
    
    // 验证配置更新效果
    // ...
}
```

## 6. 性能优化

### 6.1 连接池优化

```go
// 根据应用规模调整连接池大小
func getOptimalPoolSize() int {
    cpuCount := runtime.NumCPU()
    
    // 开发环境
    if os.Getenv("GO_ENV") == "development" {
        return cpuCount * 2
    }
    
    // 生产环境
    return cpuCount * 10
}
```

### 6.2 批量操作优化

```go
func (s *UserService) BatchGetUserProfiles(ctx context.Context, userIDs []string) ([]*Profile, error) {
    logger := clog.WithContext(ctx)
    
    // 使用管道批量获取
    pipe := s.cache.String().(*redisStringOperations).client.Pipeline()
    
    cmds := make([]*redis.StringCmd, len(userIDs))
    for i, userID := range userIDs {
        key := fmt.Sprintf("user:%s:profile", userID)
        cmds[i] = pipe.Get(ctx, key)
    }
    
    // 执行批量操作
    _, err := pipe.Exec(ctx)
    if err != nil {
        logger.Error("批量获取用户资料失败", clog.Err(err))
    }
    
    // 处理结果
    var profiles []*Profile
    var missingUserIDs []string
    
    for i, cmd := range cmds {
        profileJSON, err := cmd.Result()
        if err == redis.Nil {
            missingUserIDs = append(missingUserIDs, userIDs[i])
            continue
        }
        
        if err != nil {
            logger.Error("获取用户资料失败", clog.String("user_id", userIDs[i]), clog.Err(err))
            continue
        }
        
        var profile Profile
        if json.Unmarshal([]byte(profileJSON), &profile) == nil {
            profiles = append(profiles, &profile)
        }
    }
    
    // 批量获取缺失的用户资料
    if len(missingUserIDs) > 0 {
        missingProfiles, err := s.batchGetUsersFromDB(ctx, missingUserIDs)
        if err == nil {
            profiles = append(profiles, missingProfiles...)
        }
    }
    
    return profiles, nil
}
```

### 6.3 错误处理优化

```go
func (p *redisProvider) executeWithRetry(ctx context.Context, op func() error) error {
    const maxRetries = 3
    const retryDelay = 100 * time.Millisecond
    
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        err := op()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // 如果是网络错误，进行重试
        if isNetworkError(err) && i < maxRetries-1 {
            time.Sleep(retryDelay)
            continue
        }
        
        // 其他错误直接返回
        return err
    }
    
    return lastErr
}
```

## 7. 监控与维护

### 7.1 关键指标监控

```go
func (p *redisProvider) collectMetrics(ctx context.Context) {
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

func (p *redisProvider) reportMetrics() {
    // 获取连接池状态
    poolStats := p.client.PoolStats()
    
    // 上报到监控系统
    metrics.Gauge("cache.pool.size", poolStats.TotalConns)
    metrics.Gauge("cache.pool.idle", poolStats.IdleConns)
    metrics.Gauge("cache.pool.active", poolStats.TotalConns-poolStats.IdleConns)
    
    // 获取 Redis 服务器状态
    info, err := p.client.Info(context.Background()).Result()
    if err != nil {
        p.logger.Error("获取 Redis 信息失败", clog.Err(err))
        return
    }
    
    // 解析并上报关键指标
    // ...
}
```

### 7.2 健康检查

```go
func (p *redisProvider) HealthCheck(ctx context.Context) error {
    // 检查连接状态
    if err := p.Ping(ctx); err != nil {
        return fmt.Errorf("Redis 连接失败: %w", err)
    }
    
    // 检查连接池状态
    poolStats := p.client.PoolStats()
    if poolStats.TotalConns == 0 {
        return fmt.Errorf("连接池中没有可用连接")
    }
    
    // 检查内存使用情况
    info, err := p.client.Info(ctx, "memory").Result()
    if err != nil {
        return fmt.Errorf("获取 Redis 内存信息失败: %w", err)
    }
    
    // 解析内存使用情况
    // ...
    
    return nil
}
```

## 8. 最佳实践

### 8.1 缓存策略

- **缓存穿透**: 对不存在的数据也设置缓存，设置较短的过期时间
- **缓存击穿**: 使用分布式锁保护热点数据
- **缓存雪崩**: 设置不同的过期时间，避免同时失效
- **缓存更新**: 采用先更新数据库，再删除缓存的策略

### 8.2 性能优化

- **批量操作**: 尽可能使用管道和批量操作
- **连接池**: 合理配置连接池大小
- **序列化**: 选择高效的序列化方式
- **内存管理**: 定期清理过期数据

### 8.3 错误处理

- **降级策略**: 缓存失败时降级到数据库
- **重试机制**: 对网络错误进行适当重试
- **日志记录**: 记录关键操作和错误信息
- **监控告警**: 设置关键指标的监控和告警

---

*遵循这些指南可以确保缓存组件的高质量实现和稳定运行。*