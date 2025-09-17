package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ceyewan/infra-kit/coord"
	"github.com/ceyewan/infra-kit/coord/registry"
)

func main() {
	fmt.Println("=== 服务注册发现 - 基础用法 ===")
	fmt.Println("演示服务的注册、发现、注销等基本操作")

	// 创建协调器
	cfg := coord.GetDefaultConfig("development")
	provider, err := coord.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("创建协调器失败: %v", err)
	}
	defer provider.Close()

	registryService := provider.Registry()
	ctx := context.Background()

	// 1. 基本服务注册和发现
	basicRegistryDemo(ctx, registryService)

	// 2. 多实例服务管理
	multiInstanceDemo(ctx, registryService)

	// 3. 服务元数据管理
	metadataDemo(ctx, registryService)

	fmt.Println("\n=== 基础用法示例完成 ===")
}

// basicRegistryDemo 演示基本的服务注册和发现
func basicRegistryDemo(ctx context.Context, registryService registry.Registry) {
	fmt.Println("\n--- 基本服务注册和发现 ---")

	serviceName := "demo-service"

	// 创建服务实例信息
	serviceInfo := registry.ServiceInfo{
		ID:      "instance-1",
		Name:    serviceName,
		Address: "127.0.0.1",
		Port:    8080,
		Metadata: map[string]string{
			"version": "1.0.0",
			"region":  "local",
			"env":     "development",
		},
	}

	// 注册服务
	if err := registryService.Register(ctx, serviceInfo, 30*time.Second); err != nil {
		log.Printf("注册服务失败: %v", err)
		return
	}
	fmt.Printf("✓ 服务注册成功: %s/%s\n", serviceName, serviceInfo.ID)

	// 发现服务
	services, err := registryService.Discover(ctx, serviceName)
	if err != nil {
		log.Printf("发现服务失败: %v", err)
		return
	}

	fmt.Printf("  发现 %d 个服务实例:\n", len(services))
	for _, svc := range services {
		fmt.Printf("    - ID: %s, Address: %s:%d\n", svc.ID, svc.Address, svc.Port)
		fmt.Printf("      元数据: %v\n", svc.Metadata)
	}

	// 注销服务
	if err := registryService.Unregister(ctx, serviceInfo.ID); err != nil {
		log.Printf("注销服务失败: %v", err)
	} else {
		fmt.Printf("✓ 服务注销成功: %s/%s\n", serviceName, serviceInfo.ID)
	}

	// 验证注销
	services, err = registryService.Discover(ctx, serviceName)
	if err == nil && len(services) == 0 {
		fmt.Println("  ✓ 服务已成功注销，不再能被发现")
	}
}

// multiInstanceDemo 演示多实例服务管理
func multiInstanceDemo(ctx context.Context, registryService registry.Registry) {
	fmt.Println("\n--- 多实例服务管理 ---")

	serviceName := "multi-instance-service"

	// 创建多个服务实例
	instances := []registry.ServiceInfo{
		{
			ID:      "instance-1",
			Name:    serviceName,
			Address: "127.0.0.1",
			Port:    8081,
			Metadata: map[string]string{
				"version": "1.0.0",
				"zone":    "zone-a",
			},
		},
		{
			ID:      "instance-2",
			Name:    serviceName,
			Address: "127.0.0.1",
			Port:    8082,
			Metadata: map[string]string{
				"version": "1.0.0",
				"zone":    "zone-b",
			},
		},
		{
			ID:      "instance-3",
			Name:    serviceName,
			Address: "127.0.0.1",
			Port:    8083,
			Metadata: map[string]string{
				"version": "1.1.0",
				"zone":    "zone-a",
			},
		},
	}

	// 注册所有实例
	for _, instance := range instances {
		if err := registryService.Register(ctx, instance, 30*time.Second); err != nil {
			log.Printf("注册实例 %s 失败: %v", instance.ID, err)
			continue
		}
		fmt.Printf("✓ 实例 %s 注册成功\n", instance.ID)
	}

	// 发现所有实例
	services, err := registryService.Discover(ctx, serviceName)
	if err != nil {
		log.Printf("发现服务失败: %v", err)
		return
	}

	fmt.Printf("  发现 %d 个实例:\n", len(services))
	for _, svc := range services {
		fmt.Printf("    - %s:%d (版本: %s, 区域: %s)\n",
			svc.Address, svc.Port, svc.Metadata["version"], svc.Metadata["zone"])
	}

	// 按元数据过滤
	fmt.Println("\n  按版本过滤 (1.0.0):")
	for _, svc := range services {
		if svc.Metadata["version"] == "1.0.0" {
			fmt.Printf("    - %s:%d\n", svc.Address, svc.Port)
		}
	}

	// 注销所有实例
	for _, instance := range instances {
		if err := registryService.Unregister(ctx, instance.ID); err != nil {
			log.Printf("注销实例 %s 失败: %v", instance.ID, err)
			continue
		}
		fmt.Printf("✓ 实例 %s 注销成功\n", instance.ID)
	}
}

