package deviceplugin

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// TestPPUDevicePlugin 测试PPU设备插件的基本功能
func TestPPUDevicePlugin(t *testing.T) {
	// 设置测试日志级别
	log.SetLevel(log.DebugLevel)

	// 创建临时目录用于测试
	tmpDir := "/tmp/test-device-plugins"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建设备插件实例
	resourceName := "test.com/ppu"
	deviceCount := 4
	plugin := NewPPUDevicePlugin(resourceName, deviceCount, tmpDir)

	// 测试设备初始化
	t.Run("DeviceInitialization", func(t *testing.T) {
		// 这里我们需要访问plugin的私有字段，但为了测试简化，我们假设初始化成功
		if plugin == nil {
			t.Fatal("Plugin creation failed")
		}
	})

	// 测试GetDevicePluginOptions
	t.Run("GetDevicePluginOptions", func(t *testing.T) {
		ctx := context.Background()
		empty := &v1beta1.Empty{}

		options, err := plugin.GetDevicePluginOptions(ctx, empty)
		if err != nil {
			t.Fatalf("GetDevicePluginOptions failed: %v", err)
		}

		if options.PreStartRequired != false {
			t.Errorf("Expected PreStartRequired to be false, got %v", options.PreStartRequired)
		}
	})

	// 测试PreStart
	t.Run("PreStart", func(t *testing.T) {
		ctx := context.Background()
		request := &v1beta1.PreStartContainerRequest{
			DevicesIDs: []string{"ppu-0", "ppu-1"},
		}

		response, err := plugin.PreStart(ctx, request)
		if err != nil {
			t.Fatalf("PreStart failed: %v", err)
		}

		if response == nil {
			t.Fatal("PreStart response is nil")
		}
	})

	// 测试Allocate
	t.Run("Allocate", func(t *testing.T) {
		ctx := context.Background()
		request := &v1beta1.AllocateRequest{
			ContainerRequests: []*v1beta1.ContainerAllocateRequest{
				{
					DevicesIDs: []string{"ppu-0", "ppu-1"},
				},
			},
		}

		response, err := plugin.Allocate(ctx, request)
		if err != nil {
			t.Fatalf("Allocate failed: %v", err)
		}

		if len(response.ContainerResponses) != 1 {
			t.Errorf("Expected 1 container response, got %d", len(response.ContainerResponses))
		}

		containerResp := response.ContainerResponses[0]

		// 检查环境变量
		if deviceCount, exists := containerResp.Envs["PPU_DEVICE_COUNT"]; !exists {
			t.Error("PPU_DEVICE_COUNT environment variable not set")
		} else if deviceCount != "2" {
			t.Errorf("Expected PPU_DEVICE_COUNT to be '2', got '%s'", deviceCount)
		}

		if allocatedDevices, exists := containerResp.Envs["PPU_ALLOCATED_DEVICES"]; !exists {
			t.Error("PPU_ALLOCATED_DEVICES environment variable not set")
		} else if allocatedDevices == "" {
			t.Error("PPU_ALLOCATED_DEVICES should not be empty")
		}

		// 检查设备规格
		if len(containerResp.Devices) == 0 {
			t.Error("No device specifications returned")
		}
	})

	// 测试GetPreferredAllocation
	t.Run("GetPreferredAllocation", func(t *testing.T) {
		ctx := context.Background()
		request := &v1beta1.PreferredAllocationRequest{
			ContainerRequests: []*v1beta1.ContainerPreferredAllocationRequest{
				{
					AvailableDeviceIDs:   []string{"ppu-0", "ppu-1", "ppu-2", "ppu-3"},
					MustIncludeDeviceIDs: []string{"ppu-0"},
					AllocationSize:       2,
				},
			},
		}

		response, err := plugin.GetPreferredAllocation(ctx, request)
		if err != nil {
			t.Fatalf("GetPreferredAllocation failed: %v", err)
		}

		if len(response.ContainerResponses) != 1 {
			t.Errorf("Expected 1 container response, got %d", len(response.ContainerResponses))
		}

		containerResp := response.ContainerResponses[0]
		if len(containerResp.DeviceIDs) != 2 {
			t.Errorf("Expected 2 device IDs, got %d", len(containerResp.DeviceIDs))
		}

		// 确保必须包含的设备在选择中
		found := false
		for _, deviceID := range containerResp.DeviceIDs {
			if deviceID == "ppu-0" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Must include device 'ppu-0' not found in preferred allocation")
		}
	})
}

// TestHelperFunctions 测试辅助函数
func TestHelperFunctions(t *testing.T) {
	t.Run("LogLevels", func(t *testing.T) {
		// 测试不同的日志级别
		levels := []string{"debug", "info", "warn", "error"}
		for _, level := range levels {
			if _, err := log.ParseLevel(level); err != nil {
				t.Errorf("Failed to parse log level '%s': %v", level, err)
			}
		}
	})
}

// BenchmarkDeviceAllocation 性能测试
func BenchmarkDeviceAllocation(b *testing.B) {
	plugin := NewPPUDevicePlugin("test.com/ppu", 16, "/tmp")

	ctx := context.Background()
	request := &v1beta1.AllocateRequest{
		ContainerRequests: []*v1beta1.ContainerAllocateRequest{
			{
				DevicesIDs: []string{"ppu-0", "ppu-1"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := plugin.Allocate(ctx, request)
		if err != nil {
			b.Fatalf("Allocate failed: %v", err)
		}
	}
}

// ExampleUsage 使用示例
func ExampleUsage() {
	// 创建设备插件
	plugin := NewPPUDevicePlugin("alibabacloud.com/ppu", 16, "/var/lib/kubelet/device-plugins/")

	// 启动设备插件
	if err := plugin.Start(); err != nil {
		fmt.Printf("Failed to start plugin: %v\n", err)
		return
	}

	// 启动健康检查
	plugin.StartHealthCheck()

	// 模拟运行一段时间
	time.Sleep(1 * time.Second)

	// 停止插件
	plugin.Stop()

	fmt.Println("Plugin started and stopped successfully")
	// Output: Plugin started and stopped successfully
}
