# Go 模块创建指南

## 1. 概述

本指南详细介绍如何为 `infra-kit` 中的每个组件创建独立的 Go 模块，实现组件的模块化管理和独立发布。

### 模块化目标

- **独立发布**: 每个组件可以作为独立的 Go 模块发布和使用
- **依赖管理**: 清晰的依赖关系，避免循环依赖
- **版本控制**: 独立的版本管理，支持语义化版本
- **测试隔离**: 组件间测试隔离，提高测试可靠性

## 2. 项目结构

### 推荐的项目结构

```
go-kit/
├── .github/
│   └── workflows/
│       ├── test.yml          # CI 配置
│       └── release.yml       # 发布配置
├── .gitignore
├── LICENSE
├── README.md
├── Makefile
├── go.work                   # Go 工作区文件
│
├── clog/                     # 日志组件
│   ├── clog.go
│   ├── clog_test.go
│   ├── config.go
│   ├── options.go
│   ├── field.go
│   ├── internal/
│   │   └── logger.go
│   └── go.mod                # 模块文件
│
├── cache/                    # 缓存组件
│   ├── cache.go
│   ├── redis.go
│   ├── lock.go
│   ├── bloom.go
│   ├── internal/
│   │   └── client.go
│   └── go.mod                # 模块文件
│
├── uid/                      # ID 生成组件
│   ├── uid.go
│   ├── snowflake.go
│   ├── uuid.go
│   └── go.mod                # 模块文件
│
└── ... (其他组件)
```

## 3. 创建 Go 模块

### 3.1 初始化模块

在每个组件目录中执行以下命令：

```bash
# 进入组件目录
cd clog

# 初始化 Go 模块
go mod init github.com/your-org/go-kit/clog

# 返回根目录
cd ..
```

### 3.2 模块文件示例

**clog/go.mod**:
```go
module github.com/your-org/go-kit/clog

go 1.21

require (
    github.com/uber-go/zap v1.24.0
)
```

**cache/go.mod**:
```go
module github.com/your-org/go-kit/cache

go 1.21

require (
    github.com/your-org/go-kit/clog v0.1.0
    github.com/go-redis/redis/v8 v8.11.5
)

require (
    github.com/your-org/go-kit/clog v0.1.0 // indirect
)
```

### 3.3 工作区配置

在项目根目录创建 `go.work` 文件：

```go
go 1.21

use (
    ./clog
    ./cache
    ./uid
    ./coord
    ./db
    ./mq
    ./ratelimit
    ./once
    ./breaker
    ./es
    ./metrics
)
```

## 4. 模块依赖管理

### 4.1 依赖层次设计

```
基础层 (无依赖)
├── clog     # 日志组件
└── uid      # ID 生成组件

核心层 (依赖基础层)
├── cache    # 缓存组件 (依赖 clog)
├── coord    # 协调组件 (依赖 clog)
├── db       # 数据库组件 (依赖 clog)
└── mq       # 消息队列组件 (依赖 clog)

服务治理层 (依赖核心层)
├── ratelimit # 限流组件 (依赖 clog, coord, cache)
├── once      # 幂等组件 (依赖 clog, cache)
├── breaker   # 熔断器组件 (依赖 clog, coord)
└── es        # 搜索组件 (依赖 clog)

可观测性层 (依赖基础层)
└── metrics   # 监控组件 (依赖 clog)
```

### 4.2 依赖注入模式

所有组件通过函数选项模式注入依赖：

```go
// cache/options.go
package cache

import (
    "github.com/your-org/go-kit/clog"
    "github.com/your-org/go-kit/coord"
)

type Option func(*options)

type options struct {
    logger clog.Logger
    coord  coord.Provider
}

func WithLogger(logger clog.Logger) Option {
    return func(o *options) {
        o.logger = logger
    }
}

func WithCoordProvider(coord coord.Provider) Option {
    return func(o *options) {
        o.coord = coord
    }
}
```

### 4.3 避免循环依赖

通过接口设计和依赖注入避免循环依赖：

```go
// 错误的循环依赖
// clog 依赖 metrics 进行日志记录
// metrics 依赖 clog 进行日志记录

// 正确的设计
// clog 独立存在，不依赖其他组件
// metrics 依赖 clog 进行日志记录
```

## 5. 组件实现标准

### 5.1 目录结构标准

每个组件应该遵循统一的目录结构：

```
component-name/
├── component.go         # 主要实现文件
├── component_test.go    # 测试文件
├── config.go           # 配置结构
├── options.go          # 函数选项
├── internal/           # 内部实现
│   └── implementation.go
└── go.mod             # 模块文件
```

### 5.2 代码组织原则

- **公共 API**: 只在主包文件中暴露公共接口
- **内部实现**: 使用 `internal` 包隐藏实现细节
- **配置分离**: 配置结构和选项分离
- **测试独立**: 测试文件与实现文件分离

### 5.3 版本管理

