# 基础设施: db 数据库访问

## 1. 设计理念

`db` 是 `infra-kit` 项目的数据库访问组件，基于 `GORM v2` 构建。它是一个专注于 MySQL 的高性能数据库操作层，提供了分库分表、连接池管理、事务处理等核心功能。

### 核心设计原则

- **封装但不隐藏**: 封装 GORM 的复杂性，同时通过 `DB()` 方法提供对原生 `*gorm.DB` 的完全访问
- **分库分表支持**: 基于 `gorm.io/sharding` 实现自动分片，支持水平扩展
- **连接池优化**: 智能的连接池管理，支持高并发访问
- **事务安全**: 提供简洁的事务 API，确保数据一致性
- **监控集成**: 集成日志和监控，提供数据库操作的可观测性

### 组件价值

- **性能提升**: 通过连接池和分片技术，支持高并发和大数据量场景
- **开发效率**: 简化数据库操作，提供统一的 API 接口
- **可扩展性**: 分库分表支持，支持业务规模的线性扩展
- **运维友好**: 集成监控和日志，便于问题排查和性能优化

## 2. 核心 API 契约

### 2.1 构造函数与配置

```go
// Config 是 db 组件的配置结构体
type Config struct {
    DSN             string          `json:"dsn"`             // 数据库连接字符串
    Driver          string          `json:"driver"`          // 数据库驱动，仅支持 "mysql"
    MaxOpenConns    int             `json:"maxOpenConns"`    // 最大打开连接数
    MaxIdleConns    int             `json:"maxIdleConns"`    // 最大空闲连接数
    ConnMaxLifetime time.Duration   `json:"connMaxLifetime"` // 连接最大生命周期
    ConnMaxIdleTime time.Duration   `json:"connMaxIdleTime"` // 连接最大空闲时间
    LogLevel        string          `json:"logLevel"`        // GORM 日志级别
    SlowThreshold   time.Duration   `json:"slowThreshold"`   // 慢查询阈值
    Sharding        *ShardingConfig `json:"sharding"`        // 分片配置，可选
}

// ShardingConfig 分库分表配置
type ShardingConfig struct {
    ShardingKey    string                        `json:"shardingKey"`    // 分片键字段名
    NumberOfShards int                           `json:"numberOfShards"` // 分片总数
    Tables         map[string]*TableShardingConfig `json:"tables"`     // 表分片配置
}

// TableShardingConfig 单表分片配置
type TableShardingConfig struct {
    ShardingKey    string `json:"shardingKey,omitempty"`    // 覆盖全局分片键
    NumberOfShards int    `json:"numberOfShards,omitempty"` // 覆盖全局分片数
}

// GetDefaultConfig 返回环境相关的默认配置
func GetDefaultConfig(env string) *Config

// Option 功能选项
type Option func(*options)

// WithLogger 注入日志依赖
func WithLogger(logger clog.Logger) Option

// WithCoordProvider 注入配置中心依赖，用于动态配置管理
func WithCoordProvider(coord coord.Provider) Option

// WithMetricsProvider 注入监控依赖
func WithMetricsProvider(metrics metrics.Provider) Option

// New 创建 db Provider 实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
```

### 2.2 Provider 接口设计

```go
// Provider 是 db 组件的主接口
type Provider interface {
    // DB 获取带上下文的数据库实例
    DB(ctx context.Context) *gorm.DB
    // Transaction 执行事务
    Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error
    // AutoMigrate 自动迁移表结构
    AutoMigrate(ctx context.Context, dst ...interface{}) error
    // Ping 检查数据库连接
    Ping(ctx context.Context) error
    // Close 关闭数据库连接
    Close() error
    // HealthCheck 健康检查
    HealthCheck(ctx context.Context) error
}
```

### 2.3 分片相关接口

