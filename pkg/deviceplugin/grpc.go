package deviceplugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// GetDevicePluginOptions 返回设备插件选项
func (p *PPUDevicePlugin) GetDevicePluginOptions(ctx context.Context, empty *v1beta1.Empty) (*v1beta1.DevicePluginOptions, error) {
	log.Debug("GetDevicePluginOptions called")

	options := &v1beta1.DevicePluginOptions{
		PreStartRequired: false,
	}

	log.Debugf("Returning device plugin options: %+v", options)
	return options, nil
}

// ListAndWatch 返回设备列表，并监听设备状态变化
func (p *PPUDevicePlugin) ListAndWatch(empty *v1beta1.Empty, stream v1beta1.DevicePlugin_ListAndWatchServer) error {
	log.Info("ListAndWatch called - starting device monitoring")

	// 发送初始设备列表
	devices := make([]*v1beta1.Device, 0, len(p.devices))
	for _, device := range p.devices {
		devices = append(devices, device)
	}

	response := &v1beta1.ListAndWatchResponse{
		Devices: devices,
	}

	log.Debugf("Sending initial device list with %d devices", len(devices))
	if err := stream.Send(response); err != nil {
		log.Errorf("Failed to send initial device list: %v", err)
		return err
	}

	log.Info("Initial device list sent successfully")

	// 持续监听设备状态变化和健康检查
	for {
		select {
		case device := <-p.health:
			log.Debugf("Device health update received: %s, health: %s", device.ID, device.Health)

			// 更新设备状态
			if existingDevice, exists := p.devices[device.ID]; exists {
				existingDevice.Health = device.Health
				log.Debugf("Updated device %s health to: %s", device.ID, device.Health)
			}

			// 发送更新后的设备列表
			updatedDevices := make([]*v1beta1.Device, 0, len(p.devices))
			for _, dev := range p.devices {
				updatedDevices = append(updatedDevices, dev)
			}

			updateResponse := &v1beta1.ListAndWatchResponse{
				Devices: updatedDevices,
			}

			if err := stream.Send(updateResponse); err != nil {
				log.Errorf("Failed to send device list update: %v", err)
				return err
			}

			log.Debugf("Device list update sent successfully")

		case <-p.stop:
			log.Info("ListAndWatch stopped")
			return nil
		}
	}
}

// Allocate 分配设备给Pod
func (p *PPUDevicePlugin) Allocate(ctx context.Context, request *v1beta1.AllocateRequest) (*v1beta1.AllocateResponse, error) {
	log.Infof("Allocate called with %d container requests", len(request.ContainerRequests))

	responses := make([]*v1beta1.ContainerAllocateResponse, 0, len(request.ContainerRequests))

	for i, containerRequest := range request.ContainerRequests {
		log.Debugf("Processing container request %d with %d device IDs: %v",
			i, len(containerRequest.DevicesIDs), containerRequest.DevicesIDs)

		// 验证请求的设备是否存在且健康
		allocatedDevices := []string{}
		for _, deviceID := range containerRequest.DevicesIDs {
			if device, exists := p.devices[deviceID]; exists {
				if device.Health == v1beta1.Healthy {
					allocatedDevices = append(allocatedDevices, deviceID)
					log.Debugf("Device %s allocated successfully", deviceID)
				} else {
					log.Warnf("Device %s is not healthy, health status: %s", deviceID, device.Health)
				}
			} else {
				log.Warnf("Requested device %s not found", deviceID)
			}
		}

		// 构建容器分配响应
		containerResponse := &v1beta1.ContainerAllocateResponse{
			Envs: map[string]string{
				"PPU_DEVICE_COUNT":      fmt.Sprintf("%d", len(allocatedDevices)),
				"PPU_ALLOCATED_DEVICES": strings.Join(allocatedDevices, ","),
			},
			Mounts:  []*v1beta1.Mount{},
			Devices: []*v1beta1.DeviceSpec{},
			Annotations: map[string]string{
				"ppu.alibabacloud.com/allocated-devices": strings.Join(allocatedDevices, ","),
			},
		}

		// 为每个分配的设备添加设备规格（模拟设备文件）
		for _, deviceID := range allocatedDevices {
			deviceSpec := &v1beta1.DeviceSpec{
				ContainerPath: "/dev/" + deviceID,
				HostPath:      "/dev/null", // 模拟设备，使用/dev/null
				Permissions:   "rw",
			}
			containerResponse.Devices = append(containerResponse.Devices, deviceSpec)
			log.Debugf("Added device spec for %s: %s -> %s", deviceID, deviceSpec.HostPath, deviceSpec.ContainerPath)
		}

		responses = append(responses, containerResponse)
		log.Infof("Container request %d processed: allocated %d devices", i, len(allocatedDevices))
	}

	allocateResponse := &v1beta1.AllocateResponse{
		ContainerResponses: responses,
	}

	log.Infof("Allocate completed: returning %d container responses", len(responses))
	return allocateResponse, nil
}

