package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ceyewan/infra-kit/clog"
)

func main() {
	fmt.Println("=== clog 日志轮转完整演示 ===")

	// 演示1: 基本轮转功能
	demonstrateBasicRotation()

	// 演示2: 压缩轮转功能
	demonstrateCompressedRotation()

	// 演示3: 自定义配置轮转
	demonstrateCustomRotation()
}

// demonstrateBasicRotation 演示基本轮转功能
func demonstrateBasicRotation() {
	fmt.Println("\n--- 1. 基本轮转演示 ---")

	// 清理之前的日志文件
	logFile := "basic-rotation.log"
	cleanupLogFile(logFile)

	// 配置基本轮转
	config := &clog.Config{
		Level:     "info",
		Format:    "json",
		Output:    logFile,
		AddSource: false, // 简化输出
		Rotation: &clog.RotationConfig{
			MaxSize:    1,     // 1MB
			MaxBackups: 2,     // 保留2个备份
			MaxAge:     1,     // 1天
			Compress:   false, // 不压缩
		},
	}

	// 初始化 logger
	if err := clog.Init(context.Background(), config); err != nil {
		fmt.Printf("❌ 初始化 logger 失败: %v\n", err)
		return
	}

	ctx := clog.WithTraceID(context.Background(), "basic-rotation-trace")

	fmt.Printf("📁 日志文件: %s\n", logFile)
	fmt.Printf("⚙️  配置: MaxSize=1MB, MaxBackups=2, Compress=false\n")

	// 生成足够日志来触发轮转
	fmt.Println("🔄 生成日志...")
	for i := 1; i <= 100; i++ {
		logger := clog.WithContext(ctx).Namespace(fmt.Sprintf("batch.%d", i))

		// 生成较长内容
		content := fmt.Sprintf("基本轮转测试消息 #%d", i)
		for j := 0; j < 20; j++ {
			content += fmt.Sprintf(" 填充内容%d: ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", j)
		}

		logger.Info("轮转测试",
			clog.Int("sequence", i),
			clog.String("content", content),
			clog.Time("timestamp", time.Now()),
		)

		if i%25 == 0 {
			fmt.Printf("已生成 %d 条日志\n", i)
		}
	}

	checkRotationFiles(logFile, "基本轮转")
}

// demonstrateCompressedRotation 演示压缩轮转功能
func demonstrateCompressedRotation() {
	fmt.Println("\n--- 2. 压缩轮转演示 ---")

	// 清理之前的日志文件
	logFile := "compressed-rotation.log"
	cleanupLogFile(logFile)

	// 配置压缩轮转
	config := &clog.Config{
		Level:     "info",
		Format:    "json",
		Output:    logFile,
		AddSource: false,
		Rotation: &clog.RotationConfig{
			MaxSize:    1,    // 1MB
			MaxBackups: 3,    // 保留3个备份
			MaxAge:     7,    // 7天
			Compress:   true, // 启用压缩
		},
	}

	// 初始化 logger
	if err := clog.Init(context.Background(), config); err != nil {
		fmt.Printf("❌ 初始化 logger 失败: %v\n", err)
		return
	}

	ctx := clog.WithTraceID(context.Background(), "compressed-rotation-trace")

	fmt.Printf("📁 日志文件: %s\n", logFile)
	fmt.Printf("⚙️  配置: MaxSize=1MB, MaxBackups=3, Compress=true\n")

	// 生成大量日志
	fmt.Println("🔄 生成大量日志...")
	for i := 1; i <= 150; i++ {
		logger := clog.WithContext(ctx).Namespace(fmt.Sprintf("compressed.%d", i))

		// 生成更长的内容
		content := fmt.Sprintf("压缩轮转测试消息 #%d", i)
		for j := 0; j < 50; j++ {
			content += fmt.Sprintf(" 大量填充内容%d: ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz", j)
		}

		logger.Info("压缩轮转测试",
			clog.Int("sequence", i),
			clog.String("content", content),
			clog.Duration("processing_time", time.Duration(i)*time.Millisecond),
			clog.Time("timestamp", time.Now()),
		)

		if i%30 == 0 {
			fmt.Printf("已生成 %d 条日志\n", i)
		}
	}

	checkRotationFiles(logFile, "压缩轮转")
}