```go
// ShardingDB 分片数据库接口
type ShardingDB interface {
    // GetShardIndex 获取分片索引
    GetShardIndex(shardingKey interface{}) (int, error)
    // GetShardTable 获取分片表名
    GetShardTable(tableName string, shardingKey interface{}) (string, error)
    // ExecuteOnShard 在指定分片上执行操作
    ExecuteOnShard(ctx context.Context, shardIndex int, fn func(*gorm.DB) error) error
}

// ShardInfo 分片信息
type ShardInfo struct {
    Index      int    `json:"index"`       // 分片索引
    TableName  string `json:"tableName"`   // 分片表名
    Connection string `json:"connection"`  // 连接信息
    Status     string `json:"status"`      // 分片状态
}
```

## 3. 实现要点

### 3.1 连接池管理

```go
type dbProvider struct {
    db      *gorm.DB
    config  *Config
    logger  clog.Logger
    metrics metrics.Provider
    coord   coord.Provider

    // 分片相关
    sharding *shardingManager

    // 连接池监控
    poolStats *sql.DBStats
}

func (p *dbProvider) createDB(config *Config) (*gorm.DB, error) {
    // 创建基础连接
    sqlDB, err := sql.Open("mysql", config.DSN)
    if err != nil {
        return nil, fmt.Errorf("创建数据库连接失败: %w", err)
    }

    // 配置连接池
    sqlDB.SetMaxOpenConns(config.MaxOpenConns)
    sqlDB.SetMaxIdleConns(config.MaxIdleConns)
    sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
    sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

    // 创建 GORM 实例
    gormConfig := &gorm.Config{
        Logger: p.createGormLogger(),
        NamingStrategy: schema.NamingStrategy{
            TablePrefix:   "",
            SingularTable: true,
        },
    }

    db, err := gorm.Open(mysql.New(mysql.Config{
        Conn: sqlDB,
    }), gormConfig)
    if err != nil {
        return nil, fmt.Errorf("创建 GORM 实例失败: %w", err)
    }

    // 配置分片
    if config.Sharding != nil {
        if err := p.setupSharding(db, config.Sharding); err != nil {
            return nil, fmt.Errorf("配置分片失败: %w", err)
        }
    }

    return db, nil
}
```

### 3.2 分片管理

```go
type shardingManager struct {
    config      *ShardingConfig
    shardFunc   func(interface{}) int
    shards      map[int]*gorm.DB
    shardStates map[string]*ShardInfo
    mu          sync.RWMutex
}

func (p *dbProvider) setupSharding(db *gorm.DB, config *ShardingConfig) error {
    p.sharding = &shardingManager{
        config:      config,
        shardFunc:   p.createShardFunction(),
        shards:      make(map[int]*gorm.DB),
        shardStates: make(map[string]*ShardInfo),
    }

    // 注册分片插件
    err := db.Use(sharding.Register(sharding.Config{
        ShardingKey: config.ShardingKey,
        NumberOfShards: config.NumberOfShards,
        PrimaryKeyGenerator: func(table string, column string, value interface{}) (string, error) {
            // 生成主键的逻辑
            return fmt.Sprintf("%s_%v", table, value), nil
        },
    }, sharding.NewMySQL()))

    if err != nil {
        return fmt.Errorf("注册分片插件失败: %w", err)
    }

    // 初始化分片连接
    return p.initializeShards(db)
}

func (p *dbProvider) createShardFunction() func(interface{}) int {
    return func(key interface{}) int {
        var hash int64

        switch v := key.(type) {
        case int, int8, int16, int32, int64:
            hash = int64(v)
        case uint, uint8, uint16, uint32, uint64:
            hash = int64(v)
        case string:
            // 字符串哈希
            hash = p.stringHash(v)
        default:
            // 其他类型转换为字符串后哈希
            hash = p.stringHash(fmt.Sprintf("%v", v))
        }

        // 取绝对值并取模
        abs := hash
        if abs < 0 {
            abs = -abs
        }
        return int(abs % int64(p.sharding.config.NumberOfShards))
    }
}

func (p *dbProvider) stringHash(s string) int64 {
    var hash int64 = 5381
    for _, c := range s {
        hash = ((hash << 5) + hash) + int64(c)
    }
    return hash
}
```

