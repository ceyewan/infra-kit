# ES 组件

ES 组件提供了统一的 Elasticsearch 接口，支持高性能的文档索引和复杂搜索功能。

## 1. 概述

ES 组件实现了基于 Provider 模式的 Elasticsearch 客户端，支持泛型设计和类型安全操作：

- **泛型设计**：使用 Go 泛型提供类型安全的索引和搜索操作
- **批量操作**：支持高效的批量索引和搜索
- **动态配置**：通过配置中心实现索引配置的动态更新
- **多索引管理**：支持多个索引的统一管理

## 2. 核心接口

### 2.1 Provider 接口

```go
// Provider 定义了ES组件的核心接口
type Provider interface {
    // BulkIndex 批量索引文档
    BulkIndex[T Indexable](ctx context.Context, items []T) error

    // SearchGlobal 全局搜索
    SearchGlobal[T Indexable](ctx context.Context, operatorID string, keyword string, page, size int) (*SearchResult[T], error)

    // SearchInSession 会话内搜索
    SearchInSession[T Indexable](ctx context.Context, operatorID, sessionID, keyword string, page, size int) (*SearchResult[T], error)

    // SearchByQuery 自定义查询搜索
    SearchByQuery[T Indexable](ctx context.Context, query Query, page, size int) (*SearchResult[T], error)

    // UpdateDocument 更新文档
    UpdateDocument[T Indexable](ctx context.Context, item T) error

    // DeleteDocument 删除文档
    DeleteDocument(ctx context.Context, id string) error

    // CreateIndex 创建索引
    CreateIndex(ctx context.Context, indexConfig IndexConfig) error

    // DeleteIndex 删除索引
    DeleteIndex(ctx context.Context, indexName string) error

    // IndexExists 检查索引是否存在
    IndexExists(ctx context.Context, indexName string) (bool, error)

    // GetDocument 获取文档
    GetDocument[T Indexable](ctx context.Context, id string) (T, error)

    // Close 关闭客户端连接
    Close() error
}

// Indexable 定义了可索引对象的接口
type Indexable interface {
    // GetID 返回文档的唯一ID
    GetID() string
}

// SearchResult 搜索结果
type SearchResult[T Indexable] struct {
    Total      int64 `json:"total"`
    Hits       []T   `json:"hits"`
    Page       int   `json:"page"`
    Size       int   `json:"size"`
    TotalPages int   `json:"total_pages"`
    Took       int   `json:"took"`
}

// Query 查询条件
type Query struct {
    // 查询语句
    Query string `json:"query"`

    // 过滤条件
    Filter map[string]interface{} `json:"filter"`

    // 排序条件
    Sort []SortField `json:"sort"`

    // 聚合条件
    Aggregations map[string]Aggregation `json:"aggregations"`

    // 高亮配置
    Highlight Highlight `json:"highlight"`
}

// IndexConfig 索引配置
type IndexConfig struct {
    Name        string                 `json:"name"`
    Aliases     []string               `json:"aliases"`
    Mappings    map[string]interface{} `json:"mappings"`
    Settings    map[string]interface{} `json:"settings"`
}
```

### 2.2 构造函数和配置

