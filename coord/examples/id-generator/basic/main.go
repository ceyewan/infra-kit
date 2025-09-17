package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ceyewan/infra-kit/coord"
	"github.com/ceyewan/infra-kit/coord/allocator"
)

func main() {
	fmt.Println("=== ID生成器 - 基础用法 ===")
	fmt.Println("演示实例ID的分配、使用和释放")

	// 创建协调器
	cfg := coord.GetDefaultConfig("development")
	provider, err := coord.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("创建协调器失败: %v", err)
	}
	defer provider.Close()

	allocatorService, err := provider.InstanceIDAllocator("demo-service", 10)
	if err != nil {
		log.Fatalf("获取ID分配器失败: %v", err)
	}
	ctx := context.Background()

	// 1. 基本ID分配和释放
	basicAllocationDemo(ctx, allocatorService)

	// 2. 容量管理和ID池
	capacityManagementDemo(ctx, allocatorService)

	// 3. 并发ID分配
	concurrentAllocationDemo(ctx, allocatorService)

	fmt.Println("\n=== 基础用法示例完成 ===")
}

// basicAllocationDemo 演示基本的ID分配和释放
func basicAllocationDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 基本ID分配和释放 ---")


	// 分配第一个ID
	id1, err := allocatorService.AcquireID(ctx)
	if err != nil {
		log.Printf("分配ID失败: %v", err)
		return
	}
	fmt.Printf("✓ 分配第一个ID: %d\n", id1.ID())

	// 分配第二个ID
	id2, err := allocatorService.AcquireID(ctx)
	if err != nil {
		log.Printf("分配ID失败: %v", err)
		id1.Close(ctx)
		return
	}
	fmt.Printf("✓ 分配第二个ID: %d\n", id2.ID())

	// 验证ID不重复
	if id1.ID() == id2.ID() {
		fmt.Println("  ✗ ID重复！")
	} else {
		fmt.Println("  ✓ ID不重复")
	}

	// 模拟使用ID进行工作
	fmt.Printf("  实例 %d 正在工作...\n", id1.ID())
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("  实例 %d 正在工作...\n", id2.ID())
	time.Sleep(100 * time.Millisecond)

	// 释放ID
	if err := id1.Close(ctx); err != nil {
		log.Printf("释放ID %d 失败: %v", id1.ID(), err)
	} else {
		fmt.Printf("✓ 释放ID %d 成功\n", id1.ID())
	}

	if err := id2.Close(ctx); err != nil {
		log.Printf("释放ID %d 失败: %v", id2.ID(), err)
	} else {
		fmt.Printf("✓ 释放ID %d 成功\n", id2.ID())
	}
}

// capacityManagementDemo 演示容量管理和ID池
func capacityManagementDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 容量管理和ID池 ---")

	poolSize := 3

	// 创建容量有限的ID分配器
	fmt.Printf("  创建容量为 %d 的ID池\n", poolSize)

	// 分配ID直到池满
	var allocatedIDs []allocator.AllocatedID
	for i := 0; i < poolSize; i++ {
		id, err := allocatorService.AcquireID(ctx)
		if err != nil {
			log.Printf("分配第 %d 个ID失败: %v", i+1, err)
			break
		}
		allocatedIDs = append(allocatedIDs, id)
		fmt.Printf("✓ 分配ID %d\n", id.ID())
	}

	// 尝试分配超过容量的ID（应该失败）
	extraID, err := allocatorService.AcquireID(ctx)
	if err != nil {
		fmt.Printf("✓ 分配超过容量的ID失败（符合预期）: %v\n", err)
	} else {
		fmt.Printf("✗ 意外分配了额外的ID: %d\n", extraID.ID())
		allocatedIDs = append(allocatedIDs, extraID)
	}

	// 释放一个ID
	if len(allocatedIDs) > 0 {
		releasedID := allocatedIDs[0]
		allocatedIDs = allocatedIDs[1:]

		if err := releasedID.Close(ctx); err != nil {
			log.Printf("释放ID失败: %v", err)
		} else {
			fmt.Printf("✓ 释放ID %d，现在池中有空位\n", releasedID.ID())
		}
	}

	// 现在应该能够分配新的ID
	newID, err := allocatorService.AcquireID(ctx)
	if err != nil {
		log.Printf("在有空位时分配ID失败: %v", err)
	} else {
		fmt.Printf("✓ 在有空位时成功分配ID: %d\n", newID.ID())
		allocatedIDs = append(allocatedIDs, newID)
	}

	// 清理所有ID
	for _, id := range allocatedIDs {
		if err := id.Close(ctx); err != nil {
			log.Printf("清理ID %d 失败: %v", id.ID(), err)
		}
	}
	fmt.Println("✓ 所有ID已清理")
}