// GetPreferredAllocation 返回首选的设备分配
func (p *PPUDevicePlugin) GetPreferredAllocation(ctx context.Context, request *v1beta1.PreferredAllocationRequest) (*v1beta1.PreferredAllocationResponse, error) {
	log.Debugf("GetPreferredAllocation called with %d container requests", len(request.ContainerRequests))

	responses := make([]*v1beta1.ContainerPreferredAllocationResponse, 0, len(request.ContainerRequests))

	for i, containerRequest := range request.ContainerRequests {
		log.Debugf("Processing preferred allocation for container %d, requested: %d, available: %d",
			i, containerRequest.MustIncludeDeviceIDs, len(containerRequest.AvailableDeviceIDs))

		// 简单的分配策略：优先选择前面的设备
		selectedDeviceIDs := []string{}

		// 首先包含必须包含的设备
		for _, deviceID := range containerRequest.MustIncludeDeviceIDs {
			selectedDeviceIDs = append(selectedDeviceIDs, deviceID)
		}

		// 然后从可用设备中选择剩余需要的设备
		needed := int(containerRequest.AllocationSize) - len(selectedDeviceIDs)
		for _, deviceID := range containerRequest.AvailableDeviceIDs {
			if needed <= 0 {
				break
			}

			// 检查设备是否已经在必须包含的列表中
			alreadySelected := false
			for _, selectedID := range selectedDeviceIDs {
				if selectedID == deviceID {
					alreadySelected = true
					break
				}
			}

			if !alreadySelected {
				selectedDeviceIDs = append(selectedDeviceIDs, deviceID)
				needed--
			}
		}

		containerResponse := &v1beta1.ContainerPreferredAllocationResponse{
			DeviceIDs: selectedDeviceIDs,
		}

		responses = append(responses, containerResponse)
		log.Debugf("Preferred allocation for container %d: selected %v", i, selectedDeviceIDs)
	}

	preferredResponse := &v1beta1.PreferredAllocationResponse{
		ContainerResponses: responses,
	}

	log.Debugf("GetPreferredAllocation completed: returning %d container responses", len(responses))
	return preferredResponse, nil
}

// PreStart 在容器启动前执行的钩子函数
func (p *PPUDevicePlugin) PreStart(ctx context.Context, request *v1beta1.PreStartContainerRequest) (*v1beta1.PreStartContainerResponse, error) {
	log.Debugf("PreStart called for %d devices", len(request.DevicesIDs))

	// 对于模拟设备，我们不需要执行实际的预启动操作
	// 只是记录日志以便调试
	for _, deviceID := range request.DevicesIDs {
		log.Debugf("PreStart processing device: %s", deviceID)
	}

	response := &v1beta1.PreStartContainerResponse{}
	log.Debug("PreStart completed successfully")

	return response, nil
}

func (p *PPUDevicePlugin) PreStartContainer(ctx context.Context, request *v1beta1.PreStartContainerRequest) (*v1beta1.PreStartContainerResponse, error) {
	log.Debugf("PreStartContainer called for %d devices", len(request.DevicesIDs))
	return &v1beta1.PreStartContainerResponse{}, nil
}

// startHealthCheck 启动设备健康检查
func (p *PPUDevicePlugin) startHealthCheck() {
	log.Info("Starting device health check routine")

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Debug("Performing periodic health check")

				// 模拟设备健康检查
				for deviceID, device := range p.devices {
					// 在真实环境中，这里会检查实际的设备状态
					// 对于模拟设备，我们假设所有设备都是健康的
					if device.Health != v1beta1.Healthy {
						log.Debugf("Device %s health check: changing from %s to Healthy", deviceID, device.Health)
						device.Health = v1beta1.Healthy

						// 发送健康状态更新
						select {
						case p.health <- device:
							log.Debugf("Health update sent for device %s", deviceID)
						default:
							log.Debugf("Health channel full, skipping update for device %s", deviceID)
						}
					}
				}

			case <-p.stop:
				log.Info("Health check routine stopped")
				return
			}
		}
	}()
}