```go
// Config ES组件配置
type Config struct {
    // Addresses ES集群地址
    Addresses []string `json:"addresses"`

    // Username 用户名
    Username string `json:"username"`

    // Password 密码
    Password string `json:"password"`

    // DefaultIndex 默认索引名
    DefaultIndex string `json:"defaultIndex"`

    // Timeout 操作超时时间
    Timeout time.Duration `json:"timeout"`

    // MaxRetries 最大重试次数
    MaxRetries int `json:"maxRetries"`

    // Sniff 是否启用节点发现
    Sniff bool `json:"sniff"`

    // BulkConfig 批量操作配置
    BulkConfig BulkConfig `json:"bulkConfig"`

    // IndexConfigs 索引配置
    IndexConfigs map[string]IndexConfig `json:"indexConfigs"`
}

// BulkConfig 批量操作配置
type BulkConfig struct {
    // Workers 工作协程数
    Workers int `json:"workers"`

    // FlushBytes 刷新字节数
    FlushBytes int `json:"flushBytes"`

    // FlushInterval 刷新间隔
    FlushInterval time.Duration `json:"flushInterval"`

    // MaxRetries 最大重试次数
    MaxRetries int `json:"maxRetries"`

    // CompressRequest 是否压缩请求
    CompressRequest bool `json:"compressRequest"`
}

// GetDefaultConfig 返回默认配置
func GetDefaultConfig(env string) *Config

// Option 定义了用于定制ES Provider的函数
type Option func(*options)

// WithLogger 注入日志组件
func WithLogger(logger clog.Logger) Option

// WithMetricsProvider 注入监控组件
func WithMetricsProvider(provider metrics.Provider) Option

// WithCoordProvider 注入配置中心组件
func WithCoordProvider(provider coord.Provider) Option

// WithBulkConfig 设置批量操作配置
func WithBulkConfig(config BulkConfig) Option

// WithTimeout 设置操作超时时间
func WithTimeout(timeout time.Duration) Option

// New 创建ES Provider实例
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error)
```

## 3. 实现细节

### 3.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      ES Provider                            │
├─────────────────────────────────────────────────────────────┤
│                    Core Interface                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │BulkIndex    │  │  Search     │  │   Manager   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                    Implementation                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Bulk      │  │   Search    │  │   Index     │          │
│  │   Indexer   │  │   Engine    │  │   Manager   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                  Dependencies                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   clog      │  │   metrics   │  │   coord     │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 核心组件

**ESProvider**
- 实现Provider接口
- 管理ES客户端连接
- 提供统一的索引和搜索接口

**BulkIndexer**
- 实现批量索引功能
- 支持异步批量操作
- 提供性能优化和错误处理

**SearchEngine**
- 实现搜索功能
- 支持复杂查询和聚合
- 提供分页和排序功能

**IndexManager**
- 管理索引生命周期
- 处理索引创建和删除
- 支持索引配置管理

### 3.3 索引设计

**索引策略**:
- 支持多索引管理
- 提供索引别名和路由
- 支持索引分片和副本配置

**文档映射**:
- 支持动态映射和静态映射
- 提供字段类型和分词器配置
- 支持父子文档和嵌套对象

### 3.4 搜索算法

**查询构建**:
- 支持多种查询类型（匹配、短语、范围等）
- 提供过滤器和高亮功能
- 支持聚合和分组查询

**性能优化**:
- 使用查询缓存
- 支持搜索建议和纠错
- 提供分页和深度分页优化

## 4. 高级功能

### 4.1 复杂查询

```go
// 构建复杂查询
query := es.Query{
    Query: "content:(搜索 引擎)",
    Filter: map[string]interface{}{
        "timestamp": map[string]interface{}{
            "gte": "2023-01-01",
            "lte": "2023-12-31",
        },
    },
    Sort: []es.SortField{
        {Field: "timestamp", Order: "desc"},
    },
    Highlight: es.Highlight{
        Fields: []string{"content", "title"},
    },
}

// 执行搜索
result, err := esProvider.SearchByQuery[Document](ctx, query, 1, 10)
```

### 4.2 聚合查询

```go
// 聚合查询
query := es.Query{
    Query: "match_all",
    Aggregations: map[string]es.Aggregation{
        "by_category": {
            Terms: &es.TermsAggregation{
                Field: "category.keyword",
                Size:  10,
            },
        },
        "by_date": {
            DateHistogram: &es.DateHistogramAggregation{
                Field:    "timestamp",
                Interval: "month",
            },
        },
    },
}

result, err := esProvider.SearchByQuery[Document](ctx, query, 1, 0)
```

### 4.3 监控和指标

```go
// 监控指标
metrics := map[string]string{
    "index_operations_total":      "索引操作总数",
    "search_operations_total":      "搜索操作总数",
    "bulk_operations_total":        "批量操作总数",
    "index_latency_ms":             "索引操作延迟",
    "search_latency_ms":            "搜索操作延迟",
    "bulk_size_bytes":              "批量操作大小",
    "active_connections":          "活跃连接数",
}
```

