package internal

import (
	"fmt"
	"sync"
	"time"
)

// Snowflake 算法常量定义
const (
	SnowflakeEpoch = 1609459200000 // 2021-01-01 00:00:00 UTC (毫秒时间戳)
	InstanceIDBits = 10            // 实例 ID 占用位数
	SequenceBits   = 12            // 序列号占用位数

	MaxInstanceID = (1 << InstanceIDBits) - 1 // 最大实例 ID: 1023
	MaxSequence   = (1 << SequenceBits) - 1   // 最大序列号: 4095

	InstanceIDShift = SequenceBits                  // 实例 ID 左移位数
	TimestampShift  = InstanceIDBits + SequenceBits // 时间戳左移位数
)

// SnowflakeGenerator 实现 Snowflake ID 生成器
// 支持高并发、时钟回拨检测和序列号管理
type SnowflakeGenerator struct {
	mu         sync.Mutex
	instanceID int64
	sequence   int64
	lastTime   int64
	epoch      int64
}

// NewSnowflakeGenerator 创建新的 Snowflake 生成器
func NewSnowflakeGenerator(instanceID int64) *SnowflakeGenerator {
	if instanceID < 0 || instanceID > MaxInstanceID {
		panic(fmt.Sprintf("实例 ID 必须在 0-%d 范围内", MaxInstanceID))
	}

	return &SnowflakeGenerator{
		instanceID: instanceID,
		epoch:      SnowflakeEpoch,
		lastTime:   0,
		sequence:   0,
	}
}

// Generate 生成 Snowflake ID
// 返回生成的 ID 和可能的错误
func (g *SnowflakeGenerator) Generate() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 获取当前时间戳（相对于 epoch）
	currentTime := time.Now().UnixMilli() - g.epoch

	// 检测时钟回拨
	if currentTime < g.lastTime {
		return 0, fmt.Errorf("时钟回拨检测：上次时间 %d，当前时间 %d", g.lastTime, currentTime)
	}

	// 同一毫秒内，递增序列号
	if currentTime == g.lastTime {
		g.sequence = (g.sequence + 1) & MaxSequence
		if g.sequence == 0 {
			// 序列号溢出，等待下一毫秒
			for currentTime <= g.lastTime {
				currentTime = time.Now().UnixMilli() - g.epoch
			}
		}
	} else {
		// 新的毫秒，重置序列号
		g.sequence = 0
	}

	// 更新最后生成时间
	g.lastTime = currentTime

	// 组合 ID：时间戳 + 实例 ID + 序列号
	id := (currentTime << TimestampShift) |
		(g.instanceID << InstanceIDShift) |
		g.sequence

	return id, nil
}

// TODO: 未来考虑添加批量生成功能，但需要解决并发安全问题
// GenerateBatch 批量生成 Snowflake ID
// 适用于需要大量 ID 的场景，提高生成效率
// 注意：此方法存在并发安全问题，暂时不实现
// func (g *SnowflakeGenerator) GenerateBatch(count int) ([]int64, error)

// Parse 解析 Snowflake ID
// 返回时间戳、实例 ID 和序列号
func (g *SnowflakeGenerator) Parse(id int64) (timestamp, instanceID, sequence int64) {
	// 提取序列号（低 12 位）
	sequence = id & MaxSequence

	// 提取实例 ID（中间 10 位）
	instanceID = (id >> InstanceIDShift) & MaxInstanceID

	// 提取时间戳（高 42 位）
	timestamp = (id >> TimestampShift)

	return timestamp, instanceID, sequence
}

// GetTimestampFromID 从 Snowflake ID 获取时间戳
// 返回 Unix 时间戳（毫秒）
func (g *SnowflakeGenerator) GetTimestampFromID(id int64) int64 {
	timestamp := (id >> TimestampShift)
	return g.epoch + timestamp
}

// GetInstanceIDFromID 从 Snowflake ID 获取实例 ID
func (g *SnowflakeGenerator) GetInstanceIDFromID(id int64) int64 {
	return (id >> InstanceIDShift) & MaxInstanceID
}

// GetSequenceFromID 从 Snowflake ID 获取序列号
func (g *SnowflakeGenerator) GetSequenceFromID(id int64) int64 {
	return id & MaxSequence
}

// ExtractTime 从 Snowflake ID 提取时间信息
// 返回 time.Time 格式的时间
func (g *SnowflakeGenerator) ExtractTime(id int64) time.Time {
	timestamp := g.GetTimestampFromID(id)
	return time.Unix(timestamp/1000, (timestamp%1000)*1000000)
}
