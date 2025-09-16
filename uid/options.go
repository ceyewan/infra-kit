package uid

import (
	"github.com/ceyewan/infra-kit/clog"
)

// Options 定义 uid 组件的配置选项
type Options struct {
	logger clog.Logger // 日志依赖
}

// Option 定义配置选项的函数类型
// 实现函数式选项模式，支持灵活的依赖注入
type Option func(*Options)

// WithLogger 注入日志依赖
// 用于记录组件运行状态和错误信息
func WithLogger(logger clog.Logger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}

// parseOptions 解析选项参数并返回配置结构
func parseOptions(opts []Option) *Options {
	result := &Options{
		logger: nil,
	}

	for _, opt := range opts {
		opt(result)
	}

	return result
}