## 5. 使用示例

### 5.1 基本使用

```go
package main

import (
    "context"
    "time"

    "github.com/infra-kit/es"
    "github.com/infra-kit/clog"
)

// 定义文档结构
type Message struct {
    ID        string    `json:"id"`
    SessionID string    `json:"session_id"`
    SenderID  string    `json:"sender_id"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}

// 实现Indexable接口
func (m *Message) GetID() string {
    return m.ID
}

func main() {
    ctx := context.Background()

    // 初始化依赖组件
    logger := clog.New(ctx, &clog.Config{})

    // 获取默认配置
    config := es.GetDefaultConfig("production")
    config.Addresses = []string{"http://localhost:9200"}
    config.DefaultIndex = "messages"

    // 创建ES Provider
    opts := []es.Option{
        es.WithLogger(logger),
        es.WithTimeout(10 * time.Second),
    }

    esProvider, err := es.New(ctx, config, opts...)
    if err != nil {
        logger.Fatal("创建ES Provider失败", clog.Err(err))
    }
    defer esProvider.Close()

    // 批量索引文档
    messages := []*Message{
        {ID: "1", SessionID: "s1", SenderID: "u1", Content: "Hello", Timestamp: time.Now()},
        {ID: "2", SessionID: "s1", SenderID: "u2", Content: "World", Timestamp: time.Now()},
    }

    err = esProvider.BulkIndex(ctx, messages)
    if err != nil {
        logger.Error("批量索引失败", clog.Err(err))
    }

    // 搜索文档
    result, err := esProvider.SearchGlobal[Message](ctx, "u1", "Hello", 1, 10)
    if err != nil {
        logger.Error("搜索失败", clog.Err(err))
    } else {
        logger.Info("搜索结果", clog.Int64("total", result.Total))
    }
}
```

### 5.2 高级搜索

```go
package search

import (
    "context"

    "github.com/infra-kit/es"
    "github.com/infra-kit/clog"
)

type SearchService struct {
    esProvider es.Provider
    logger     clog.Logger
}

func NewSearchService(esProvider es.Provider, logger clog.Logger) *SearchService {
    return &SearchService{
        esProvider: esProvider,
        logger:     logger,
    }
}

func (s *SearchService) AdvancedSearch(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
    // 构建复杂查询
    query := es.Query{
        Query: req.Keyword,
        Filter: map[string]interface{}{
            "session_id": req.SessionID,
            "timestamp": map[string]interface{}{
                "gte": req.StartTime,
                "lte": req.EndTime,
            },
        },
        Sort: []es.SortField{
            {Field: "timestamp", Order: "desc"},
        },
        Highlight: es.Highlight{
            Fields: []string{"content"},
            PreTags:  "<em>",
            PostTags: "</em>",
        },
    }

    // 执行搜索
    result, err := s.esProvider.SearchByQuery[Document](ctx, query, req.Page, req.Size)
    if err != nil {
        s.logger.Error("高级搜索失败", clog.Err(err))
        return nil, err
    }

    // 转换结果
    response := &SearchResponse{
        Total:      result.Total,
        Page:       result.Page,
        Size:       result.Size,
        TotalPages: result.TotalPages,
        Took:       result.Took,
        Documents:  make([]*Document, len(result.Hits)),
    }

    for i, doc := range result.Hits {
        response.Documents[i] = doc
    }

    return response, nil
}
```

### 5.3 索引管理

```go
package admin

import (
    "context"

    "github.com/infra-kit/es"
    "github.com/infra-kit/clog"
)

type IndexAdmin struct {
    esProvider es.Provider
    logger     clog.Logger
}

func NewIndexAdmin(esProvider es.Provider, logger clog.Logger) *IndexAdmin {
    return &IndexAdmin{
        esProvider: esProvider,
        logger:     logger,
    }
}