使用语义化版本管理：

```
v0.1.0    # 初始版本
v0.1.1    # 补丁版本，修复错误
v0.2.0    # 次要版本，新增功能，保持向后兼容
v1.0.0    # 主要版本，可能包含不兼容的变更
```

## 6. 构建和测试

### 6.1 构建脚本

创建 `Makefile` 简化构建过程：

```makefile
# Makefile
.PHONY: all clean test build release

# 构建所有组件
all:
	@echo "Building all components..."
	@for dir in */; do \
		if [ -f "$$dir/go.mod" ]; then \
			echo "Building $$dir..."; \
			cd $$dir && go build ./... && cd ..; \
		fi \
	done

# 测试所有组件
test:
	@echo "Testing all components..."
	@for dir in */; do \
		if [ -f "$$dir/go.mod" ]; then \
			echo "Testing $$dir..."; \
			cd $$dir && go test -v ./... && cd ..; \
		fi \
	done

# 清理构建产物
clean:
	@echo "Cleaning build artifacts..."
	@find . -name "*.test" -delete
	@find . -name "bin" -type d -exec rm -rf {} +
```

### 6.2 测试策略

每个组件都应该有完整的测试覆盖：

```go
// clog/clog_test.go
package clog

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestLoggerInitialization(t *testing.T) {
    config := GetDefaultConfig("test")
    logger, err := New(context.Background(), config)

    assert.NoError(t, err)
    assert.NotNil(t, logger)

    logger.Info("Test message", String("key", "value"))
}
```

### 6.3 集成测试

创建专门的集成测试目录：

```
integration/
├── test_main.go
├── components/
│   ├── cache_test.go
│   ├── coord_test.go
│   └── db_test.go
└── docker-compose.yml
```

## 7. 发布流程

### 7.1 版本标记

```bash
# 标记版本
git tag v0.1.0
git push origin v0.1.0

# 发布到远程仓库
git push origin main
```

### 7.2 自动化发布

使用 GitHub Actions 自动化发布流程：

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.21

      - name: Release components
        run: |
          for component in clog cache uid coord db mq ratelimit once breaker es metrics; do
            if [ -d "$component" ]; then
              echo "Releasing $component..."
              cd $component
              go mod tidy
              go publish
              cd ..
            fi
          done
```

## 8. 使用示例

### 8.1 在项目中使用

```go
// go.mod
module my-service

go 1.21

require (
    github.com/your-org/go-kit/clog v0.1.0
    github.com/your-org/go-kit/cache v0.1.0
    github.com/your-org/go-kit/uid v0.1.0
)
```

### 8.2 初始化示例

```go
package main

import (
    "context"
    "github.com/your-org/go-kit/clog"
    "github.com/your-org/go-kit/cache"
    "github.com/your-org/go-kit/uid"
)

func main() {
    ctx := context.Background()

    // 初始化日志
    clog.Init(ctx, clog.GetDefaultConfig("production"),
        clog.WithNamespace("my-service"))

    // 初始化缓存
    cacheProvider, err := cache.New(ctx, cache.GetDefaultConfig("production"),
        cache.WithLogger(clog.Namespace("cache")))
    if err != nil {
        panic(err)
    }

    // 初始化 UID 生成器
    uidProvider, err := uid.New(ctx, uid.GetDefaultConfig("production"),
        uid.WithLogger(clog.Namespace("uid")))
    if err != nil {
        panic(err)
    }

    // 使用组件...
}
```

## 9. 最佳实践

### 9.1 模块设计原则

- **单一职责**: 每个模块只负责一个明确的功能
- **最小依赖**: 只依赖必要的第三方库
- **向后兼容**: 保持 API 的向后兼容性
- **清晰文档**: 提供完整的文档和示例

### 9.2 性能优化

- **按需导入**: 只导入实际需要的模块
- **版本管理**: 使用具体的版本号而不是 latest
- **依赖优化**: 定期清理不必要的依赖

### 9.3 维护建议

- **定期更新**: 保持依赖库的版本更新
- **安全扫描**: 定期进行安全漏洞扫描
- **性能测试**: 定期进行性能测试和优化
- **文档同步**: 保持文档与代码同步更新

## 10. 故障排除

### 10.1 常见问题

**问题 1**: 模块依赖解析失败
```bash
go mod tidy
go work sync
```

**问题 2**: 版本冲突
```bash
go list -m all
go mod why -m <module-name>
```

**问题 3**: 循环依赖
```bash
go mod graph | grep "->" | awk '{print $1}' | sort | uniq -d
```

### 10.2 调试工具

使用 Go 提供的工具进行调试：

```bash
# 查看模块依赖图
go mod graph

# 查看为什么需要某个依赖
go mod why -m github.com/uber-go/zap

# 检查依赖一致性
go mod verify
```

---

*遵循这个指南可以确保 Go 模块的正确创建和管理，提高代码的可维护性和可复用性。*
