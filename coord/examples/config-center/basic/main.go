package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ceyewan/infra-kit/coord"
)

// DatabaseConfig 数据库配置示例
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// ServerConfig 服务器配置示例
type ServerConfig struct {
	Port         int      `json:"port"`
	ReadTimeout  int      `json:"read_timeout"`
	WriteTimeout int      `json:"write_timeout"`
	AllowedHosts []string `json:"allowed_hosts"`
}

func main() {
	fmt.Println("=== 配置中心 - 基础用法 ===")
	fmt.Println("演示配置的增删改查和版本控制")

	// 创建协调器
	cfg := coord.GetDefaultConfig("development")
	provider, err := coord.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("创建协调器失败: %v", err)
	}
	defer provider.Close()

	configService := provider.Config()
	ctx := context.Background()

	// 1. 基本CRUD操作
	basicCRUDDemo(ctx, configService)

	// 2. 版本控制和CAS操作
	versionControlDemo(ctx, configService)

	// 3. 配置前缀操作
	prefixOperationsDemo(ctx, configService)

	fmt.Println("\n=== 基础用法示例完成 ===")
}

// basicCRUDDemo 演示配置的基本增删改查操作
func basicCRUDDemo(ctx context.Context, configService config.ConfigCenter) {
	fmt.Println("\n--- 基本CRUD操作 ---")

	// 设置数据库配置
	dbConfig := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Username: "admin",
		Password: "secret",
		Database: "myapp",
	}

	dbConfigKey := "config/database"
	if err := configService.Set(ctx, dbConfigKey, dbConfig); err != nil {
		log.Printf("设置数据库配置失败: %v", err)
		return
	}
	fmt.Println("✓ 数据库配置设置成功")

	// 获取数据库配置
	var retrievedDBConfig DatabaseConfig
	if err := configService.Get(ctx, dbConfigKey, &retrievedDBConfig); err != nil {
		log.Printf("获取数据库配置失败: %v", err)
		return
	}
	fmt.Printf("  获取到数据库配置: Host=%s, Port=%d, Database=%s\n",
		retrievedDBConfig.Host, retrievedDBConfig.Port, retrievedDBConfig.Database)

	// 设置服务器配置
	serverConfig := ServerConfig{
		Port:         8080,
		ReadTimeout:  30,
		WriteTimeout: 30,
		AllowedHosts: []string{"localhost", "127.0.0.1"},
	}

	serverConfigKey := "config/server"
	if err := configService.Set(ctx, serverConfigKey, serverConfig); err != nil {
		log.Printf("设置服务器配置失败: %v", err)
		return
	}
	fmt.Println("✓ 服务器配置设置成功")

	// 获取服务器配置
	var retrievedServerConfig ServerConfig
	if err := configService.Get(ctx, serverConfigKey, &retrievedServerConfig); err != nil {
		log.Printf("获取服务器配置失败: %v", err)
		return
	}
	fmt.Printf("  获取到服务器配置: Port=%d, AllowedHosts=%v\n",
		retrievedServerConfig.Port, retrievedServerConfig.AllowedHosts)

	// 删除配置
	if err := configService.Delete(ctx, dbConfigKey); err != nil {
		log.Printf("删除数据库配置失败: %v", err)
	} else {
		fmt.Println("✓ 数据库配置删除成功")
	}

	if err := configService.Delete(ctx, serverConfigKey); err != nil {
		log.Printf("删除服务器配置失败: %v", err)
	} else {
		fmt.Println("✓ 服务器配置删除成功")
	}
}

