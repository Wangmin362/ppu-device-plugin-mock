package deviceplugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	// PPU设备插件的Socket名称
	PPUSocket = "ppu.sock"
	// Kubelet设备插件注册Socket
	KubeletSocket = "kubelet.sock"
)

// PPUDevicePlugin 代表PPU设备插件
type PPUDevicePlugin struct {
	resourceName string
	deviceCount  int
	socketPath   string
	socket       string

	server  *grpc.Server
	devices map[string]*v1beta1.Device
	health  chan *v1beta1.Device
	stop    chan struct{}
}

// NewPPUDevicePlugin 创建新的PPU设备插件实例
func NewPPUDevicePlugin(resourceName string, deviceCount int, socketPath string) *PPUDevicePlugin {
	log.Debugf("Creating new PPU device plugin with resource name: %s, device count: %d", resourceName, deviceCount)

	return &PPUDevicePlugin{
		resourceName: resourceName,
		deviceCount:  deviceCount,
		socketPath:   socketPath,
		socket:       filepath.Join(socketPath, PPUSocket),
		devices:      make(map[string]*v1beta1.Device),
		health:       make(chan *v1beta1.Device),
		stop:         make(chan struct{}),
	}
}

// Start 启动设备插件
func (p *PPUDevicePlugin) Start() error {
	log.Info("Starting PPU device plugin")

	// 初始化模拟设备
	if err := p.initDevices(); err != nil {
		return fmt.Errorf("failed to initialize devices: %v", err)
	}

	// 启动gRPC服务器
	if err := p.serve(); err != nil {
		return fmt.Errorf("failed to start gRPC server: %v", err)
	}

	// 注册到kubelet
	if err := p.register(); err != nil {
		return fmt.Errorf("failed to register with kubelet: %v", err)
	}

	log.Info("PPU device plugin started successfully")
	return nil
}

// Stop 停止设备插件
func (p *PPUDevicePlugin) Stop() {
	log.Info("Stopping PPU device plugin")

	close(p.stop)

	if p.server != nil {
		p.server.Stop()
	}

	// 清理socket文件
	if err := os.Remove(p.socket); err != nil && !os.IsNotExist(err) {
		log.Warnf("Failed to remove socket file: %v", err)
	}

	log.Info("PPU device plugin stopped")
}

// initDevices 初始化模拟PPU设备
func (p *PPUDevicePlugin) initDevices() error {
	log.Infof("Initializing %d PPU devices", p.deviceCount)

	for i := 0; i < p.deviceCount; i++ {
		deviceID := fmt.Sprintf("ppu-%d", i)
		device := &v1beta1.Device{
			ID:     deviceID,
			Health: v1beta1.Healthy,
		}

		p.devices[deviceID] = device
		log.Debugf("Initialized PPU device: %s", deviceID)
	}

	log.Infof("Successfully initialized %d PPU devices", len(p.devices))
	return nil
}

// serve 启动gRPC服务器
func (p *PPUDevicePlugin) serve() error {
	log.Debugf("Starting gRPC server on socket: %s", p.socket)

	// 确保socket目录存在
	if err := os.MkdirAll(filepath.Dir(p.socket), 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %v", err)
	}

	// 删除已存在的socket文件
	if err := os.Remove(p.socket); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %v", err)
	}

	// 创建Unix socket监听器
	listener, err := net.Listen("unix", p.socket)
	if err != nil {
		return fmt.Errorf("failed to listen on socket %s: %v", p.socket, err)
	}

	// 创建gRPC服务器
	p.server = grpc.NewServer([]grpc.ServerOption{}...)
	v1beta1.RegisterDevicePluginServer(p.server, p)

	// 在后台启动服务器
	go func() {
		log.Debugf("gRPC server listening on socket: %s", p.socket)
		if err := p.server.Serve(listener); err != nil {
			log.Errorf("gRPC server failed: %v", err)
		}
	}()

	// 等待服务器启动
	conn, err := p.dial(p.socket, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	conn.Close()

	log.Info("gRPC server started successfully")
	return nil
}

// dial 连接到Unix socket
func (p *PPUDevicePlugin) dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

// register 向kubelet注册设备插件
func (p *PPUDevicePlugin) register() error {
	log.Info("Registering PPU device plugin with kubelet")

	kubeletSocket := filepath.Join(p.socketPath, KubeletSocket)
	conn, err := p.dial(kubeletSocket, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to kubelet: %v", err)
	}
	defer conn.Close()

	client := v1beta1.NewRegistrationClient(conn)

	request := &v1beta1.RegisterRequest{
		Version:      v1beta1.Version,
		Endpoint:     PPUSocket,
		ResourceName: p.resourceName,
	}

	log.Debugf("Sending registration request: %+v", request)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Register(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to register device plugin: %v", err)
	}

	log.Infof("Successfully registered PPU device plugin with resource name: %s", p.resourceName)
	return nil
}

// StartHealthCheck 启动设备健康检查
func (p *PPUDevicePlugin) StartHealthCheck() {
	p.startHealthCheck()
}
