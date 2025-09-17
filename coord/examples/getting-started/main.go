package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ceyewan/infra-kit/coord"
	"github.com/ceyewan/infra-kit/coord/registry"
)

// AppConfig 应用配置示例
type AppConfig struct {
	AppName   string `json:"app_name"`
	Version   string `json:"version"`
	Port      int    `json:"port"`
	DebugMode bool   `json:"debug_mode"`
}

func main() {
	fmt.Println("=== Coord 快速入门示例 ===")
	fmt.Println("本示例展示 coord 模块四大核心功能的基本用法")

	// 创建协调器
	cfg := coord.GetDefaultConfig("development")
	provider, err := coord.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("创建协调器失败: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()

	// 1. 分布式锁示例
	fmt.Println("\n--- 1. 分布式锁 ---")
	distributedLockDemo(ctx, provider)

	// 2. 配置中心示例
	fmt.Println("\n--- 2. 配置中心 ---")
	configCenterDemo(ctx, provider)

	// 3. 服务注册发现示例
	fmt.Println("\n--- 3. 服务注册发现 ---")
	serviceDiscoveryDemo(ctx, provider)

	// 4. ID生成器示例
	fmt.Println("\n--- 4. ID生成器 ---")
	idGeneratorDemo(ctx, provider)

	fmt.Println("\n=== 快速入门示例完成 ===")
}

// distributedLockDemo 演示分布式锁的基本用法
func distributedLockDemo(ctx context.Context, provider coord.Provider) {
	const lockKey = "getting-started-lock"

	// 获取分布式锁服务
	lockService := provider.Lock()

	// 非阻塞获取锁
	lock, err := lockService.TryAcquire(ctx, lockKey, 10*time.Second)
	if err != nil {
		fmt.Printf("非阻塞获取锁失败: %v\n", err)
		// 尝试阻塞获取
		fmt.Println("尝试阻塞获取锁...")
		lock, err = lockService.Acquire(ctx, lockKey, 10*time.Second)
		if err != nil {
			log.Fatalf("获取锁失败: %v", err)
		}
	}

	fmt.Printf("✓ 成功获取锁: %s\n", lock.Key())

	// 检查锁状态
	ttl, err := lock.TTL(ctx)
	if err == nil {
		fmt.Printf("  锁剩余TTL: %v\n", ttl)
	}

	// 释放锁
	if err := lock.Unlock(ctx); err != nil {
		log.Printf("释放锁失败: %v", err)
	} else {
		fmt.Println("  ✓ 锁已释放")
	}
}

// configCenterDemo 演示配置中心的基本用法
func configCenterDemo(ctx context.Context, provider coord.Provider) {
	const configKey = "getting-started/config"

	// 获取配置服务
	configService := provider.Config()

	// 设置配置
	appConfig := AppConfig{
		AppName:   "demo-app",
		Version:   "1.0.0",
		Port:      8080,
		DebugMode: true,
	}

	if err := configService.Set(ctx, configKey, appConfig); err != nil {
		log.Printf("设置配置失败: %v", err)
		return
	}
	fmt.Println("✓ 配置设置成功")

	// 获取配置
	var retrievedConfig AppConfig
	if err := configService.Get(ctx, configKey, &retrievedConfig); err != nil {
		log.Printf("获取配置失败: %v", err)
		return
	}

	fmt.Printf("  获取到配置: %+v\n", retrievedConfig)

	// 获取配置版本
	version, err := configService.GetWithVersion(ctx, configKey, &retrievedConfig)
	if err == nil {
		fmt.Printf("  配置版本: %d\n", version)
	}

	// 删除配置
	if err := configService.Delete(ctx, configKey); err != nil {
		log.Printf("删除配置失败: %v", err)
	} else {
		fmt.Println("  ✓ 配置已删除")
	}
}

// serviceDiscoveryDemo 演示服务注册发现的基本用法
func serviceDiscoveryDemo(ctx context.Context, provider coord.Provider) {
	serviceName := "demo-service"

	// 获取服务注册服务
	registryService := provider.Registry()

	// 注册服务实例
	serviceInfo := registry.ServiceInfo{
		ID:      "instance-1",
		Name:    serviceName,
		Address: "127.0.0.1",
		Port:    8080,
		Metadata: map[string]string{
			"version": "1.0.0",
			"region":  "local",
		},
	}

	if err := registryService.Register(ctx, serviceInfo, 30*time.Second); err != nil {
		log.Printf("注册服务失败: %v", err)
		return
	}
	fmt.Println("✓ 服务注册成功")

	// 发现服务
	services, err := registryService.Discover(ctx, serviceName)
	if err != nil {
		log.Printf("发现服务失败: %v", err)
		return
	}

	fmt.Printf("  发现 %d 个服务实例:\n", len(services))
	for _, svc := range services {
		fmt.Printf("    - ID: %s, Address: %s:%d\n", svc.ID, svc.Address, svc.Port)
	}

	// 注销服务
	if err := registryService.Unregister(ctx, serviceInfo.ID); err != nil {
		log.Printf("注销服务失败: %v", err)
	} else {
		fmt.Println("  ✓ 服务已注销")
	}
}

// idGeneratorDemo 演示ID生成器的基本用法
func idGeneratorDemo(ctx context.Context, provider coord.Provider) {
	// 获取ID分配器服务
	allocatorService, err := provider.InstanceIDAllocator("demo-service", 10)
	if err != nil {
		log.Printf("获取ID分配器失败: %v", err)
		return
	}

	// 分配实例ID
	allocatedID, err := allocatorService.AcquireID(ctx)
	if err != nil {
		log.Printf("分配ID失败: %v", err)
		return
	}
	defer allocatedID.Close(ctx)

	fmt.Printf("✓ 成功分配ID: %d\n", allocatedID.ID())

	// 模拟使用ID进行工作
	fmt.Printf("  实例 %d 正在工作...\n", allocatedID.ID())
	time.Sleep(100 * time.Millisecond)

	// 释放ID（会通过defer自动释放）
	fmt.Println("  ✓ ID 已释放（将在租约到期时自动清理）")
}