func (a *IndexAdmin) CreateMessageIndex(ctx context.Context) error {
    indexConfig := es.IndexConfig{
        Name: "messages",
        Aliases: []string{"messages_v1"},
        Mappings: map[string]interface{}{
            "properties": map[string]interface{}{
                "id": map[string]interface{}{
                    "type": "keyword",
                },
                "session_id": map[string]interface{}{
                    "type": "keyword",
                },
                "sender_id": map[string]interface{}{
                    "type": "keyword",
                },
                "content": map[string]interface{}{
                    "type": "text",
                    "analyzer": "ik_max_word",
                    "search_analyzer": "ik_smart",
                },
                "timestamp": map[string]interface{}{
                    "type": "date",
                },
            },
        },
        Settings: map[string]interface{}{
            "number_of_shards":   3,
            "number_of_replicas": 1,
        },
    }

    return a.esProvider.CreateIndex(ctx, indexConfig)
}

func (a *IndexAdmin) Reindex(ctx context.Context, sourceIndex, targetIndex string) error {
    // 实现索引重建逻辑
    return nil
}
```

## 6. 最佳实践

### 6.1 索引设计

1. **合理分片**：根据数据量和查询模式设置合适的分片数
2. **字段映射**：为字段选择合适的数据类型和分析器
3. **索引别名**：使用别名实现索引的无缝切换
4. **生命周期**：实现索引的生命周期管理

### 6.2 性能优化

1. **批量操作**：使用批量索引提高写入性能
2. **查询缓存**：合理使用查询缓存
3. **分页优化**：避免深度分页，使用search_after
4. **连接池**：合理配置连接池大小

### 6.3 监控和运维

1. **集群健康**：监控集群健康状态
2. **性能指标**：监控索引和搜索性能
3. **磁盘空间**：监控磁盘使用情况
4. **内存使用**：监控JVM堆内存使用

### 6.4 错误处理

1. **重试机制**：实现网络异常的重试逻辑
2. **降级策略**：ES不可用时的降级处理
3. **错误分类**：区分网络错误和业务错误
4. **熔断保护**：使用熔断器保护ES调用

## 7. 监控和运维

### 7.1 关键指标

- **索引性能**：索引操作的数量和延迟
- **搜索性能**：搜索操作的数量和延迟
- **集群状态**：集群健康度和节点状态
- **资源使用**：CPU、内存、磁盘使用率

### 7.2 日志规范

- 使用clog组件记录ES操作日志
- 记录查询性能和错误信息
- 支持链路追踪集成

### 7.3 故障排除

1. **连接问题**：检查ES集群和网络连接
2. **性能问题**：检查查询语句和索引配置
3. **数据问题**：检查数据格式和映射配置
4. **内存问题**：检查JVM配置和查询复杂度

## 8. 配置示例

### 8.1 基础配置

```go
// 开发环境配置
config := &es.Config{
    Addresses:     []string{"http://localhost:9200"},
    DefaultIndex:  "messages-dev",
    Timeout:       10 * time.Second,
    MaxRetries:    3,
    Sniff:         false,
}

// 生产环境配置
config := &es.Config{
    Addresses:     []string{"http://es1:9200", "http://es2:9200", "http://es3:9200"},
    Username:     "admin",
    Password:     "password",
    DefaultIndex: "messages-prod",
    Timeout:      30 * time.Second,
    MaxRetries:   5,
    Sniff:        true,
    BulkConfig: es.BulkConfig{
        Workers:         5,
        FlushBytes:     5 * 1024 * 1024, // 5MB
        FlushInterval:  10 * time.Second,
        MaxRetries:     3,
        CompressRequest: true,
    },
}
```

### 8.2 高级配置

```go
// 启用监控和配置中心
opts := []es.Option{
    es.WithLogger(logger),
    es.WithMetricsProvider(metricsProvider),
    es.WithCoordProvider(coordProvider),
    es.WithBulkConfig(es.BulkConfig{
        Workers:        10,
        FlushBytes:    10 * 1024 * 1024,
        FlushInterval: 5 * time.Second,
        MaxRetries:    5,
    }),
    es.WithTimeout(30 * time.Second),
}
```
