# Breaker 组件

Breaker 组件提供了统一的熔断器接口，实现服务保护、防止雪崩效应，确保系统的可用性和稳定性。

## 1. 概述

Breaker 组件实现了基于 Provider 模式的熔断器管理，支持动态配置和自动恢复：

- **熔断保护**：当下游服务持续失败时自动熔断，防止雪崩效应
- **自动恢复**：支持半开状态探测，自动恢复服务调用
- **动态配置**：通过配置中心实现熔断策略的动态更新
- **多实例管理**：支持多个熔断器实例的统一管理

## 2. 核心接口

### 2.1 Provider 接口

```go
// Provider 定义了熔断器管理的核心接口
type Provider interface {
    // GetBreaker 获取或创建指定名称的熔断器实例
    // name是被保护资源的唯一标识，如"grpc:user-service"或"http:payment-api"
    GetBreaker(name string) Breaker
    
    // Close 关闭Provider，释放资源
    Close() error
}

// Breaker 定义了熔断器的核心接口
type Breaker interface {
    // Do 执行受熔断器保护的操作
    // 如果熔断器打开，立即返回ErrBreakerOpen错误
    // 否则执行操作，根据执行结果更新熔断器状态
    Do(ctx context.Context, op func() error) error
    
    // Execute 执行带返回值的受保护操作
    Execute(ctx context.Context, op func() (any, error)) (any, error)
    
    // State 获取当前熔断器状态
    State() State
    
    // Name 获取熔断器名称
    Name() string
    
    // Reset 重置熔断器状态
    Reset() error
}

// State 定义了熔断器的状态
type State int

const (
    StateClosed State = iota  // 关闭状态：正常调用
    StateOpen                // 打开状态：熔断中
    StateHalfOpen            // 半开状态：探测恢复
)
```

### 2.2 构造函数和配置

```go
// Config 熔断器组件配置
type Config struct {
    // ServiceName 服务名称，用于日志和监控
    ServiceName string `json:"serviceName"`
    
    // PoliciesPath 熔断策略在配置中心的路径
    PoliciesPath string `json:"policiesPath"`
    
    // DefaultPolicy 默认熔断策略
    DefaultPolicy Policy `json:"defaultPolicy"`
    
    // EnableDynamicConfig 是否启用动态配置
    EnableDynamicConfig bool `json:"enableDynamicConfig"`
}

// Policy 熔断策略定义
type Policy struct {
    // FailureThreshold 触发熔断的连续失败次数
    FailureThreshold int `json:"failureThreshold"`
    
    // SuccessThreshold 半开状态下连续成功次数
    SuccessThreshold int `json:"successThreshold"`
    
    // OpenStateTimeout 熔断器打开状态的持续时间
    OpenStateTimeout time.Duration `json:"openStateTimeout"`
    
    // HalfOpenMaxRequests 半开状态最大请求数
    HalfOpenMaxRequests int `json:"halfOpenMaxRequests"`
    
    // Interval 状态检查间隔
    Interval time.Duration `json:"interval"`
    
    // Timeout 操作超时时间
    Timeout time.Duration `json:"timeout"`
}

// GetDefaultConfig 返回默认配置
func GetDefaultConfig(serviceName, env string) *Config

// Option 定义了用于定制熔断器Provider的函数
type Option func(*options)

// WithLogger 注入日志组件
func WithLogger(logger clog.Logger) Option

// WithCoordProvider 注入配置中心组件
func WithCoordProvider(provider coord.Provider) Option

// WithMetricsProvider 注入监控组件
func WithMetricsProvider(provider metrics.Provider) Option

// WithDefaultPolicy 设置默认熔断策略
func WithDefaultPolicy(policy Policy) Option

// New 创建熔断器Provider实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
```

## 3. 实现细节

### 3.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                   Breaker Provider                         │
├─────────────────────────────────────────────────────────────┤
│                    Core Interface                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │GetBreaker   │  │   Close     │  │   Manager   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                    Implementation                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Circuit   │  │   State     │  │   Config    │          │
│  │   Breaker   │  │   Manager   │  │   Manager   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                  Dependencies                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   clog      │  │   coord     │  │   metrics   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 核心组件

**BreakerProvider**
- 实现Provider接口
- 管理多个熔断器实例
- 提供熔断器生命周期管理