// versionControlDemo 演示版本控制和CAS操作
func versionControlDemo(ctx context.Context, configService config.ConfigCenter) {
	fmt.Println("\n--- 版本控制和CAS操作 ---")

	configKey := "config/versioned/app"

	// 初始配置
	initialConfig := ServerConfig{
		Port:         8000,
		ReadTimeout:  10,
		WriteTimeout: 10,
		AllowedHosts: []string{"localhost"},
	}

	// 设置初始配置
	if err := configService.Set(ctx, configKey, initialConfig); err != nil {
		log.Printf("设置初始配置失败: %v", err)
		return
	}
	fmt.Println("✓ 初始配置设置成功")

	// 获取配置和版本
	var config ServerConfig
	version, err := configService.GetWithVersion(ctx, configKey, &config)
	if err != nil {
		log.Printf("获取配置和版本失败: %v", err)
		return
	}
	fmt.Printf("  初始配置版本: %d\n", version)

	// 尝试CAS操作 - 使用正确的版本应该成功
	newConfig := ServerConfig{
		Port:         8001,
		ReadTimeout:  15,
		WriteTimeout: 15,
		AllowedHosts: []string{"localhost", "127.0.0.1"},
	}

	if err := configService.CompareAndSet(ctx, configKey, newConfig, version); err != nil {
		log.Printf("CAS操作失败: %v", err)
	} else {
		fmt.Println("✓ CAS操作成功（使用正确版本）")
	}

	// 尝试CAS操作 - 使用错误的版本应该失败
	wrongConfig := ServerConfig{
		Port:         8002,
		ReadTimeout:  20,
		WriteTimeout: 20,
		AllowedHosts: []string{"*"},
	}

	if err := configService.CompareAndSet(ctx, configKey, wrongConfig, 999); err != nil {
		fmt.Printf("✓ CAS操作失败（使用错误版本，符合预期）: %v\n", err)
	} else {
		fmt.Println("✗ CAS操作意外成功")
	}

	// 获取最新版本
	var latestConfig ServerConfig
	latestVersion, err := configService.GetWithVersion(ctx, configKey, &latestConfig)
	if err != nil {
		log.Printf("获取最新配置失败: %v", err)
		return
	}
	fmt.Printf("  最新配置版本: %d, Port: %d\n", latestVersion, latestConfig.Port)

	// 清理
	if err := configService.Delete(ctx, configKey); err != nil {
		log.Printf("删除配置失败: %v", err)
	} else {
		fmt.Println("✓ 配置删除成功")
	}
}

// prefixOperationsDemo 演示配置前缀操作
func prefixOperationsDemo(ctx context.Context, configService config.ConfigCenter) {
	fmt.Println("\n--- 配置前缀操作 ---")

	basePath := "config/microservice"

	// 创建多个配置
	configs := []struct {
		key   string
		value interface{}
	}{
		{basePath + "/service1", ServerConfig{Port: 8081, ReadTimeout: 30, WriteTimeout: 30}},
		{basePath + "/service2", ServerConfig{Port: 8082, ReadTimeout: 30, WriteTimeout: 30}},
		{basePath + "/database", DatabaseConfig{Host: "db1.example.com", Port: 5432, Username: "user1"}},
		{basePath + "/cache", map[string]interface{}{"type": "redis", "host": "redis.example.com"}},
	}

	// 批量设置配置
	for _, cfg := range configs {
		if err := configService.Set(ctx, cfg.key, cfg.value); err != nil {
			log.Printf("设置配置 %s 失败: %v", cfg.key, err)
			continue
		}
		fmt.Printf("✓ 配置 %s 设置成功\n", cfg.key)
	}

	// 列出指定前缀的所有配置
	keys, err := configService.ListKeys(ctx, basePath)
	if err != nil {
		log.Printf("列出配置键失败: %v", err)
		return
	}

	fmt.Printf("  发现 %d 个配置:\n", len(keys))
	for _, key := range keys {
		fmt.Printf("    - %s\n", key)
	}

	// 批量删除配置
	for _, cfg := range configs {
		if err := configService.Delete(ctx, cfg.key); err != nil {
			log.Printf("删除配置 %s 失败: %v", cfg.key, err)
			continue
		}
		fmt.Printf("✓ 配置 %s 删除成功\n", cfg.key)
	}

	// 验证删除
	keys, err = configService.ListKeys(ctx, basePath)
	if err != nil {
		log.Printf("列出配置键失败: %v", err)
		return
	}

	if len(keys) == 0 {
		fmt.Println("✓ 所有配置已成功删除")
	} else {
		fmt.Printf("✗ 仍有 %d 个配置未删除\n", len(keys))
	}
}

// errorHandlingDemo 演示错误处理
func errorHandlingDemo(ctx context.Context, configService config.ConfigCenter) {
	fmt.Println("\n--- 错误处理 ---")

	// 测试获取不存在的配置
	var nonExistentConfig ServerConfig
	err := configService.Get(ctx, "config/nonexistent", &nonExistentConfig)
	if err != nil {
		fmt.Printf("✓ 获取不存在的配置失败（符合预期）: %v\n", err)
	} else {
		fmt.Println("✗ 获取不存在的配置意外成功")
	}

	// 测试删除不存在的配置
	err = configService.Delete(ctx, "config/nonexistent")
	if err != nil {
		fmt.Printf("✓ 删除不存在的配置失败（符合预期）: %v\n", err)
	} else {
		fmt.Println("✗ 删除不存在的配置意外成功")
	}

	// 测试使用错误类型的结构体获取配置
	err = configService.Get(ctx, "config/invalid", struct{}{})
	if err != nil {
		fmt.Printf("✓ 使用错误类型获取配置失败（符合预期）: %v\n", err)
	} else {
		fmt.Println("✗ 使用错误类型获取配置意外成功")
	}
}
