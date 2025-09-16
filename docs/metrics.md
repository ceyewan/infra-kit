# Metrics 组件

Metrics 组件提供了统一的可观测性接口，基于 OpenTelemetry 实现指标收集和分布式追踪。

## 1. 概述

Metrics 组件实现了基于 Provider 模式的可观测性管理，支持自动化指标收集和链路追踪：

- **自动化收集**：通过拦截器和中间件自动收集请求指标
- **分布式追踪**：基于 OpenTelemetry 的分布式链路追踪
- **多协议支持**：支持多种指标导出协议（Prometheus、Jaeger、Zipkin）
- **自定义指标**：支持业务自定义指标的创建和上报

## 2. 核心接口

### 2.1 Provider 接口

```go
// Provider 定义了可观测性组件的核心接口
type Provider interface {
    // GRPCServerInterceptor 返回 gRPC 服务端拦截器
    GRPCServerInterceptor() grpc.UnaryServerInterceptor

    // GRPCClientInterceptor 返回 gRPC 客户端拦截器
    GRPCClientInterceptor() grpc.UnaryClientInterceptor

    // HTTPMiddleware 返回 HTTP 中间件
    HTTPMiddleware() gin.HandlerFunc

    // NewCounter 创建计数器指标
    NewCounter(name, description string) (Counter, error)

    // NewHistogram 创建直方图指标
    NewHistogram(name, description, unit string) (Histogram, error)

    // NewGauge 创建仪表指标
    NewGauge(name, description string) (Gauge, error)

    // GetMeter 获取 OpenTelemetry Meter
    GetMeter() metric.Meter

    // GetTracer 获取 OpenTelemetry Tracer
    GetTracer() trace.Tracer

    // Shutdown 优雅关闭所有服务
    Shutdown(ctx context.Context) error
}

// Counter 计数器指标接口
type Counter interface {
    Inc(ctx context.Context, attrs ...attribute.KeyValue)
    Add(ctx context.Context, value int64, attrs ...attribute.KeyValue)
}

// Histogram 直方图指标接口
type Histogram interface {
    Record(ctx context.Context, value float64, attrs ...attribute.KeyValue)
}

// Gauge 仪表指标接口
type Gauge interface {
    Record(ctx context.Context, value float64, attrs ...attribute.KeyValue)
}
```

### 2.2 构造函数和配置

```go
// Config 可观测性组件配置
type Config struct {
    // ServiceName 服务名称
    ServiceName string `json:"serviceName"`

    // ServiceVersion 服务版本
    ServiceVersion string `json:"serviceVersion"`

    // Environment 运行环境
    Environment string `json:"environment"`

    // ExporterType 导出器类型
    ExporterType string `json:"exporterType"`

    // ExporterEndpoint 导出器端点
    ExporterEndpoint string `json:"exporterEndpoint"`

    // PrometheusListenAddr Prometheus 监听地址
    PrometheusListenAddr string `json:"prometheusListenAddr"`

    // SamplerType 采样器类型
    SamplerType string `json:"samplerType"`

    // SamplerRatio 采样比例
    SamplerRatio float64 `json:"samplerRatio"`

    // ResourceAttributes 资源属性
    ResourceAttributes map[string]string `json:"resourceAttributes"`

    // ViewConfig 指标视图配置
    ViewConfig []ViewConfig `json:"viewConfig"`
}

// ViewConfig 视图配置
type ViewConfig struct {
    Name      string `json:"name"`
    Description string `json:"description"`
    Aggregation string `json:"aggregation"`
    Attributes []string `json:"attributes"`
}

// GetDefaultConfig 返回默认配置
func GetDefaultConfig(serviceName, env string) *Config

// Option 定义了用于定制可观测性Provider的函数
type Option func(*options)

// WithLogger 注入日志组件
func WithLogger(logger clog.Logger) Option

// WithResourceAttributes 设置资源属性
func WithResourceAttributes(attrs map[string]string) Option

// WithCustomExporter 设置自定义导出器
func WithCustomExporter(exporter spans.Exporter) Option

// WithMetricViews 设置指标视图
func WithMetricViews(views ...view.View) Option

// New 创建可观测性Provider实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
```

## 3. 实现细节

### 3.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                   Metrics Provider                         │
├─────────────────────────────────────────────────────────────┤
│                    Core Interface                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │  Interceptors │  │   Metrics   │  │   Tracing   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                    Implementation                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   OpenTelemetry│  │   Prometheus│  │   Exporters │          │
│  │   SDK       │  │   Exporter  │  │             │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                  Dependencies                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   clog      │  │   gin       │  │   grpc      │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 核心组件

**MetricsProvider**
- 实现Provider接口
- 管理OpenTelemetry Meter和Tracer
- 提供拦截器和中间件

**MetricsCollector**
- 收集和聚合指标数据
- 管理指标生命周期
- 支持自定义指标创建

**TraceCollector**
- 管理分布式追踪
- 处理链路传播
- 支持多种导出器

**ExporterManager**
- 管理指标导出器
- 支持多协议导出
- 处理导出配置

### 3.3 指标类型

