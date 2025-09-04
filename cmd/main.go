package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/wangmin362/ppu-device-plugin/pkg/deviceplugin"
)

var (
	resourceName = flag.String("resource-name", "alibabacloud.com/ppu", "Resource name for the device plugin")
	deviceCount  = flag.Int("device-count", 16, "Number of PPU devices to simulate")
	logLevel     = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	socketPath   = flag.String("socket-path", "/var/lib/kubelet/device-plugins/", "Path for device plugin socket")
)

func main() {
	flag.Parse()

	// 配置日志级别
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatalf("Invalid log level: %s", *logLevel)
	}
	log.SetLevel(level)

	// 配置日志格式
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	log.Infof("Starting PPU Device Plugin")
	log.Infof("Resource Name: %s", *resourceName)
	log.Infof("Device Count: %d", *deviceCount)
	log.Infof("Log Level: %s", *logLevel)
	log.Infof("Socket Path: %s", *socketPath)

	// 创建设备插件实例
	plugin := deviceplugin.NewPPUDevicePlugin(*resourceName, *deviceCount, *socketPath)

	// 启动设备插件
	if err := plugin.Start(); err != nil {
		log.Fatalf("Failed to start device plugin: %v", err)
	}

	// 启动健康检查
	plugin.StartHealthCheck()

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("PPU Device Plugin is running...")
	<-sigChan

	log.Info("Shutting down PPU Device Plugin...")
	plugin.Stop()
}
