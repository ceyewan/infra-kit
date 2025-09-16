package config

import "context"

// EventType 表示事件类型。
type EventType string

const (
	EventTypePut    EventType = "PUT"
	EventTypeDelete EventType = "DELETE"
)

// ConfigEvent 表示配置变更事件，泛型以支持类型化的值。
type ConfigEvent[T any] struct {
	Type  EventType // 事件类型
	Key   string    // 配置键
	Value T         // 配置值
}

// Watcher 是用于监听配置变更的泛型接口。
type Watcher[T any] interface {
	// Chan 返回一个接收配置变更事件的通道。
	Chan() <-chan ConfigEvent[T]
	// Close 停止监听器。
	Close()
}

// ConfigCenter 是键值配置存储的接口。
type ConfigCenter interface {
	// Get 获取配置值并反序列化到提供的类型中。
	Get(ctx context.Context, key string, v interface{}) error
	// Set 序列化并存储配置值。
	Set(ctx context.Context, key string, value interface{}) error
	// Delete 删除配置键。
	Delete(ctx context.Context, key string) error
	// Watch 监听单个键的变更，并尝试反序列化为给定类型。
	Watch(ctx context.Context, key string, v interface{}) (Watcher[any], error)
	// WatchPrefix 监听指定前缀下所有键的变更。
	WatchPrefix(ctx context.Context, prefix string, v interface{}) (Watcher[any], error)
	// List 列出指定前缀下的所有键。
	List(ctx context.Context, prefix string) ([]string, error)

	// ===== CAS (Compare-And-Swap) 操作支持 =====

	// GetWithVersion 获取配置值和版本信息
	// 返回值、版本号和错误。版本号用于后续的 CompareAndSet 操作
	GetWithVersion(ctx context.Context, key string, v interface{}) (version int64, err error)

	// CompareAndSet 原子地比较并设置配置值
	// 只有当远程配置的版本号与期望版本号匹配时，才会更新配置
	// 这确保了配置更新的原子性，避免并发修改导致的数据丢失
	CompareAndSet(ctx context.Context, key string, value interface{}, expectedVersion int64) error
}