### 3.3 事务管理

```go
func (p *dbProvider) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
    logger := clog.WithContext(ctx)

    // 获取数据库实例
    db := p.db.WithContext(ctx)

    // 开始事务
    tx := db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            logger.Error("事务 panic", clog.Any("recover", r))
        }
    }()

    // 执行事务函数
    if err := fn(tx); err != nil {
        // 事务回滚
        if rbErr := tx.Rollback().Error; rbErr != nil {
            logger.Error("事务回滚失败",
                clog.Err(err),
                clog.Err("rollback_error", rbErr))
        } else {
            logger.Warn("事务回滚成功", clog.Err(err))
        }
        return err
    }

    // 提交事务
    if err := tx.Commit().Error; err != nil {
        logger.Error("事务提交失败", clog.Err(err))
        return err
    }

    logger.Info("事务提交成功")
    return nil
}
```

### 3.4 监控和健康检查

```go
func (p *dbProvider) startMetricsCollection(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            p.collectMetrics()
        }
    }
}

func (p *dbProvider) collectMetrics() {
    if p.db == nil {
        return
    }

    sqlDB, err := p.db.DB()
    if err != nil {
        return
    }

    stats := sqlDB.Stats()

    // 上报连接池指标
    if p.metrics != nil {
        p.metrics.Gauge("db.pool.open_connections", float64(stats.OpenConnections))
        p.metrics.Gauge("db.pool.in_use", float64(stats.InUse))
        p.metrics.Gauge("db.pool.idle", float64(stats.Idle))
        p.metrics.Gauge("db.pool.wait_count", float64(stats.WaitCount))
        p.metrics.Gauge("db.pool.wait_duration", float64(stats.WaitDuration.Milliseconds()))
    }
}

func (p *dbProvider) HealthCheck(ctx context.Context) error {
    if p.db == nil {
        return fmt.Errorf("数据库未初始化")
    }

    sqlDB, err := p.db.DB()
    if err != nil {
        return fmt.Errorf("获取数据库连接失败: %w", err)
    }

    // 检查连接
    if err := sqlDB.Ping(); err != nil {
        return fmt.Errorf("数据库连接失败: %w", err)
    }

    // 检查连接池状态
    stats := sqlDB.Stats()
    if stats.OpenConnections <= 0 {
        return fmt.Errorf("连接池中没有可用连接")
    }

    // 检查分片状态
    if p.sharding != nil {
        if err := p.checkShardsHealth(ctx); err != nil {
            return fmt.Errorf("分片健康检查失败: %w", err)
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

    // 1. 初始化数据库组件
    config := db.GetDefaultConfig("production")
    config.DSN = "user:password@tcp(localhost:3306)/app_db?charset=utf8mb4&parseTime=True&loc=Local"

    // 配置分片
    config.Sharding = &db.ShardingConfig{
        ShardingKey:    "user_id",
        NumberOfShards: 16,
        Tables: map[string]*db.TableShardingConfig{
            "orders": {
                ShardingKey:    "user_id",
                NumberOfShards: 16,
            },
            "payments": {
                ShardingKey:    "user_id",
                NumberOfShards: 8,
            },
        },
    }

    dbProvider, err := db.New(ctx, config,
        db.WithLogger(clog.Namespace("db")),
        db.WithCoordProvider(coordProvider),
        db.WithMetricsProvider(metricsProvider),
    )
    if err != nil {
        log.Fatal("数据库初始化失败:", err)
    }
    defer dbProvider.Close()
}
```

### 4.2 基本 CRUD 操作