**Counter**:
- 单调递增的计数器
- 适用于计数统计
- 支持标签维度

**Histogram**:
- 记录数值分布
- 支持分位数统计
- 适用于延迟和大小统计

**Gauge**:
- 可增可减的仪表
- 适用于实时状态监控
- 支持瞬时值记录

### 3.4 链路追踪

**上下文传播**:
- 使用OpenTelemetry上下文传播
- 支持HTTP和gRPC协议
- 自动生成和传递trace ID

**采样策略**:
- 支持多种采样策略
- 可配置采样率
- 支持自适应采样

## 4. 高级功能

### 4.1 自定义指标

```go
// 创建自定义指标
counter, err := metricsProvider.NewCounter("business_operations_total", "业务操作总数")
histogram, err := metricsProvider.NewHistogram("operation_duration_seconds", "操作耗时", "s")
gauge, err := metricsProvider.NewGauge("active_connections", "活跃连接数")

// 使用自定义指标
counter.Inc(ctx, attribute.String("operation", "create_user"))
histogram.Record(ctx, 1.23, attribute.String("operation", "create_user"))
gauge.Record(ctx, 42.0, attribute.String("service", "database"))
```

### 4.2 指标视图

```go
// 创建指标视图
customView := view.View{
    Name:        "custom_operation_duration",
    Description: "自定义操作耗时视图",
    Aggregation: view.Aggregation{
        Type: view.AggTypeHistogram,
        Bounds: []float64{0.1, 0.5, 1.0, 2.0, 5.0},
    },
    AttributeKeys: []attribute.Key{
        attribute.Key("operation"),
        attribute.Key("status"),
    },
}

// 使用自定义视图
opts := []metrics.Option{
    metrics.WithMetricViews(customView),
}
```

### 4.3 多导出器支持

```go
// 创建自定义导出器
customExporter := &CustomExporter{}

// 使用自定义导出器
opts := []metrics.Option{
    metrics.WithCustomExporter(customExporter),
}
```

## 5. 使用示例

### 5.1 基本使用

```go
package main

import (
    "context"

    "github.com/infra-kit/metrics"
    "github.com/infra-kit/clog"
)

func main() {
    ctx := context.Background()

    // 初始化依赖组件
    logger := clog.New(ctx, &clog.Config{})

    // 获取默认配置
    config := metrics.GetDefaultConfig("user-service", "production")
    config.PrometheusListenAddr = ":9090"
    config.ExporterEndpoint = "http://jaeger:14268/api/traces"

    // 创建可观测性Provider
    opts := []metrics.Option{
        metrics.WithLogger(logger),
    }

    metricsProvider, err := metrics.New(ctx, config, opts...)
    if err != nil {
        logger.Fatal("创建可观测性Provider失败", clog.Err(err))
    }
    defer metricsProvider.Shutdown(ctx)

    // 创建自定义指标
    userCounter, err := metricsProvider.NewCounter("user_operations_total", "用户操作总数")
    if err != nil {
        logger.Fatal("创建指标失败", clog.Err(err))
    }

    // 使用指标
    userCounter.Inc(ctx, attribute.String("operation", "create"))
}
```

### 5.2 gRPC 服务集成

```go
package server

import (
    "context"

    "github.com/infra-kit/metrics"
    "github.com/infra-kit/clog"
    "google.golang.org/grpc"
)

type UserService struct {
    metricsProvider metrics.Provider
    logger         clog.Logger
}

func NewUserService(metricsProvider metrics.Provider, logger clog.Logger) *UserService {
    return &UserService{
        metricsProvider: metricsProvider,
        logger:         logger,
    }
}

func (s *UserService) StartGRPCServer() error {
    // 创建gRPC服务器
    server := grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            s.metricsProvider.GRPCServerInterceptor(),
            // 其他拦截器
        ),
    )

    // 注册服务
    // pb.RegisterUserServiceServer(server, s)

    // 启动服务器
    // return server.ListenAndServe(":8080")
    return nil
}

func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
    // 创建自定义指标
    operationCounter, _ := s.metricsProvider.NewCounter("user_operations_total", "用户操作总数")
    operationHistogram, _ := s.metricsProvider.NewHistogram("user_operation_duration_seconds", "用户操作耗时", "s")

    // 记录操作开始
    startTime := time.Now()

    // 执行业务逻辑
    err := s.doCreateUser(ctx, req)

    // 记录指标
    duration := time.Since(startTime).Seconds()
    operationHistogram.Record(ctx, duration, attribute.String("operation", "create_user"))

    if err != nil {
        operationCounter.Inc(ctx,
            attribute.String("operation", "create_user"),
            attribute.String("status", "failed"),
        )
        return nil, err
    }

    operationCounter.Inc(ctx,
        attribute.String("operation", "create_user"),
        attribute.String("status", "success"),
    )

    return &CreateUserResponse{}, nil
}

func (s *UserService) doCreateUser(ctx context.Context, req *CreateUserRequest) error {
    // 实际的创建用户逻辑
    return nil
}
```

### 5.3 HTTP 服务集成