// metadataDemo 演示服务元数据管理
func metadataDemo(ctx context.Context, registryService registry.Registry) {
	fmt.Println("\n--- 服务元数据管理 ---")

	serviceName := "metadata-demo-service"

	// 创建带丰富元数据的服务实例
	serviceInfo := registry.ServiceInfo{
		ID:      "metadata-instance",
		Name:    serviceName,
		Address: "127.0.0.1",
		Port:    8090,
		Metadata: map[string]string{
			"version":      "2.0.0",
			"build":        "2024091701",
			"commit":       "abc123def",
			"environment":  "production",
			"region":       "us-east-1",
			"availability": "high",
			"owner":        "team-a",
			"tags":         "api,gateway",
		},
	}

	// 注册服务
	if err := registryService.Register(ctx, serviceInfo, 30*time.Second); err != nil {
		log.Printf("注册服务失败: %v", err)
		return
	}
	fmt.Printf("✓ 带元数据的服务注册成功: %s\n", serviceInfo.ID)

	// 发现服务并分析元数据
	services, err := registryService.Discover(ctx, serviceName)
	if err != nil || len(services) == 0 {
		log.Printf("发现服务失败或未找到服务: %v", err)
		return
	}

	svc := services[0]
	fmt.Println("  服务元数据:")
	for key, value := range svc.Metadata {
		fmt.Printf("    %s: %s\n", key, value)
	}

	// 演示基于元数据的查询
	fmt.Println("\n  基于元数据的条件检查:")

	// 检查版本
	if version, exists := svc.Metadata["version"]; exists {
		fmt.Printf("    服务版本: %s\n", version)
		if version == "2.0.0" {
			fmt.Println("    ✓ 版本符合预期")
		}
	}

	// 检查环境
	if env, exists := svc.Metadata["environment"]; exists {
		fmt.Printf("    运行环境: %s\n", env)
	}

	// 检查可用性级别
	if availability, exists := svc.Metadata["availability"]; exists {
		fmt.Printf("    可用性级别: %s\n", availability)
	}

	// 更新元数据（通过重新注册）
	updatedMetadata := svc.Metadata
	updatedMetadata["status"] = "healthy"
	updatedMetadata["last_updated"] = time.Now().Format(time.RFC3339)

	updatedService := registry.ServiceInfo{
		ID:       svc.ID,
		Name:     svc.Name,
		Address:  svc.Address,
		Port:     svc.Port,
		Metadata: updatedMetadata,
	}

	// 先注销再重新注册
	if err := registryService.Unregister(ctx, svc.ID); err != nil {
		log.Printf("注销服务失败: %v", err)
		return
	}

	if err := registryService.Register(ctx, updatedService, 30*time.Second); err != nil {
		log.Printf("重新注册服务失败: %v", err)
		return
	}
	fmt.Println("✓ 服务元数据更新成功")

	// 验证更新
	services, err = registryService.Discover(ctx, serviceName)
	if err == nil && len(services) > 0 {
		updatedSvc := services[0]
		if status, exists := updatedSvc.Metadata["status"]; exists {
			fmt.Printf("    更新后状态: %s\n", status)
		}
	}

	// 清理
	if err := registryService.Unregister(ctx, serviceInfo.ID); err != nil {
		log.Printf("注销服务失败: %v", err)
	} else {
		fmt.Println("✓ 服务清理完成")
	}
}

