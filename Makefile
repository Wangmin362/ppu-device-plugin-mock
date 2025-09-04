# 变量定义
REGISTRY ?= registry.cn-hangzhou.aliyuncs.com/your-namespace
IMAGE_NAME ?= ppu-device-plugin
TAG ?= v1.0.0
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):$(TAG)

# Go相关变量
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# 二进制文件名
BINARY_NAME = ppu-device-plugin
BINARY_PATH = ./bin/$(BINARY_NAME)

# K8S相关变量
NAMESPACE ?= kube-system
KUBECTL = kubectl

# 默认目标
.PHONY: all
all: clean deps build

# 帮助信息
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Download dependencies"
	@echo "  test           - Run tests"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-push    - Push Docker image to registry"
	@echo "  deploy         - Deploy to Kubernetes"
	@echo "  undeploy       - Remove from Kubernetes"
	@echo "  logs          - Show pod logs"
	@echo "  all           - Clean, deps, and build"
	@echo ""
	@echo "Variables:"
	@echo "  REGISTRY      - Docker registry (default: $(REGISTRY))"
	@echo "  TAG           - Image tag (default: $(TAG))"
	@echo "  NAMESPACE     - K8s namespace (default: $(NAMESPACE))"

# 下载依赖
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# 构建二进制文件
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -installsuffix cgo -ldflags '-w -s' -o $(BINARY_PATH) ./cmd/main.go
	@echo "Binary built: $(BINARY_PATH)"

# 本地构建（当前平台）
.PHONY: build-local
build-local:
	@echo "Building $(BINARY_NAME) for local platform..."
	@mkdir -p bin
	$(GOBUILD) -o $(BINARY_PATH) ./cmd/main.go
	@echo "Local binary built: $(BINARY_PATH)"

# 运行测试
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# 清理构建产物
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf bin/
	@rm -f $(BINARY_NAME)

# 构建Docker镜像
.PHONY: docker-build
docker-build:
	@echo "Building Docker image: $(FULL_IMAGE_NAME)"
	docker build -t $(FULL_IMAGE_NAME) .
	@echo "Docker image built successfully"

# 推送Docker镜像
.PHONY: docker-push
docker-push: docker-build
	@echo "Pushing Docker image: $(FULL_IMAGE_NAME)"
	docker push $(FULL_IMAGE_NAME)
	@echo "Docker image pushed successfully"

# 部署到Kubernetes
.PHONY: deploy
deploy:
	@echo "Deploying to Kubernetes namespace: $(NAMESPACE)"
	$(KUBECTL) apply -f deployments/namespace.yaml
	$(KUBECTL) apply -f deployments/rbac.yaml
	@sed 's|IMAGE_PLACEHOLDER|$(FULL_IMAGE_NAME)|g' deployments/daemonset.yaml | $(KUBECTL) apply -f -
	@echo "Deployment completed"

# 从Kubernetes删除
.PHONY: undeploy
undeploy:
	@echo "Removing from Kubernetes namespace: $(NAMESPACE)"
	$(KUBECTL) delete -f deployments/daemonset.yaml --ignore-not-found=true
	$(KUBECTL) delete -f deployments/rbac.yaml --ignore-not-found=true
	@echo "Undeploy completed"

# 查看Pod日志
.PHONY: logs
logs:
	@echo "Showing logs for PPU device plugin pods..."
	$(KUBECTL) logs -l app=ppu-device-plugin -n $(NAMESPACE) --follow

# 查看Pod状态
.PHONY: status
status:
	@echo "Checking PPU device plugin status..."
	$(KUBECTL) get pods -l app=ppu-device-plugin -n $(NAMESPACE) -o wide
	$(KUBECTL) get ds ppu-device-plugin -n $(NAMESPACE)

# 重新部署（先删除再部署）
.PHONY: redeploy
redeploy: undeploy deploy

# 完整的CI/CD流程
.PHONY: ci
ci: clean deps test docker-build docker-push

# 验证部署
.PHONY: verify
verify:
	@echo "Verifying deployment..."
	$(KUBECTL) get nodes -o json | jq '.items[].status.allocatable' | grep "alibabacloud.com/ppu" || echo "No PPU resources found"
	@echo "Verification completed"

# 开发模式（本地构建并部署）
.PHONY: dev-deploy
dev-deploy: build-local docker-build deploy

# 清理Docker镜像
.PHONY: docker-clean
docker-clean:
	@echo "Cleaning Docker images..."
	docker rmi $(FULL_IMAGE_NAME) || true
	docker system prune -f

# 生成版本信息
.PHONY: version
version:
	@echo "Image: $(FULL_IMAGE_NAME)"
	@echo "Registry: $(REGISTRY)"
	@echo "Tag: $(TAG)"
	@echo "Namespace: $(NAMESPACE)"