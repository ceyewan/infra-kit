# uid 唯一标识符生成组件

**详细文档已迁移到 uid 组件目录**

## 📖 主要文档

- **[README.md](../uid/README.md)** - 完整的使用指南和 API 参考
- **[DESIGN.md](../uid/DESIGN.md)** - 架构设计和实现原理

## 🔗 快速链接

### 功能特性
- **Snowflake 算法**: 高性能、时间排序的分布式 ID 生成
- **UUID v7 算法**: 基于时间戳的全局唯一标识符
- **多种部署模式**: 单机、多实例、容器化部署支持
- **灵活配置**: 代码配置、环境变量、配置文件支持
- **无外部依赖**: 无需协调服务，简化部署

### 核心组件
- **Provider 模式**: 支持依赖注入和独立实例
- **实例 ID 管理**: 支持配置指定、环境变量、随机分配
- **错误处理**: 完善的错误处理和资源管理
- **高并发安全**: 线程安全的 ID 生成

### 使用示例
```go
// 基础初始化
config := uid.GetDefaultConfig("production")
config.ServiceName = "my-service"

provider, err := uid.New(context.Background(), config)
if err != nil {
    log.Fatal(err)
}
defer provider.Close()

// 生成 UUID v7
requestID := provider.GetUUIDV7()

// 生成 Snowflake ID
orderID, err := provider.GenerateSnowflake()
```

### 应用场景
- **数据库主键**: Snowflake ID 提供排序性和高性能
- **请求追踪**: UUID v7 提供全局唯一的请求 ID
- **会话管理**: UUID v7 提供安全的会话标识符
- **消息队列**: Snowflake ID 提供时间排序的消息 ID
- **外部资源**: UUID v7 提供不暴露内部信息的资源 ID

### 部署模式
- **单机部署**: 自动分配实例 ID，简化配置
- **多实例部署**: 通过配置或环境变量分配实例 ID
- **容器化部署**: 支持 Docker 和 Kubernetes 部署
- **环境配置**: 支持开发、测试、生产环境的不同配置

## 🚀 快速开始

查看 [README.md](../uid/README.md) 获取完整的使用指南和最佳实践。

## 🏗️ 架构设计

查看 [DESIGN.md](../uid/DESIGN.md) 了解详细的架构设计和实现原理。

## 📝 使用示例

查看 [examples/main.go](../uid/examples/main.go) 获取实际使用场景的代码示例。