```go
type UserService struct {
    db db.Provider
}

func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
    logger := clog.WithContext(ctx)

    user := &User{
        ID:        uuid.New().String(),
        Username:  req.Username,
        Email:     req.Email,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }

    // 插入用户
    if err := s.db.DB(ctx).Create(user).Error; err != nil {
        logger.Error("创建用户失败", clog.Err(err))
        return nil, fmt.Errorf("创建用户失败: %w", err)
    }

    logger.Info("用户创建成功", clog.String("user_id", user.ID))
    return user, nil
}

func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
    logger := clog.WithContext(ctx)

    var user User
    if err := s.db.DB(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, fmt.Errorf("用户不存在")
        }
        logger.Error("查询用户失败", clog.Err(err))
        return nil, fmt.Errorf("查询用户失败: %w", err)
    }

    return &user, nil
}
```

### 4.3 事务操作

```go
func (s *OrderService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    logger := clog.WithContext(ctx)

    var order *Order
    err := s.db.Transaction(ctx, func(tx *gorm.DB) error {
        // 1. 扣减库存
        if err := s.decreaseInventory(ctx, tx, req.ProductID, req.Quantity); err != nil {
            return fmt.Errorf("扣减库存失败: %w", err)
        }

        // 2. 创建订单
        order = &Order{
            ID:         uuid.New().String(),
            UserID:     req.UserID,
            ProductID:  req.ProductID,
            Quantity:   req.Quantity,
            Amount:     req.Amount,
            Status:     "pending",
            CreatedAt:  time.Now(),
        }

        if err := tx.Create(order).Error; err != nil {
            return fmt.Errorf("创建订单失败: %w", err)
        }

        // 3. 创建支付记录
        payment := &Payment{
            ID:        uuid.New().String(),
            OrderID:   order.ID,
            Amount:    req.Amount,
            Status:    "pending",
            CreatedAt: time.Now(),
        }

        if err := tx.Create(payment).Error; err != nil {
            return fmt.Errorf("创建支付记录失败: %w", err)
        }

        logger.Info("订单创建成功",
            clog.String("order_id", order.ID),
            clog.String("user_id", req.UserID))

        return nil
    })

    if err != nil {
        return nil, err
    }

    return order, nil
}
```

### 4.4 分片查询

```go
func (s *OrderService) GetUserOrders(ctx context.Context, userID string, page, pageSize int) ([]*Order, int64, error) {
    logger := clog.WithContext(ctx)

    // 计算分页偏移量
    offset := (page - 1) * pageSize

    var orders []*Order
    var total int64

    // 在分片表中查询用户订单
    if err := s.db.DB(ctx).Model(&Order{}).
        Where("user_id = ?", userID).
        Count(&total).Error; err != nil {
        logger.Error("查询订单总数失败", clog.Err(err))
        return nil, 0, fmt.Errorf("查询订单失败: %w", err)
    }

    if err := s.db.DB(ctx).Where("user_id = ?", userID).
        Order("created_at DESC").
        Offset(offset).
        Limit(pageSize).
        Find(&orders).Error; err != nil {
        logger.Error("查询订单列表失败", clog.Err(err))
        return nil, 0, fmt.Errorf("查询订单失败: %w", err)
    }

    logger.Info("查询用户订单成功",
        clog.String("user_id", userID),
        clog.Int64("total", total),
        clog.Int("count", len(orders)))

    return orders, total, nil
}
```

### 4.5 批量操作

```go
func (s *UserService) BatchUpdateUsers(ctx context.Context, updates []*UserUpdate) error {
    logger := clog.WithContext(ctx)

    // 使用事务进行批量更新
    return s.db.Transaction(ctx, func(tx *gorm.DB) error {
        for _, update := range updates {
            if err := tx.Model(&User{}).
                Where("id = ?", update.ID).
                Updates(update).Error; err != nil {
                logger.Error("批量更新用户失败",
                    clog.String("user_id", update.ID),
                    clog.Err(err))
                return err
            }
        }

        logger.Info("批量更新用户成功", clog.Int("count", len(updates)))
        return nil
    })
}
```

