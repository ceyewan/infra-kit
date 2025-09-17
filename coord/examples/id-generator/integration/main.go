package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ceyewan/infra-kit/coord"
	"github.com/ceyewan/infra-kit/coord/allocator"
	"github.com/ceyewan/infra-kit/coord/lock"
	"github.com/ceyewan/infra-kit/coord/registry"
)

func main() {
	fmt.Println("=== ID生成器 - 集成使用 ===")
	fmt.Println("演示ID生成器与其他coord模块的集成使用")

	// 创建协调器
	cfg := coord.GetDefaultConfig("development")
	provider, err := coord.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("创建协调器失败: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()

	// 1. 与分布式锁集成
	integrationWithLock(ctx, provider)

	// 2. 与配置中心集成
	integrationWithConfig(ctx, provider)

	// 3. 微服务实例管理集成
	integrationWithServiceManagement(ctx, provider)

	// 4. 分布式任务调度集成
	integrationWithTaskScheduling(ctx, provider)

	// 5. 集群管理集成
	integrationWithClusterManagement(ctx, provider)

	fmt.Println("\n=== 集成使用演示完成 ===")
}

// integrationWithLock 演示与分布式锁的集成
func integrationWithLock(ctx context.Context, provider coord.Provider) {
	fmt.Println("\n--- 与分布式锁集成 ---")

	// 获取服务
	allocatorService, err := provider.InstanceIDAllocator("lock-integration-service", 5)
	if err != nil {
		log.Fatalf("获取ID分配器失败: %v", err)
	}

	lockService := provider.Lock()

	// 场景：为每个需要锁保护的资源分配唯一ID
	type ProtectedResource struct {
		ResourceID string
		Lock       lock.Lock
		OwnerID    int
	}

	resources := []string{"resource-1", "resource-2", "resource-3"}
	var protectedResources []*ProtectedResource
	var mu sync.Mutex

	// 为每个资源分配ID和锁
	for _, resourceID := range resources {
		// 获取实例ID
		instanceID, err := allocatorService.AcquireID(ctx)
		if err != nil {
			log.Printf("为资源 %s 分配ID失败: %v", resourceID, err)
			continue
		}

		// 获取资源锁
		resourceLock, err := lockService.Acquire(ctx, resourceID, 10*time.Second)
		if err != nil {
			log.Printf("为资源 %s 获取锁失败: %v", resourceID, err)
			instanceID.Close(ctx)
			continue
		}

		protected := &ProtectedResource{
			ResourceID: resourceID,
			Lock:       resourceLock,
			OwnerID:    instanceID.ID(),
		}

		mu.Lock()
		protectedResources = append(protectedResources, protected)
		mu.Unlock()

		fmt.Printf("  资源 %s 受保护，OwnerID: %d\n", resourceID, instanceID.ID())
	}

	// 模拟使用受保护的资源
	for _, resource := range protectedResources {
		fmt.Printf("  处理资源 %s (Owner: %d)\n", resource.ResourceID, resource.OwnerID)
		time.Sleep(50 * time.Millisecond)
	}

	// 清理资源
	for _, resource := range protectedResources {
		resource.Lock.Unlock(ctx)
		// 注意：在实际应用中，我们需要跟踪对应的 AllocatedID 来释放
		// 这里简化处理
	}

	fmt.Printf("✓ 与分布式锁集成完成: 保护了 %d 个资源\n", len(resources))
}

// integrationWithConfig 演示与配置中心的集成
func integrationWithConfig(ctx context.Context, provider coord.Provider) {
	fmt.Println("\n--- 与配置中心集成 ---")

	// 获取服务
	allocatorService, err := provider.InstanceIDAllocator("config-integration-service", 3)
	if err != nil {
		log.Fatalf("获取ID分配器失败: %v", err)
	}

	configService := provider.Config()

	// 场景：每个实例ID对应不同的配置
	type InstanceConfig struct {
		InstanceID int    `json:"instance_id"`
		Port       int    `json:"port"`
		DataDir    string `json:"data_dir"`
		Enabled    bool   `json:"enabled"`
	}

	// 为每个实例ID创建配置
	instanceConfigs := map[int]InstanceConfig{
		1: {InstanceID: 1, Port: 8081, DataDir: "/data/instance1", Enabled: true},
		2: {InstanceID: 2, Port: 8082, DataDir: "/data/instance2", Enabled: true},
		3: {InstanceID: 3, Port: 8083, DataDir: "/data/instance3", Enabled: false},
	}

	// 存储配置到配置中心
	for instanceID, config := range instanceConfigs {
		configKey := fmt.Sprintf("instances/config/%d", instanceID)
		err := configService.Set(ctx, configKey, config)
		if err != nil {
			log.Printf("存储实例 %d 配置失败: %v", instanceID, err)
			continue
		}
		fmt.Printf("  存储实例 %d 配置: %+v\n", instanceID, config)
	}

	// 模拟实例启动并获取配置
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(workerNum int) {
			defer wg.Done()

			// 获取实例ID
			instanceIDAlloc, err := allocatorService.AcquireID(ctx)
			if err != nil {
				log.Printf("Worker %d 获取实例ID失败: %v", workerNum, err)
				return
			}
			defer instanceIDAlloc.Close(ctx)

			instanceID := instanceIDAlloc.ID()
			fmt.Printf("  Worker %d 获得实例ID: %d\n", workerNum, instanceID)

			// 获取对应的配置
			configKey := fmt.Sprintf("instances/config/%d", instanceID)
			var config InstanceConfig
			err = configService.Get(ctx, configKey, &config)
			if err != nil {
				log.Printf("Worker %d 获取配置失败: %v", workerNum, err)
				return
			}

			fmt.Printf("  Worker %d 使用配置: 端口 %d, 数据目录 %s\n",
				workerNum, config.Port, config.DataDir)

			// 模拟工作
			time.Sleep(100 * time.Millisecond)
		}(i + 1)
	}

	wg.Wait()
	fmt.Printf("✓ 与配置中心集成完成\n")
}