// leaseManagementDemo 演示租约管理
func leaseManagementDemo(ctx context.Context, registryService registry.Registry) {
	fmt.Println("\n--- 租约管理 ---")

	serviceName := "lease-demo-service"

	// 创建短租约的服务实例（5秒TTL）
	serviceInfo := registry.ServiceInfo{
		ID:      "lease-instance",
		Name:    serviceName,
		Address: "127.0.0.1",
		Port:    8091,
		Metadata: map[string]string{
			"lease_type": "short",
		},
	}

	// 注册短租约服务
	if err := registryService.Register(ctx, serviceInfo, 5*time.Second); err != nil {
		log.Printf("注册短租约服务失败: %v", err)
		return
	}
	fmt.Println("✓ 短租约服务注册成功 (TTL: 5秒)")

	// 立即发现服务
	services, err := registryService.Discover(ctx, serviceName)
	if err != nil {
		log.Printf("发现服务失败: %v", err)
		return
	}
	fmt.Printf("  初始发现 %d 个实例\n", len(services))

	// 等待租约过期
	fmt.Println("  等待租约过期...")
	time.Sleep(6 * time.Second)

	// 再次发现服务
	services, err = registryService.Discover(ctx, serviceName)
	if err != nil {
		log.Printf("发现服务失败: %v", err)
		return
	}
	fmt.Printf("  租约过期后发现 %d 个实例\n", len(services))

	if len(services) == 0 {
		fmt.Println("  ✓ 租约正确过期，服务实例已自动清理")
	} else {
		fmt.Println("  ✗ 租约过期但服务实例仍然存在")
	}
}

// errorHandlingDemo 演示错误处理
func errorHandlingDemo(ctx context.Context, registryService registry.Registry) {
	fmt.Println("\n--- 错误处理 ---")

	// 测试重复注册
	serviceName := "error-demo-service"
	serviceInfo := registry.ServiceInfo{
		ID:      "duplicate-instance",
		Name:    serviceName,
		Address: "127.0.0.1",
		Port:    8092,
	}

	// 第一次注册
	if err := registryService.Register(ctx, serviceInfo, 10*time.Second); err != nil {
		log.Printf("首次注册失败: %v", err)
		return
	}
	fmt.Println("✓ 首次注册成功")

	// 第二次注册相同ID（应该失败或覆盖）
	if err := registryService.Register(ctx, serviceInfo, 10*time.Second); err != nil {
		fmt.Printf("✓ 重复注册失败（符合预期）: %v\n", err)
	} else {
		fmt.Println("  重复注册成功（可能覆盖了之前的注册）")
	}

	// 测试注销不存在的服务
	if err := registryService.Unregister(ctx, "nonexistent-instance"); err != nil {
		fmt.Printf("✓ 注销不存在的服务失败（符合预期）: %v\n", err)
	} else {
		fmt.Println("  注销不存在的服务意外成功")
	}

	// 测试发现不存在的服务
	services, err := registryService.Discover(ctx, "nonexistent-service")
	if err != nil {
		fmt.Printf("✓ 发现不存在的服务失败（符合预期）: %v\n", err)
	} else if len(services) == 0 {
		fmt.Println("  ✓ 发现不存在的服务返回空列表（符合预期）")
	} else {
		fmt.Printf("  ✗ 发现不存在的服务返回 %d 个实例\n", len(services))
	}

	// 清理
	if err := registryService.Unregister(ctx, serviceInfo.ID); err != nil {
		log.Printf("清理失败: %v", err)
	} else {
		fmt.Println("✓ 清理完成")
	}
}
