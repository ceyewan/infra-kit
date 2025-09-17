# Coord 模块示例

本目录提供了 coord 模块的完整使用示例，涵盖分布式锁、配置中心、服务注册发现和ID生成器四大核心功能。

## 🚀 快速开始

### 运行快速入门示例

```bash
# 查看所有功能的基本用法
cd getting-started
go run main.go
```

### 运行特定功能示例

```bash
# 分布式锁示例
cd distributed-lock/basic
go run main.go

# 配置中心示例
cd config-center/basic
go run main.go

# 服务注册发现示例
cd service-discovery/basic
go run main.go

# ID生成器示例
cd id-generator/basic
go run main.go
```

## 📁 目录结构

```
coord/examples/
├── README.md                          # 本文档
├── getting-started/                   # 快速入门 - 所有功能一览
│   └── main.go                        # 四大功能的基本用法演示
├── distributed-lock/                 # 分布式锁专题
│   ├── basic/                         # 基础用法
│   │   └── main.go                    # 锁获取/释放、阻塞/非阻塞、TTL管理
│   ├── advanced/                      # 高级用法
│   │   └── main.go                    # 手动续约、过期检查、长时间持有
│   └── patterns/                      # 使用模式
│       └── main.go                    # 错误处理、重试机制、并发控制
├── config-center/                     # 配置中心专题
│   ├── basic/                         # 基础用法
│   │   └── main.go                    # 配置CRUD、版本控制、前缀操作
│   ├── watch/                         # 配置监听
│   │   └── main.go                    # 动态配置更新、事件监听
│   └── manager/                       # 配置管理器
│       └── main.go                    # 类型安全配置管理、验证器、更新器
├── service-discovery/                 # 服务注册发现专题
│   ├── basic/                         # 基础用法
│   │   └── main.go                    # 服务注册/发现/注销、元数据管理
│   ├── grpc-integration/             # gRPC集成
│   │   └── main.go                    # gRPC resolver动态服务发现
│   └── health-check/                  # 健康检查
│       └── main.go                    # 服务健康状态监控
├── id-generator/                      # ID生成器专题
│   ├── basic/                         # 基础用法
│   │   └── main.go                    # 实例ID分配/释放、容量管理
│   ├── patterns/                      # 使用模式
│   │   └── main.go                    # ID池管理、并发分配、租约管理
│   └── integration/                   # 集成使用
│       └── main.go                    # 与其他功能配合使用
└── comprehensive/                     # 综合示例（高级场景）
    ├── microservice/                  # 微服务场景
    │   └── main.go                    # 完整的微服务协调示例
    └── benchmark/                     # 性能测试
        └── main.go                    # 性能基准测试和对比
```

## 🔧 环境要求

### 前置条件

- Go 1.19+
- etcd 3.4+ （运行在 `localhost:2379`）

### 启动 etcd

如果还没有运行 etcd，可以使用 Docker 启动：

```bash
# 启动 etcd
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  quay.io/coreos/etcd:v3.5.0 \
  /usr/local/bin/etcd \
  --name etcd0 \
  --advertise-client-urls http://localhost:2379 \
  --listen-client-urls http://0.0.0.0:2379 \
  --initial-advertise-peer-urls http://localhost:2380 \
  --listen-peer-urls http://0.0.0.0:2380 \
  --initial-cluster etcd0=http://localhost:2380
```

## 📚 功能详解

### 1. 分布式锁 (Distributed Lock)

提供基于 etcd session 的分布式锁实现，支持自动续约和手动管理。

#### 特性
- ✅ 阻塞和非阻塞获取模式
- ✅ TTL 自动续约机制
- ✅ 手动续约和过期检查
- ✅ 完善的错误处理
- ✅ 并发安全保证

#### 适用场景
- 资源互斥访问
- 任务调度
- 分布式事务协调
- 防止重复操作

#### 快速示例
```go
// 获取锁服务
lockService := provider.Lock()

// 阻塞获取锁
lock, err := lockService.Acquire(ctx, "my-resource", 10*time.Second)
if err != nil {
    log.Fatal(err)
}
defer lock.Unlock(ctx)

// 受保护的操作
fmt.Println("执行受保护的工作...")

// 检查TTL
ttl, _ := lock.TTL(ctx)
fmt.Printf("锁剩余TTL: %v\n", ttl)
```

---

### 2. 配置中心 (Config Center)

提供统一的配置管理，支持版本控制、动态更新和类型安全。

#### 特性
- ✅ 配置的增删改查
- ✅ 版本控制和CAS操作
- ✅ 配置前缀管理
- ✅ 动态配置监听
- ✅ 类型安全的配置管理器

#### 适用场景
- 应用配置管理
- 功能开关控制
- 环境配置同步
- 运行时配置更新

#### 快速示例
```go
// 获取配置服务
configService := provider.Config()

// 设置配置
appConfig := AppConfig{
    AppName: "my-app",
    Version: "1.0.0",
    Port:    8080,
}

err := configService.Set(ctx, "config/app", appConfig)
if err != nil {
    log.Fatal(err)
}

// 获取配置
var retrievedConfig AppConfig
err = configService.Get(ctx, "config/app", &retrievedConfig)
if err != nil {
    log.Fatal(err)
}
```

