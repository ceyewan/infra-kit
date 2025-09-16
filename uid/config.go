package uid

import (
	"fmt"
	"os"
	"strconv"
)

// Config 定义 uid 组件的配置结构
type Config struct {
	ServiceName   string `json:"serviceName"`   // 服务名称，用于日志和监控
	MaxInstanceID int    `json:"maxInstanceID"` // 最大实例 ID，默认 1023
	InstanceID    int    `json:"instanceId"`    // 实例 ID，可选（0 表示自动分配）
}

// GetDefaultConfig 返回环境相关的默认配置
// 根据不同的运行环境提供优化的配置
func GetDefaultConfig(env string) *Config {
	config := &Config{
		ServiceName:   getEnvWithDefault("SERVICE_NAME", "unknown-service"),
		MaxInstanceID: getEnvIntWithDefault("MAX_INSTANCE_ID", 1023),
		InstanceID:    getEnvIntWithDefault("INSTANCE_ID", 0), // 0 表示自动分配
	}

	// 根据环境调整默认值
	if env == "development" {
		if config.InstanceID == 0 {
			config.InstanceID = 1 // 开发环境默认使用实例 ID 1
		}
	}

	return config
}

// Validate 验证配置的有效性
// 在初始化组件之前调用，确保配置参数的正确性
func (c *Config) Validate() error {
	// 验证服务名称
	if c.ServiceName == "" {
		return fmt.Errorf("服务名称不能为空")
	}

	// 验证实例 ID 范围
	if c.InstanceID < 0 || c.InstanceID > c.MaxInstanceID {
		return fmt.Errorf("实例 ID 必须在 0-%d 范围内（0 表示自动分配）", c.MaxInstanceID)
	}

	// 验证最大实例 ID
	if c.MaxInstanceID <= 0 || c.MaxInstanceID > 1023 {
		return fmt.Errorf("最大实例 ID 必须在 1-1023 范围内")
	}

	return nil
}

// SetServiceName 设置服务名称
// 提供便捷的配置方法
func (c *Config) SetServiceName(name string) *Config {
	c.ServiceName = name
	return c
}

// SetMaxInstanceID 设置最大实例 ID
// 提供便捷的配置方法
func (c *Config) SetMaxInstanceID(maxID int) *Config {
	c.MaxInstanceID = maxID
	return c
}

// SetInstanceID 设置实例 ID
// 提供便捷的配置方法
func (c *Config) SetInstanceID(instanceID int) *Config {
	c.InstanceID = instanceID
	return c
}

// 环境变量辅助函数
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