// demonstrateCustomRotation 演示自定义配置轮转
func demonstrateCustomRotation() {
	fmt.Println("\n--- 3. 自定义配置轮转演示 ---")

	// 清理之前的日志文件
	logFile := "custom-rotation.log"
	cleanupLogFile(logFile)

	// 自定义配置：更小的文件大小，更多备份
	config := &clog.Config{
		Level:     "debug",
		Format:    "console", // 控制台格式，便于观察
		Output:    logFile,
		AddSource: true,
		Rotation: &clog.RotationConfig{
			MaxSize:    1,    // 1MB - 小文件便于快速测试
			MaxBackups: 5,    // 保留5个备份
			MaxAge:     30,   // 30天
			Compress:   true, // 启用压缩
		},
	}

	// 初始化 logger
	if err := clog.Init(context.Background(), config); err != nil {
		fmt.Printf("❌ 初始化 logger 失败: %v\n", err)
		return
	}

	ctx := clog.WithTraceID(context.Background(), "custom-rotation-trace")

	fmt.Printf("📁 日志文件: %s\n", logFile)
	fmt.Printf("⚙️  配置: MaxSize=1MB, MaxBackups=5, MaxAge=30d, Compress=true\n")

	// 模拟不同级别的日志
	fmt.Println("🔄 生成不同级别的日志...")
	for i := 1; i <= 80; i++ {
		logger := clog.WithContext(ctx).Namespace(fmt.Sprintf("custom.%d", i))

		// 生成超长内容
		content := fmt.Sprintf("自定义轮转测试消息 #%d - 包含大量数据用于测试轮转机制", i)
		for j := 0; j < 100; j++ {
			content += fmt.Sprintf(" 数据块%d: 这是超长的测试数据，包含字母、数字和特殊字符ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz!@#$%^&*()", j)
		}

		// 不同级别的日志
		switch i % 4 {
		case 0:
			logger.Debug("调试信息",
				clog.Int("sequence", i),
				clog.String("content", content),
				clog.String("level", "debug"))
		case 1:
			logger.Info("信息日志",
				clog.Int("sequence", i),
				clog.String("content", content),
				clog.String("level", "info"))
		case 2:
			logger.Warn("警告日志",
				clog.Int("sequence", i),
				clog.String("content", content),
				clog.String("level", "warn"))
		case 3:
			logger.Error("错误日志",
				clog.Int("sequence", i),
				clog.String("content", content),
				clog.String("level", "error"))
		}

		if i%20 == 0 {
			fmt.Printf("已生成 %d 条日志\n", i)
		}
	}

	checkRotationFiles(logFile, "自定义轮转")
}

// cleanupLogFile 清理日志文件
func cleanupLogFile(baseFile string) {
	os.Remove(baseFile)
	for i := 1; i <= 10; i++ {
		os.Remove(fmt.Sprintf("%s.%d", baseFile, i))
		os.Remove(fmt.Sprintf("%s.%d.gz", baseFile, i))
	}
}

// checkRotationFiles 检查轮转文件
func checkRotationFiles(baseFile string, testName string) {
	fmt.Printf("\n📊 %s 结果:\n", testName)

	// 检查主文件
	if info, err := os.Stat(baseFile); err == nil {
		fmt.Printf("📄 当前日志: %s (%.2f KB)\n", baseFile, float64(info.Size())/1024)
	}

	// 检查备份文件
	totalBackups := 0
	totalCompressed := 0

	for i := 1; i <= 10; i++ {
		backupFile := fmt.Sprintf("%s.%d", baseFile, i)
		if info, err := os.Stat(backupFile); err == nil {
			fmt.Printf("📄 备份文件: %s (%.2f KB)\n", backupFile, float64(info.Size())/1024)
			totalBackups++
		}

		// 检查压缩文件
		compressedFile := fmt.Sprintf("%s.%d.gz", baseFile, i)
		if info, err := os.Stat(compressedFile); err == nil {
			fmt.Printf("📦 压缩文件: %s (%.2f KB)\n", compressedFile, float64(info.Size())/1024)
			totalCompressed++
		}
	}

	fmt.Printf("📈 统计: 备份文件=%d, 压缩文件=%d\n", totalBackups, totalCompressed)

	if totalBackups > 0 || totalCompressed > 0 {
		fmt.Println("✅ 轮转功能正常工作")
	} else {
		fmt.Println("ℹ️  未触发轮转 (可能需要更多日志或更小的 MaxSize)")
	}
}
