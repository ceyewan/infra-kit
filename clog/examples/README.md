# clog 使用示例

本目录包含了 clog 包的使用示例，展示了如何按照 GoChat 项目规范使用结构化日志。

## 示例文件

### 1. getting-started/main.go - 快速上手指南 ⭐ **推荐新手首先阅读**
这个示例专门为初学者设计，涵盖了 clog 的核心功能和基本用法。

**覆盖场景：**
- 环境相关配置（开发环境 vs 生产环境）
- 基础日志记录（不同级别、结构化字段）
- 层次化命名空间系统
- 上下文感知日志与链路追踪
- Options 配置模式
- 文件输出功能

**运行方式：**
```bash
cd examples/getting-started
go run main.go
```

### 2. advanced/main.go - 高级功能演示 ⭐ **推荐进阶阅读**
这个示例展示了 clog 在实际生产环境中的高级用法和最佳实践。

**覆盖场景：**
- HTTP 服务集成（Gin 框架）
- 中间件实现（链路追踪、日志记录、异常恢复）
- 复杂业务流程的日志追踪
- 错误处理与监控策略
- 性能优化技巧（批量操作、异步日志、条件日志）

**运行方式：**
```bash
cd examples/advanced
go run main.go
```

### 3. rotation/main.go - 日志轮转演示
专门演示日志文件轮转功能的使用。

**覆盖场景：**
- 文件输出配置
- 日志轮转参数设置
- 大小和数量控制
- 压缩和清理策略

**运行方式：**
```bash
cd examples/rotation
go run main.go
```

## 🎯 学习路径建议

### 初学者路径：
1. **getting-started/main.go** → 了解基本概念和用法
2. **rotation/main.go** → 了解文件管理功能
3. **advanced/main.go** → 学习生产环境最佳实践

### 有经验开发者路径：
1. **advanced/main.go** → 直接学习高级用法
2. **getting-started/main.go** → 快速回顾基础功能
3. **rotation/main.go** → 了解特定功能的实现

## 📋 核心概念演示

### 层次化命名空间系统
所有示例都展示了 clog 的层次化命名空间特性：

```go
// 创建模块级别的命名空间
userLogger := clog.Namespace("user")
authLogger := userLogger.Namespace("auth")
dbLogger := userLogger.Namespace("database")

// 输出示例：
// {"namespace": "advanced-demo.user", "msg": "用户模块启动"}
// {"namespace": "advanced-demo.user.auth", "msg": "用户认证检查"}
// {"namespace": "advanced-demo.user.database", "msg": "查询用户信息"}
```

### 上下文感知与链路追踪
展示了分布式系统中链路追踪的实现：

```go
// 注入 trace ID
ctx := clog.WithTraceID(context.Background(), "trace-123")

// 获取带 trace ID 的 logger
logger := clog.WithContext(ctx)
logger.Info("处理请求") // 自动包含 trace_id 字段
```

### Options 配置模式
演示了函数式选项模式的使用：

```go
// 使用选项初始化
err := clog.Init(ctx, config, clog.WithNamespace("my-service"))

// 创建独立 logger
logger, err := clog.New(ctx, config, clog.WithNamespace("payment-service"))
```

## 🏗️ 架构设计

示例代码展示了 clog 在不同架构层次中的应用：

### 应用层
- HTTP 请求处理
- 业务流程管理
- 错误处理策略

### 服务治理层  
- 中间件实现
- 监控和指标
- 性能优化

### 基础设施层
- 日志配置管理
- 文件轮转策略
- 输出格式控制

## 🚀 运行示例

### 前置条件
确保 Go 环境已正确配置：
```bash
go version  # 应该显示 Go 1.18+
```

### 运行单个示例
```bash
# 快速上手指南
cd examples/getting-started && go run main.go

# 高级功能演示  
cd examples/advanced && go run main.go

# 日志轮转演示
cd examples/rotation && go run main.go
```

### 运行所有示例
```bash
# 在项目根目录运行
find examples -name "main.go" -execdir go run {} \;
```

## 📊 输出示例

### 控制台输出（开发环境）
```
2025-09-16 19:23:05.696	INFO	/Users/harrick/CodeField/infra-kit/clog/examples/getting-started/main.go:88	用户登录	{"email": "user@example.com", "user_id": "12345"}
```

### JSON 输出（生产环境）
```json
{
  "level": "info",
  "time": "2025-09-16 19:23:05.696",
  "caller": "examples/advanced/main.go:234",
  "msg": "HTTP 请求完成",
  "namespace": "advanced-demo.http",
  "method": "POST",
  "path": "/api/users",
  "status": 201,
  "latency": "54.123ms",
  "trace_id": "trace-123456"
}
```

## 💡 最佳实践

从这些示例中，您可以学到以下最佳实践：

1. **统一的初始化模式**：在服务启动时统一初始化 logger
2. **层次化命名空间**：根据业务模块创建清晰的命名空间层次
3. **链路追踪集成**：在请求入口注入 trace ID，贯穿整个调用链
4. **错误处理策略**：使用结构化字段记录详细的错误信息
5. **性能优化**：在高并发场景下使用适当的日志策略
6. **监控集成**：通过日志记录系统健康状态和性能指标

## 🔄 从旧版本迁移

如果您之前使用过旧版本的示例，主要变化：

- 删除了 `basic/main.go`，功能整合到 `getting-started/main.go`
- 删除了 `comprehensive/main.go`，高级功能整合到 `advanced/main.go`
- 删除了 `options/main.go`，配置功能整合到 `getting-started/main.go`
- 保留了 `rotation/main.go`，功能专注且独特

新的示例结构更加清晰，学习路径更加明确。