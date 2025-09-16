# infra-kit 子模块组织指南

## 📋 概述

本文档描述了 infra-kit 项目的子模块组织方案，确保所有组件遵循统一的设计规范和开发模式。

## 🏗️ 目录结构

```
infra-kit/
├── go.work                    # Go 工作区配置
├── go.mod                     # 根模块（文档和示例）
├── README.md                  # 项目总览
├── docs/                      # 文档目录
│   ├── README.md              # 文档导航
│   ├── usage_guide.md         # 使用指南
│   ├── module_creation_guide.md  # 模块创建指南
│   └── {module}.md            # 各组件快速参考
├── scripts/                   # 构建脚本
├── examples/                  # 综合示例
└── components/                # 组件源码（实际模块目录）
    ├── clog/                  # 结构化日志 ✅
    ├── uid/                   # 唯一 ID 生成
    ├── coord/                 # 分布式协调
    ├── cache/                 # 分布式缓存
    ├── db/                    # 数据库访问
    ├── mq/                    # 消息队列
    ├── ratelimit/             # 分布式限流
    ├── once/                  # 分布式幂等
    ├── breaker/               # 熔断器
    ├── es/                    # 搜索引擎
    └── metrics/               # 监控指标
```

## 📦 模块命名规范

### 1. Go 模块名
```
github.com/ceyewan/infra-kit/{component_name}
```

### 2. 组件命名
- 使用小写字母
- 简洁明了，表达核心功能
- 避免缩写，保持可读性

### 3. 版本规范
遵循语义化版本：`MAJOR.MINOR.PATCH`
- `v0.1.0` - 初始版本
- `v0.1.1` - 补丁版本，修复错误
- `v0.2.0` - 次要版本，新增功能
- `v1.0.0` - 主要版本，可能包含不兼容变更

## 🎯 模块创建标准

### 1. 目录结构模板
```
components/{module_name}/
├── go.mod                    # 模块定义
├── README.md                 # 使用文档
├── DESIGN.md                 # 设计文档
├── {module_name}.go          # 主要实现
├── config.go                 # 配置结构
├── options.go                # 函数式选项
├── internal/                 # 内部实现
│   ├── logger.go             # 日志相关（如需要）
│   ├── encoder.go            # 编码器（如需要）
│   └── writer.go             # 写入器（如需要）
├── {module_name}_test.go     # 单元测试
└── examples/                 # 使用示例
    ├── basic/                # 基础用法
    └── advanced/             # 高级用法
```

### 2. go.mod 模板
```go
module github.com/ceyewan/infra-kit/{module_name}

go 1.21

require (
    github.com/ceyewan/infra-kit/clog v0.1.0
    // 其他依赖...
)
```

### 3. Provider 模式实现
每个组件必须实现标准的 Provider 接口：

```go
// 标准构造函数
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)

// 标准配置获取
func GetDefaultConfig(env string) *Config

// Provider 接口
type Provider interface {
    // 组件特定方法
    SomeMethod(ctx context.Context, args ...interface{}) error
    Close() error
}
```

## 🔄 依赖关系管理

### 依赖层次
```
应用层
├── 可观测性层 (metrics)
├── 服务治理层 (ratelimit, once, breaker, es)
├── 核心基础设施层 (coord, cache, db, mq)
└── 基础设施层 (clog, uid)
```

### 依赖规则
1. **基础设施层**: 无依赖或最小依赖
2. **核心基础设施层**: 可依赖基础设施层
3. **服务治理层**: 可依赖基础设施层和核心基础设施层
4. **可观测性层**: 可依赖基础设施层
5. **禁止循环依赖**: 确保依赖图无环

## 🛠️ 开发工作流

### 1. 创建新组件
```bash
# 1. 创建组件目录
mkdir components/{module_name}

# 2. 初始化 Go 模块
cd components/{module_name}
go mod init github.com/ceyewan/infra-kit/{module_name}

# 3. 添加到工作区
cd ../../
go work use ./components/{module_name}

# 4. 创建基础文件
touch README.md DESIGN.md {module_name}.go config.go options.go
```

### 2. 本地开发
```bash
# 在项目根目录
cd /Users/harrick/CodeField/infra-kit

# 工作区会自动处理本地依赖
# 无需手动替换 github.com/ceyewan/infra-kit/{module} 路径
```

### 3. 测试组件
```bash
# 测试单个组件
cd components/{module_name}
go test -v ./...

# 测试所有组件
cd /Users/harrick/CodeField/infra-kit
go work ./...
```

### 4. 发布组件
```bash
# 1. 更新版本
cd components/{module_name}
go mod tidy

# 2. 提交代码
git add .
git commit -m "feat({module}): 新增功能"

# 3. 创建标签
git tag v{version}

# 4. 推送到远程
git push origin main
git push origin v{version}
```

## 📚 文档规范

### 1. README.md 内容
- 组件描述和核心特性
- 快速开始示例
- API 参考
- 配置说明
- 最佳实践

### 2. DESIGN.md 内容
- 设计目标和原则
- 架构设计
- 核心组件说明
- 性能考虑
- 未来扩展

### 3. 代码注释
- 包级别的文档注释
- 公共接口的详细说明
- 复杂算法的实现说明

## 🎨 质量标准

### 1. 代码质量
- 遵循 Go 语言规范
- 使用 golangci-lint 进行代码检查
- 测试覆盖率不低于 80%
- 提供 benchmark 测试（性能关键组件）

### 2. API 设计
- 遵循 Provider 模式
- 使用函数式选项模式
- 提供合理的默认配置
- 支持上下文传播

### 3. 错误处理
- 提供明确的错误类型
- 错误信息包含足够的上下文
- 支持错误包装和链式追踪

## 🔧 工具和脚本

### 1. 构建脚本
```bash
# scripts/build-all.sh
#!/bin/bash
for dir in components/*; do
    if [ -d "$dir" ]; then
        echo "Building $dir..."
        cd "$dir"
        go build ./...
        cd - > /dev/null
    fi
done
```

### 2. 测试脚本
```bash
# scripts/test-all.sh
#!/bin/bash
go work ./...
```

### 3. 发布脚本
```bash
# scripts/release.sh
#!/bin/bash
version=$1
if [ -z "$version" ]; then
    echo "Usage: $0 <version>"
    exit 1
fi

for dir in components/*; do
    if [ -d "$dir" ]; then
        echo "Releasing $dir v$version..."
        cd "$dir"
        git tag "v$version"
        git push origin "v$version"
        cd - > /dev/null
    fi
done
```

## 📋 检查清单

### 创建新组件时
- [ ] 创建标准目录结构
- [ ] 初始化 Go 模块
- [ ] 更新 go.work 文件
- [ ] 实现 Provider 模式
- [ ] 编写单元测试
- [ ] 创建使用示例
- [ ] 编写文档
- [ ] 更新根目录 README

### 发布新版本时
- [ ] 更新所有组件版本号
- [ ] 运行完整测试套件
- [ ] 更新文档
- [ ] 创建 Git 标签
- [ ] 推送到远程仓库

## 🔄 版本兼容性

### 向后兼容
- v0.x 版本保持向后兼容
- v1.0.0 前可以有破坏性变更
- 提供迁移指南

### 废弃策略
- 标记废弃的 API
- 提供替代方案
- 在主版本中移除废弃功能

## 📞 支持

- 创建 Issue 描述问题
- 提交 Pull Request
- 参与讨论和改进

---

遵循此指南可确保 infra-kit 项目的一致性和可维护性。