**CircuitBreaker**
- 基于sony/gobreaker实现
- 支持状态管理和自动恢复
- 提供熔断和恢复逻辑

**StateManager**
- 管理熔断器状态
- 处理状态转换逻辑
- 支持状态持久化

**ConfigManager**
- 管理熔断策略配置
- 监听配置变化
- 动态更新熔断器策略

### 3.3 熔断算法

**状态机设计**:
- 关闭状态：正常调用，统计失败次数
- 打开状态：熔断中，快速失败
- 半开状态：探测恢复，限量放行

**触发条件**:
- 连续失败达到阈值触发熔断
- 超时时间后进入半开状态
- 半开状态连续成功后恢复

### 3.4 动态配置

**配置热更新**:
- 监听配置中心变化
- 自动加载新策略
- 平滑更新熔断器配置

**策略管理**:
- 支持策略的CRUD操作
- 策略验证和冲突检测
- 版本控制和回滚

## 4. 高级功能

### 4.1 自定义策略

```go
// 自定义熔断策略
customPolicy := &breaker.Policy{
    FailureThreshold:    5,
    SuccessThreshold:    3,
    OpenStateTimeout:    30 * time.Second,
    HalfOpenMaxRequests: 5,
    Interval:           1 * time.Second,
    Timeout:            5 * time.Second,
}

// 创建带自定义策略的熔断器
breakerInstance := provider.GetBreaker("custom-service")
```

### 4.2 监控和指标

```go
// 监控指标
metrics := map[string]string{
    "requests_total":        "总请求数",
    "success_total":        "成功请求数",
    "failure_total":        "失败请求数",
    "circuit_opened_total": "熔断开启次数",
    "state_changes_total":  "状态变更次数",
    "current_state":        "当前状态",
}
```

### 4.3 事件回调

```go
// 设置熔断器事件回调
breakerInstance.OnStateChange(func(from, to breaker.State) {
    logger.Info("熔断器状态变更", 
        clog.String("from", from.String()),
        clog.String("to", to.String()))
})
```

## 5. 使用示例

### 5.1 基本使用

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/gochat-kit/breaker"
    "github.com/gochat-kit/clog"
    "github.com/gochat-kit/coord"
)

func main() {
    ctx := context.Background()
    
    // 初始化依赖组件
    logger := clog.New(ctx, &clog.Config{})
    coordProvider := coord.New(ctx, &coord.Config{})
    
    // 获取默认配置
    config := breaker.GetDefaultConfig("user-service", "production")
    config.PoliciesPath = "/config/prod/user-service/breakers/"
    
    // 创建熔断器Provider
    opts := []breaker.Option{
        breaker.WithLogger(logger),
        breaker.WithCoordProvider(coordProvider),
    }
    
    breakerProvider, err := breaker.New(ctx, config, opts...)
    if err != nil {
        logger.Fatal("创建熔断器失败", clog.Err(err))
    }
    defer breakerProvider.Close()
    
    // 使用熔断器
    serviceBreaker := breakerProvider.GetBreaker("user-service")
    
    // 执行受保护的操作
    err = serviceBreaker.Do(ctx, func() error {
        // 调用下游服务
        return callUserService()
    })
    
    if err != nil {
        if errors.Is(err, breaker.ErrBreakerOpen) {
            logger.Error("服务熔断中")
        } else {
            logger.Error("服务调用失败", clog.Err(err))
        }
    }
}

func callUserService() error {
    // 实际的服务调用逻辑
    return nil
}
```

### 5.2 gRPC 拦截器集成

```go
package interceptor

import (
    "context"
    "errors"
    
    "github.com/gochat-kit/breaker"
    "github.com/gochat-kit/clog"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func BreakerClientInterceptor(provider breaker.Provider) grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        // 使用方法名作为熔断器名称
        b := provider.GetBreaker(method)
        
        // 执行受保护的gRPC调用
        err := b.Do(ctx, func() error {
            return invoker(ctx, method, req, reply, cc, opts...)
        })
        
        // 转换熔断器错误为gRPC错误
        if errors.Is(err, breaker.ErrBreakerOpen) {
            return status.Error(codes.Unavailable, "service unavailable due to circuit breaker")
        }
        
        return err
    }
}
```

### 5.3 HTTP 客户端集成

```go
package client