---

### 3. 服务注册发现 (Service Discovery)

提供完整的服务注册、发现和健康检查机制，支持 gRPC 集成。

#### 特性
- ✅ 服务注册和注销
- ✅ 服务发现和查询
- ✅ 丰富的元数据支持
- ✅ gRPC resolver 集成
- ✅ 健康状态监控
- ✅ 多实例负载均衡

#### 适用场景
- 微服务架构
- 服务网格
- 动态负载均衡
- 服务治理

#### 快速示例
```go
// 获取注册服务
registryService := provider.Registry()

// 注册服务
serviceInfo := registry.ServiceInfo{
    ID:      "instance-1",
    Name:    "my-service",
    Address: "127.0.0.1",
    Port:    8080,
    Metadata: map[string]string{
        "version": "1.0.0",
        "region":  "local",
    },
}

err := registryService.Register(ctx, serviceInfo, 30*time.Second)
if err != nil {
    log.Fatal(err)
}
defer registryService.Unregister(ctx, serviceInfo.ID)

// 发现服务
services, err := registryService.Discover(ctx, "my-service")
if err != nil {
    log.Fatal(err)
}

for _, svc := range services {
    fmt.Printf("发现服务: %s:%d\n", svc.Address, svc.Port)
}
```

---

### 4. ID生成器 (ID Generator)

提供分布式环境下的唯一实例ID分配和管理。

#### 特性
- ✅ 唯一ID保证
- ✅ 容量管理
- ✅ 租约自动管理
- ✅ 并发安全分配
- ✅ ID回收机制

#### 适用场景
- 微服务实例标识
- 分布式任务分配
- 会话管理
- 资源标识

#### 快速示例
```go
// 获取ID分配器
allocatorService := provider.Allocator()

// 分配实例ID
instanceID, err := allocatorService.AcquireID(ctx)
if err != nil {
    log.Fatal(err)
}
defer instanceID.Close(ctx)

fmt.Printf("分配到实例ID: %d\n", instanceID.ID())

// 使用ID进行工作
fmt.Printf("实例 %d 正在工作...\n", instanceID.ID())
```

## 🎯 学习路径

### 初学者

1. **开始** → `getting-started/` - 了解所有功能
2. **深入** → `*/basic/` - 学习每个功能的基础用法
3. **实践** → `*/patterns/` - 掌握常见使用模式

### 进阶用户

1. **高级特性** → `*/advanced/` - 学习高级功能和最佳实践
2. **集成使用** → `*/integration/` - 了解功能间的配合使用
3. **性能优化** → `comprehensive/benchmark/` - 学习性能调优

### 生产环境

1. **完整示例** → `comprehensive/microservice/` - 参考完整的微服务实现
2. **错误处理** → 学习各个示例中的错误处理模式
3. **监控和日志** → 了解如何集成监控和日志系统

## 💡 最佳实践

### 1. 资源管理
- 始终使用 `defer` 确保资源释放
- 正确处理上下文取消
- 合理设置TTL和租约时间

### 2. 错误处理
- 检查所有可能的错误
- 实现适当的重试机制
- 记录详细的错误信息

### 3. 并发安全
- 避免在关键路径中的阻塞操作
- 合理使用超时控制
- 注意资源竞争条件

### 4. 性能优化
- 批量操作优于单个操作
- 合理使用连接池
- 避免频繁的配置更新

## 🐛 故障排除

### 常见问题

1. **etcd 连接失败**
   - 检查 etcd 是否运行在 `localhost:2379`
   - 确认网络连接正常
   - 检查防火墙设置

2. **锁获取超时**
   - 增加锁的TTL时间
   - 检查是否有死锁
   - 优化锁的持有时间

3. **配置更新不生效**
   - 检查配置监听器是否正确启动
   - 确认配置路径正确
   - 检查网络连接

4. **服务发现失败**
   - 确认服务已正确注册
   - 检查服务名称是否匹配
   - 验证元数据格式

### 调试技巧

1. **启用详细日志**
   ```go
   cfg := coord.GetDefaultConfig("development")
   // 或使用 "debug" 获取更详细的日志
   ```

2. **使用测试工具**
   ```bash
   # 运行测试验证功能
   go test -v ./coord/...
   ```

3. **监控 etcd 状态**
   ```bash
   # 查看 etcd 键值
   etcdctl get --prefix /coord/
   ```

## 📖 更多资源

- [API 文档](../../docs/api.md)
- [设计文档](../../docs/design.md)
- [性能测试报告](../../docs/performance.md)
- [部署指南](../../docs/deployment.md)

## 🤝 贡献

欢迎提交问题和改进建议！请参考项目的 [贡献指南](../../CONTRIBUTING.md)。

---

**注意**: 所有示例都需要 etcd 服务运行在 `localhost:2379`。在生产环境中，请根据实际环境修改连接配置。