## 5. 高级特性

### 5.1 动态配置支持

```go
func (p *dbProvider) watchConfigChanges(ctx context.Context) {
    if p.coord == nil {
        return
    }

    configPath := "/config/db/"
    watcher, err := p.coord.Config().WatchPrefix(ctx, configPath, &p.config)
    if err != nil {
        p.logger.Error("监听数据库配置失败", clog.Err(err))
        return
    }

    go func() {
        for range watcher.Changes() {
            p.updateConnection()
        }
    }()
}

func (p *dbProvider) updateConnection() {
    // 获取新配置
    newConfig := p.getConfigFromCoord()
    if newConfig == nil {
        return
    }

    // 更新连接池配置
    sqlDB, err := p.db.DB()
    if err != nil {
        return
    }

    sqlDB.SetMaxOpenConns(newConfig.MaxOpenConns)
    sqlDB.SetMaxIdleConns(newConfig.MaxIdleConns)
    sqlDB.SetConnMaxLifetime(newConfig.ConnMaxLifetime)

    p.logger.Info("数据库配置已更新")
}
```

### 5.2 慢查询监控

```go
func (p *dbProvider) createGormLogger() logger.Interface {
    return logger.New(
        log.New(os.Stdout, "\r\n", log.LstdFlags),
        logger.Config{
            SlowThreshold: p.config.SlowThreshold,
            LogLevel:      parseLogLevel(p.config.LogLevel),
            Colorful:      false,
        },
    )
}

func parseLogLevel(level string) logger.LogLevel {
    switch level {
    case "debug":
        return logger.Info
    case "info":
        return logger.Info
    case "warn":
        return logger.Warn
    case "error":
        return logger.Error
    case "silent":
        return logger.Silent
    default:
        return logger.Warn
    }
}
```

### 5.3 连接池健康检查

```go
func (p *dbProvider) checkConnectionPool() error {
    if p.db == nil {
        return fmt.Errorf("数据库未初始化")
    }

    sqlDB, err := p.db.DB()
    if err != nil {
        return fmt.Errorf("获取数据库连接失败: %w", err)
    }

    stats := sqlDB.Stats()

    // 检查连接数是否合理
    if stats.OpenConnections > p.config.MaxOpenConns {
        return fmt.Errorf("连接数超过限制: %d > %d", stats.OpenConnections, p.config.MaxOpenConns)
    }

    // 检查等待数量
    if stats.WaitCount > 1000 {
        return fmt.Errorf("连接等待次数过多: %d", stats.WaitCount)
    }

    // 检查平均等待时间
    if stats.WaitDuration > 5*time.Second {
        return fmt.Errorf("连接等待时间过长: %v", stats.WaitDuration)
    }

    return nil
}
```

## 6. 最佳实践

### 6.1 连接池配置

- **开发环境**: 连接数较少，便于调试
- **生产环境**: 根据业务负载合理配置连接数
- **监控指标**: 定期监控连接池使用情况
- **动态调整**: 支持运行时调整连接池配置

### 6.2 事务使用

- **短事务**: 保持事务简短，避免长事务
- **错误处理**: 正确处理事务错误，确保数据一致性
- **嵌套事务**: 避免过度使用嵌套事务
- **超时控制**: 设置合理的超时时间

### 6.3 分片策略

- **分片键选择**: 选择高基数的字段作为分片键
- **数据均匀**: 确保数据在分片间均匀分布
- **查询优化**: 优先使用分片键进行查询
- **扩容考虑**: 预留扩容空间

### 6.4 性能优化

- **索引优化**: 合理设计索引，提高查询性能
- **批量操作**: 使用批量操作减少数据库访问次数
- **连接复用**: 复用数据库连接，减少连接开销
- **缓存策略**: 合理使用缓存，减少数据库压力

---

*遵循这些指南可以确保数据库组件的高质量实现和稳定运行。*
