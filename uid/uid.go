package uid

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/uid/internal"
)

// Provider 定义唯一 ID 生成组件的主接口
// 提供 Snowflake 和 UUID v7 两种 ID 生成方案
type Provider interface {
	// GetUUIDV7 生成 UUID v7 格式的唯一标识符
	// 适用于需要全局唯一性和可读性的场景，如请求 ID、会话 ID
	GetUUIDV7() string

	// GenerateSnowflake 生成 Snowflake 格式的唯一标识符
	// 适用于需要排序和高性能的场景，如数据库主键、消息 ID
	GenerateSnowflake() (int64, error)

	// IsValidUUID 验证字符串是否为有效的 UUID 格式
	IsValidUUID(s string) bool

	// ParseSnowflake 解析 Snowflake ID，返回时间戳、实例ID和序列号
	ParseSnowflake(id int64) (timestamp, instanceID, sequence int64)

	// Close 释放资源
	Close() error
}

// uidProvider 实现 Provider 接口的具体结构
type uidProvider struct {
	config     *Config
	logger     clog.Logger
	snowflake  *internal.SnowflakeGenerator
	instanceID int64
	closeOnce  sync.Once
}

// New 创建 uid 组件实例
// 遵循 infra-kit 的 Provider 模式
func New(ctx context.Context, config *Config, opts ...Option) (Provider, error) {
	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 解析选项
	options := parseOptions(opts)

	provider := &uidProvider{
		config: config,
		logger: options.logger,
	}

	// 确定实例 ID
	if config.InstanceID > 0 {
		// 使用配置的实例 ID
		provider.instanceID = int64(config.InstanceID)
	} else {
		// 自动分配随机实例 ID
		provider.instanceID = rand.Int63n(int64(config.MaxInstanceID + 1))
	}

	// 初始化 Snowflake 生成器
	provider.snowflake = internal.NewSnowflakeGenerator(provider.instanceID)

	// 记录初始化信息
	if provider.logger != nil {
		provider.logger.Info("uid 组件初始化成功",
			clog.String("service_name", config.ServiceName),
			clog.Int64("instance_id", provider.instanceID),
			clog.Int("max_instance_id", config.MaxInstanceID),
		)
	}

	return provider, nil
}

// GetUUIDV7 生成 UUID v7 格式的唯一标识符
func (p *uidProvider) GetUUIDV7() string {
	return internal.GenerateUUIDV7()
}

// GenerateSnowflake 生成 Snowflake ID
func (p *uidProvider) GenerateSnowflake() (int64, error) {
	return p.snowflake.Generate()
}

// IsValidUUID 验证 UUID 格式
func (p *uidProvider) IsValidUUID(s string) bool {
	return internal.IsValidUUID(s)
}

// ParseSnowflake 解析 Snowflake ID
func (p *uidProvider) ParseSnowflake(id int64) (timestamp, instanceID, sequence int64) {
	return p.snowflake.Parse(id)
}

// Close 释放资源
func (p *uidProvider) Close() error {
	p.closeOnce.Do(func() {
		if p.logger != nil {
			p.logger.Info("uid 组件已关闭",
				clog.String("service_name", p.config.ServiceName),
				clog.Int64("instance_id", p.instanceID),
			)
		}
	})
	return nil
}