// concurrentAllocationDemo 演示并发ID分配
func concurrentAllocationDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 并发ID分配 ---")

	const numWorkers = 10
	const poolSize = 5

	fmt.Printf("  启动 %d 个worker并发分配ID（池大小: %d）\n", numWorkers, poolSize)

	var wg sync.WaitGroup
	results := make(chan int, numWorkers)
	allocatedIDs := make(chan allocator.AllocatedID, numWorkers)

	// 启动worker并发分配ID
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// 尝试分配ID
			id, err := allocatorService.AcquireID(ctx)
			if err != nil {
				results <- -1 // -1表示失败
				return
			}

			// 成功分配ID
			results <- workerID
			allocatedIDs <- id

			// 模拟工作
			time.Sleep(time.Duration(100+workerID*10) * time.Millisecond)

			// 释放ID
			if err := id.Close(ctx); err != nil {
				log.Printf("Worker %d 释放ID失败: %v", workerID, err)
			}
		}(i)
	}

	// 等待所有worker完成
	go func() {
		wg.Wait()
		close(results)
		close(allocatedIDs)
	}()

	// 统计结果
	successCount := 0
	failureCount := 0
	usedIDs := make(map[int]bool)

	for result := range results {
		if result == -1 {
			failureCount++
		} else {
			successCount++
		}
	}

	// 检查分配的ID是否唯一
	for id := range allocatedIDs {
		if usedIDs[id.ID()] {
			fmt.Printf("  ✗ 发现重复ID: %d\n", id.ID())
		} else {
			usedIDs[id.ID()] = true
		}
	}

	fmt.Printf("  并发分配结果: 成功 %d, 失败 %d\n", successCount, failureCount)
	fmt.Printf("  分配的唯一ID数量: %d\n", len(usedIDs))

	if failureCount > 0 {
		fmt.Printf("  ✓ 部分worker因池满而失败（符合预期）\n")
	}
	if len(usedIDs) == successCount {
		fmt.Printf("  ✓ 所有成功分配的ID都是唯一的\n")
	}
}

// leaseManagementDemo 演示租约管理
func leaseManagementDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 租约管理 ---")

	// 分配一个短租约的ID
	fmt.Println("  分配短租约ID（5秒TTL）...")
	id, err := allocatorService.AcquireID(ctx)
	if err != nil {
		log.Printf("分配ID失败: %v", err)
		return
	}
	fmt.Printf("✓ 分配ID: %d\n", id.ID())

	// 检查ID状态
	fmt.Printf("  ID %d 状态正常\n", id.ID())

	// 等待租约过期
	fmt.Println("  等待租约过期...")
	time.Sleep(6 * time.Second)

	// 尝试使用已过期的ID（应该失败）
	fmt.Println("  尝试释放已过期的ID...")
	err = id.Close(ctx)
	if err != nil {
		fmt.Printf("✓ 释放过期ID失败（符合预期）: %v\n", err)
	} else {
		fmt.Println("  释放过期ID意外成功")
	}

	// 尝试分配新的ID
	newID, err := allocatorService.AcquireID(ctx)
	if err != nil {
		log.Printf("分配新ID失败: %v", err)
		return
	}
	fmt.Printf("✓ 过期后成功分配新ID: %d\n", newID.ID())

	// 清理
	if err := newID.Close(ctx); err != nil {
		log.Printf("清理失败: %v", err)
	} else {
		fmt.Println("✓ 清理完成")
	}
}

// errorHandlingDemo 演示错误处理
func errorHandlingDemo(ctx context.Context, allocatorService allocator.InstanceIDAllocator) {
	fmt.Println("\n--- 错误处理 ---")

	// 分配ID
	id, err := allocatorService.AcquireID(ctx)
	if err != nil {
		log.Printf("分配ID失败: %v", err)
		return
	}
	fmt.Printf("✓ 分配ID: %d\n", id.ID())

	// 释放ID
	if err := id.Close(ctx); err != nil {
		log.Printf("释放ID失败: %v", err)
		return
	}
	fmt.Println("✓ ID释放成功")

	// 尝试重复释放（应该失败）
	fmt.Println("  尝试重复释放...")
	err = id.Close(ctx)
	if err != nil {
		fmt.Printf("✓ 重复释放失败（符合预期）: %v\n", err)
	} else {
		fmt.Println("  重复释放意外成功")
	}

	// 分配多个ID直到池满
	fmt.Println("  测试池满情况...")
	var ids []allocator.AllocatedID
	for i := 0; i < 10; i++ {
		id, err := allocatorService.AcquireID(ctx)
		if err != nil {
			fmt.Printf("  第 %d 次分配失败（池满）: %v\n", i+1, err)
			break
		}
		ids = append(ids, id)
		fmt.Printf("  ✓ 分配ID %d\n", id.ID())
	}

	// 清理
	for _, id := range ids {
		if err := id.Close(ctx); err != nil {
			log.Printf("清理ID %d 失败: %v", id.ID(), err)
		}
	}
	fmt.Println("✓ 错误处理演示完成")
}