// integrationWithServiceManagement 演示微服务实例管理集成
func integrationWithServiceManagement(ctx context.Context, provider coord.Provider) {
	fmt.Println("\n--- 微服务实例管理集成 ---")

	// 获取服务
	allocatorService, err := provider.InstanceIDAllocator("service-management", 5)
	if err != nil {
		log.Fatalf("获取ID分配器失败: %v", err)
	}

	registryService := provider.Registry()

	// 场景：管理微服务实例的生命周期
	type ServiceInstance struct {
		InstanceID    int
		ServiceName   string
		IPAddress     string
		Port          int
		AllocatedID   allocator.AllocatedID
		HealthStatus  string
		LastHeartbeat time.Time
	}

	instances := make(map[int]*ServiceInstance)
	var mu sync.Mutex

	// 启动实例管理器
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mu.Lock()
				// 检查实例健康状态
				for id, instance := range instances {
					if time.Since(instance.LastHeartbeat) > 5*time.Second {
						instance.HealthStatus = "unhealthy"
						fmt.Printf("  实例 %d 状态不健康\n", id)
					}
				}
				mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	// 模拟实例注册
	const numInstances = 3
	for i := 0; i < numInstances; i++ {
		// 获取实例ID
		allocatedID, err := allocatorService.AcquireID(ctx)
		if err != nil {
			log.Printf("获取实例ID失败: %v", err)
			continue
		}

		instanceID := allocatedID.ID()

		// 创建服务实例
		instance := &ServiceInstance{
			InstanceID:    instanceID,
			ServiceName:   "user-service",
			IPAddress:     "127.0.0.1",
			Port:          8080 + instanceID,
			AllocatedID:   allocatedID,
			HealthStatus:  "healthy",
			LastHeartbeat: time.Now(),
		}

		// 注册到服务发现
		serviceInfo := registry.ServiceInfo{
			ID:      fmt.Sprintf("user-service-%d", instanceID),
			Name:    instance.ServiceName,
			Address: instance.IPAddress,
			Port:    instance.Port,
			Metadata: map[string]string{
				"instance_id": fmt.Sprintf("%d", instanceID),
				"status":      instance.HealthStatus,
			},
		}

		err = registryService.Register(ctx, serviceInfo, 30*time.Second)
		if err != nil {
			log.Printf("注册实例 %d 失败: %v", instanceID, err)
			allocatedID.Close(ctx)
			continue
		}

		mu.Lock()
		instances[instanceID] = instance
		mu.Unlock()

		fmt.Printf("  实例 %d 注册成功，端口: %d\n", instanceID, instance.Port)
	}

	// 模拟心跳更新
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for i := 0; i < 5; i++ {
			select {
			case <-ticker.C:
				mu.Lock()
				for _, instance := range instances {
					instance.LastHeartbeat = time.Now()
					instance.HealthStatus = "healthy"
					fmt.Printf("  实例 %d 心跳更新\n", instance.InstanceID)
				}
				mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	// 等待一段时间观察
	time.Sleep(3 * time.Second)

	// 清理实例
	mu.Lock()
	for id, instance := range instances {
		// 注销服务
		err := registryService.Unregister(ctx, fmt.Sprintf("user-service-%d", id))
		if err != nil {
			log.Printf("注销实例 %d 失败: %v", id, err)
		}

		// 释放ID
		instance.AllocatedID.Close(ctx)
		fmt.Printf("  实例 %d 已清理\n", id)
	}
	mu.Unlock()

	fmt.Printf("✓ 微服务实例管理集成完成: 管理了 %d 个实例\n", numInstances)
}

// integrationWithTaskScheduling 演示分布式任务调度集成
func integrationWithTaskScheduling(ctx context.Context, provider coord.Provider) {
	fmt.Println("\n--- 分布式任务调度集成 ---")

	// 获取服务
	allocatorService, err := provider.InstanceIDAllocator("task-scheduler", 3)
	if err != nil {
		log.Fatalf("获取ID分配器失败: %v", err)
	}

	lockService := provider.Lock()

	// 场景：分布式任务调度，每个worker分配唯一ID
	type Task struct {
		ID         int
		Type       string
		Payload    string
		Status     string
		AssignedTo int
	}

	type Worker struct {
		ID          int
		AllocatedID allocator.AllocatedID
		Active      bool
		Processed   int
	}

	// 任务队列
	tasks := []Task{
		{ID: 1, Type: "email", Payload: "hello@example.com", Status: "pending"},
		{ID: 2, Type: "report", Payload: "daily-report", Status: "pending"},
		{ID: 3, Type: "cleanup", Payload: "temp-files", Status: "pending"},
		{ID: 4, Type: "backup", Payload: "database", Status: "pending"},
		{ID: 5, Type: "notification", Payload: "alert", Status: "pending"},
	}

	workers := make(map[int]*Worker)
	var mu sync.Mutex

	// 创建worker
	const numWorkers = 2
	for i := 0; i < numWorkers; i++ {
		allocatedID, err := allocatorService.AcquireID(ctx)
		if err != nil {
			log.Printf("创建worker %d 失败: %v", i+1, err)
			continue
		}

		worker := &Worker{
			ID:          allocatedID.ID(),
			AllocatedID: allocatedID,
			Active:      true,
		}

		mu.Lock()
		workers[worker.ID] = worker
		mu.Unlock()

		fmt.Printf("  Worker %d 已创建\n", worker.ID)
	}

	// 任务调度器
	go func() {
		for _, task := range tasks {
			// 获取任务分配锁
			taskLock, err := lockService.Acquire(ctx, fmt.Sprintf("task-lock-%d", task.ID), 5*time.Second)
			if err != nil {
				fmt.Printf("  任务 %d 分配失败: %v\n", task.ID, err)
				continue
			}

			// 分配任务给可用的worker
			mu.Lock()
			for _, worker := range workers {
				if worker.Active {
					task.AssignedTo = worker.ID
					task.Status = "assigned"
					worker.Processed++
					fmt.Printf("  任务 %d 分配给 worker %d\n", task.ID, worker.ID)
					break
				}
			}
			mu.Unlock()

			taskLock.Unlock(ctx)
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// 模拟worker处理任务
	time.Sleep(3 * time.Second)

	// 清理worker
	mu.Lock()
	for _, worker := range workers {
		worker.Active = false
		worker.AllocatedID.Close(ctx)
		fmt.Printf("  Worker %d 已停止，处理了 %d 个任务\n", worker.ID, worker.Processed)
	}
	mu.Unlock()

	fmt.Printf("✓ 分布式任务调度集成完成\n")
}

// integrationWithClusterManagement 演示集群管理集成
func integrationWithClusterManagement(ctx context.Context, provider coord.Provider) {
	fmt.Println("\n--- 集群管理集成 ---")

	// 获取服务
	allocatorService, err := provider.InstanceIDAllocator("cluster-management", 5)
	if err != nil {
		log.Fatalf("获取ID分配器失败: %v", err)
	}

	// 场景：集群节点管理
	type ClusterNode struct {
		NodeID      int
		Role        string
		Status      string
		AllocatedID allocator.AllocatedID
		LastSeen    time.Time
	}

	nodes := make(map[int]*ClusterNode)
	var mu sync.RWMutex

	// 模拟节点加入集群
	const numNodes = 3
	for i := 0; i < numNodes; i++ {
		// 获取节点ID
		allocatedID, err := allocatorService.AcquireID(ctx)
		if err != nil {
			log.Printf("节点 %d 获取ID失败: %v", i+1, err)
			continue
		}

		nodeID := allocatedID.ID()
		roles := []string{"leader", "follower", "candidate"}
		role := roles[i%len(roles)]

		node := &ClusterNode{
			NodeID:      nodeID,
			Role:        role,
			Status:      "active",
			AllocatedID: allocatedID,
			LastSeen:    time.Now(),
		}

		mu.Lock()
		nodes[nodeID] = node
		mu.Unlock()

		fmt.Printf("  节点 %d 加入集群，角色: %s\n", nodeID, role)
	}

	// 集群健康检查
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for i := 0; i < 3; i++ {
			select {
			case <-ticker.C:
				mu.RLock()
				activeNodes := 0
				for _, node := range nodes {
					if node.Status == "active" {
						activeNodes++
					}
				}
				mu.RUnlock()

				fmt.Printf("  集群状态: %d/%d 节点活跃\n", activeNodes, numNodes)
			case <-ctx.Done():
				return
			}
		}
	}()

	// 模拟leader选举
	go func() {
		time.Sleep(1 * time.Second)

		mu.Lock()
		defer mu.Unlock()

		// 找到最小的活跃节点作为leader
		var minNodeID int
		found := false
		for _, node := range nodes {
			if node.Status == "active" {
				if !found || node.NodeID < minNodeID {
					minNodeID = node.NodeID
					found = true
				}
			}
		}

		if found {
			// 更新节点角色
			for _, node := range nodes {
				if node.NodeID == minNodeID {
					node.Role = "leader"
					fmt.Printf("  节点 %d 被选为 leader\n", node.NodeID)
				} else if node.Status == "active" {
					node.Role = "follower"
				}
			}
		}
	}()

	// 等待观察
	time.Sleep(2 * time.Second)

	// 清理节点
	mu.Lock()
	for _, node := range nodes {
		node.Status = "inactive"
		node.AllocatedID.Close(ctx)
		fmt.Printf("  节点 %d 离开集群\n", node.NodeID)
	}
	mu.Unlock()

	fmt.Printf("✓ 集群管理集成完成: 管理了 %d 个节点\n", numNodes)
}