```go
package api

import (
    "context"

    "github.com/infra-kit/metrics"
    "github.com/infra-kit/clog"
    "github.com/gin-gonic/gin"
)

type APIServer struct {
    metricsProvider metrics.Provider
    logger         clog.Logger
}

func NewAPIServer(metricsProvider metrics.Provider, logger clog.Logger) *APIServer {
    return &APIServer{
        metricsProvider: metricsProvider,
        logger:         logger,
    }
}

func (s *APIServer) StartHTTPServer() error {
    // 创建Gin引擎
    engine := gin.New()

    // 注册中间件
    engine.Use(s.metricsProvider.HTTPMiddleware())
    engine.Use(gin.Recovery())

    // 注册路由
    s.setupRoutes(engine)

    // 启动服务器
    return engine.Run(":8080")
}

func (s *APIServer) setupRoutes(engine *gin.Engine) {
    // 健康检查
    engine.GET("/health", s.healthCheck)

    // 用户相关接口
    userGroup := engine.Group("/users")
    {
        userGroup.POST("/", s.createUser)
        userGroup.GET("/:id", s.getUser)
    }
}

func (s *APIServer) healthCheck(c *gin.Context) {
    // 创建自定义指标
    healthGauge, _ := s.metricsProvider.NewGauge("health_check_status", "健康检查状态")

    // 执行健康检查
    isHealthy := s.doHealthCheck(c.Request.Context())

    // 记录指标
    status := 0.0
    if isHealthy {
        status = 1.0
    }
    healthGauge.Record(c.Request.Context(), status, attribute.String("service", "user-service"))

    if isHealthy {
        c.JSON(200, gin.H{"status": "healthy"})
    } else {
        c.JSON(503, gin.H{"status": "unhealthy"})
    }
}

func (s *APIServer) doHealthCheck(ctx context.Context) bool {
    // 实际的健康检查逻辑
    return true
}
```

## 6. 最佳实践

### 6.1 指标设计

1. **命名规范**：使用一致的指标命名规范
2. **标签使用**：合理使用标签进行维度划分
3. **类型选择**：根据业务场景选择合适的指标类型
4. **粒度控制**：控制指标的粒度和数量

### 6.2 性能优化

1. **采样策略**：合理设置采样率避免性能影响
2. **批量导出**：使用批量导出减少网络开销
3. **内存管理**：控制指标数据的内存占用
4. **异步处理**：使用异步处理避免阻塞业务逻辑

### 6.3 监控配置

1. **告警规则**：设置合理的告警规则和阈值
2. **仪表板**：创建监控仪表板展示关键指标
3. **趋势分析**：分析指标趋势和异常模式
4. **容量规划**：基于指标数据进行容量规划

### 6.4 链路追踪

1. **关键路径**：确保关键业务路径的追踪覆盖
2. **上下文传播**：正确处理上下文传播和父子关系
3. **错误处理**：记录错误信息和异常堆栈
4. **性能分析**：使用追踪数据进行性能分析

## 7. 监控和运维

### 7.1 关键指标

- **请求指标**：请求数量、响应时间、错误率
- **系统指标**：CPU、内存、磁盘、网络使用率
- **业务指标**：业务操作计数、成功率、延迟
- **追踪指标**：追踪数量、采样率、导出成功率

### 7.2 日志规范

- 使用clog组件记录可观测性相关日志
- 记录指标创建和导出过程
- 支持链路追踪日志集成

### 7.3 配置管理

- 通过配置中心统一管理可观测性配置
- 支持配置动态更新
- 提供配置验证和测试

## 8. 配置示例

### 8.1 基础配置

```go
// 开发环境配置
config := &metrics.Config{
    ServiceName:        "user-service-dev",
    Environment:        "development",
    ExporterType:       "stdout",
    SamplerType:        "always_on",
    PrometheusListenAddr: ":9090",
}

// 生产环境配置
config := &metrics.Config{
    ServiceName:        "user-service-prod",
    Environment:        "production",
    ExporterType:       "jaeger",
    ExporterEndpoint:   "http://jaeger:14268/api/traces",
    SamplerType:        "trace_id_ratio",
    SamplerRatio:       0.1,
    PrometheusListenAddr: ":9090",
}
```

### 8.2 高级配置

```go
// 启用资源属性和自定义视图
config := &metrics.Config{
    ServiceName:    "user-service",
    ServiceVersion: "1.0.0",
    Environment:    "production",
    ExporterType:   "jaeger",
    ExporterEndpoint: "http://jaeger:14268/api/traces",
    ResourceAttributes: map[string]string{
        "service.namespace": "gochat",
        "service.instance":  "instance-1",
    },
    ViewConfig: []metrics.ViewConfig{
        {
            Name:        "custom_operation_duration",
            Description: "自定义操作耗时视图",
            Aggregation: "histogram",
            Attributes:  []string{"operation", "status"},
        },
    },
}

// 使用自定义选项
opts := []metrics.Option{
    metrics.WithLogger(logger),
    metrics.WithResourceAttributes(map[string]string{
        "host.name": "server-01",
    }),
    metrics.WithCustomExporter(customExporter),
}
```
