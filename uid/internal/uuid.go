package internal

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GenerateUUIDV7 生成 UUID v7 格式的唯一标识符
// 使用 Google UUID 库确保安全性和稳定性
func GenerateUUIDV7() string {
	// 使用 Google UUID 库生成 UUID v7
	// UUID v7 是基于时间戳的 UUID，提供时间排序保证
	u, err := uuid.NewV7()
	if err != nil {
		// 如果 Google UUID 库失败，使用基于时间的随机 UUID 作为备选
		return uuid.New().String()
	}
	return u.String()
}

// GenerateUUIDV7Batch 批量生成 UUID v7
// 适用于需要大量 UUID 的场景
func GenerateUUIDV7Batch(count int) []string {
	if count <= 0 {
		return nil
	}

	uuids := make([]string, count)
	for i := 0; i < count; i++ {
		uuids[i] = GenerateUUIDV7()
	}
	return uuids
}

// IsValidUUID 验证字符串是否为有效的 UUID v7 格式
func IsValidUUID(s string) bool {
	// 使用 Google UUID 库验证基本格式
	parsed, err := uuid.Parse(s)
	if err != nil {
		return false
	}

	// 验证版本号（UUID v7 的版本号必须是 7）
	if parsed.Version() != 7 {
		return false
	}

	// 验证变体（必须是 RFC 4122 变体）
	if parsed.Variant() != uuid.RFC4122 {
		return false
	}

	return true
}

// ExtractTimestampFromUUIDV7 从 UUID v7 中提取时间戳
// 返回 Unix 时间戳（毫秒）
func ExtractTimestampFromUUIDV7(uuidStr string) (int64, error) {
	if !IsValidUUID(uuidStr) {
		return 0, fmt.Errorf("无效的 UUID v7 格式")
	}

	// 使用 Google UUID 库解析 UUID
	u, err := uuid.Parse(uuidStr)
	if err != nil {
		return 0, fmt.Errorf("解析 UUID 失败: %w", err)
	}

	// 使用 Google UUID 库的时间戳提取方法
	// uuid.Time 是 100 纳秒单位，需要转换为毫秒
	uuidTime := u.Time()
	sec, nsec := uuidTime.UnixTime()
	timestamp := sec*1000 + nsec/1000000
	return timestamp, nil
}

// ExtractTimeFromUUIDV7 从 UUID v7 提取时间信息
// 返回 time.Time 格式的时间
func ExtractTimeFromUUIDV7(uuidStr string) (time.Time, error) {
	if !IsValidUUID(uuidStr) {
		return time.Time{}, fmt.Errorf("无效的 UUID v7 格式")
	}

	// 使用 Google UUID 库解析 UUID
	u, err := uuid.Parse(uuidStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("解析 UUID 失败: %w", err)
	}

	// 使用 Google UUID 库的时间戳提取方法
	uuidTime := u.Time()
	sec, nsec := uuidTime.UnixTime()
	return time.Unix(sec, nsec), nil
}