import (
    "context"
    "encoding/json"
    "net/http"
    
    "github.com/gochat-kit/breaker"
    "github.com/gochat-kit/clog"
)

type HTTPClient struct {
    breaker breaker.Breaker
    client  *http.Client
}

func NewHTTPClient(breaker breaker.Breaker) *HTTPClient {
    return &HTTPClient{
        breaker: breaker,
        client:  &http.Client{},
    }
}

func (c *HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
    var resp *http.Response
    var err error
    
    // 使用熔断器保护HTTP请求
    err = c.breaker.Do(ctx, func() error {
        resp, err = c.client.Do(req)
        return err
    })
    
    if err != nil {
        return nil, err
    }
    
    return resp, nil
}

func (c *HTTPClient) GetJSON(ctx context.Context, url string, target interface{}) error {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }
    
    resp, err := c.Do(ctx, req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return json.NewDecoder(resp.Body).Decode(target)
}
```

## 6. 最佳实践

### 6.1 熔断策略配置

1. **阈值设置**：根据服务重要性设置合理的熔断阈值
2. **超时配置**：设置适当的熔断超时和恢复时间
3. **分级熔断**：对不同重要性的服务使用不同的熔断策略
4. **动态调整**：根据监控数据动态调整熔断策略

### 6.2 错误处理

1. **快速失败**：熔断时立即返回错误，避免等待
2. **降级处理**：提供降级逻辑或默认值
3. **重试机制**：在熔断器关闭时实现重试
4. **错误分类**：区分网络错误和业务错误

### 6.3 监控和告警

1. **实时监控**：监控熔断器状态和切换频率
2. **告警设置**：设置熔断告警阈值
3. **趋势分析**：分析熔断趋势，优化服务配置
4. **容量规划**：基于熔断数据进行容量规划

### 6.4 性能优化

1. **轻量级实现**：使用高效的熔断算法
2. **内存优化**：避免熔断器实例泄漏
3. **并发安全**：确保熔断器状态更新的线程安全
4. **资源清理**：及时清理不再使用的熔断器

## 7. 监控和运维

### 7.1 关键指标

- **熔断次数**：熔断器开启的次数
- **成功率**：成功请求数占比
- **响应时间**：请求响应时间分布
- **状态变更**：熔断器状态变更频率

### 7.2 日志规范

- 使用clog组件记录熔断相关日志
- 记录状态变更和熔断决策
- 支持链路追踪集成

### 7.3 配置管理

- 通过配置中心统一管理熔断策略
- 支持配置版本控制和回滚
- 提供配置验证和冲突检测

## 8. 故障排除

### 8.1 常见问题

1. **频繁熔断**：检查熔断阈值和服务可用性
2. **恢复缓慢**：检查半开状态配置和探测逻辑
3. **配置不同步**：检查配置中心连接和监听
4. **性能问题**：检查熔断器实例数量和管理逻辑

### 8.2 调试方法

1. **启用调试日志**：查看熔断器状态变更过程
2. **监控指标**：分析熔断器行为模式
3. **压力测试**：验证熔断策略的有效性
4. **配置验证**：验证熔断配置的正确性

### 8.3 配置示例

```go
// 高敏感服务熔断配置
sensitivePolicy := &breaker.Policy{
    FailureThreshold:    3,
    SuccessThreshold:    5,
    OpenStateTimeout:    60 * time.Second,
    HalfOpenMaxRequests: 2,
    Interval:           500 * time.Millisecond,
    Timeout:            3 * time.Second,
}

// 普通服务熔断配置
normalPolicy := &breaker.Policy{
    FailureThreshold:    10,
    SuccessThreshold:    3,
    OpenStateTimeout:    30 * time.Second,
    HalfOpenMaxRequests: 5,
    Interval:           1 * time.Second,
    Timeout:            5 * time.Second,
}

// 后台服务熔断配置
backgroundPolicy := &breaker.Policy{
    FailureThreshold:    20,
    SuccessThreshold:    1,
    OpenStateTimeout:    120 * time.Second,
    HalfOpenMaxRequests: 10,
    Interval:           2 * time.Second,
    Timeout:            10 * time.Second,
}
```