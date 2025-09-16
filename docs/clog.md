# clog 结构化日志组件

**详细文档已迁移到 clog 组件目录**

## 📖 主要文档

- **[README.md](../clog/README.md)** - 完整的使用指南和 API 参考
- **[DESIGN.md](../clog/DESIGN.md)** - 架构设计和实现原理

## 🔗 快速链接

### 功能特性
- 基于 `uber-go/zap` 的高性能结构化日志
- 层次化命名空间系统
- 上下文感知的链路追踪
- 自动日志轮转和文件管理
- 压缩和备份策略

### 核心组件
- **Provider 模式**: 支持依赖注入和独立实例
- **全局初始化**: 简单易用的全局日志器
- **动态配置**: 支持运行时配置更新
- **类型安全**: 编译时检查和错误预防

### 使用示例
```go
// 基础初始化
config := clog.GetDefaultConfig("production")
if err := clog.Init(context.Background(), config, clog.WithNamespace("my-service")); err != nil {
    log.Fatal(err)
}

// 带日志轮转的配置
config := &clog.Config{
    Level:  "info",
    Format: "json",
    Output: "./logs/app.log",
    Rotation: &clog.RotationConfig{
        MaxSize:    100,  // 100MB
        MaxBackups: 5,    // 5 个备份文件
        MaxAge:     30,   // 30 天保留期
        Compress:   true, // 启用压缩
    },
}
```

---

**提示**: 请参考 [clog/README.md](../clog/README.md) 获取完整的文档和示例。