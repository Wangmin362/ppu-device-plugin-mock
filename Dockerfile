# 构建阶段
FROM golang:1.25-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go模块文件
COPY go.mod ./
COPY go.sum ./

# 下载依赖
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ppu-device-plugin ./cmd/main.go

# 运行阶段
FROM alpine:latest

# 安装ca证书
RUN apk --no-cache add ca-certificates

# 创建非root用户
RUN adduser -D -s /bin/sh appuser

# 设置工作目录
WORKDIR /root/

# 从构建阶段复制二进制文件
COPY --from=builder /app/ppu-device-plugin .

# 设置权限
RUN chmod +x ./ppu-device-plugin

# 创建必要的目录
RUN mkdir -p /var/lib/kubelet/device-plugins/

# 暴露健康检查端口（可选）
EXPOSE 8080

# 设置用户
USER appuser

# 启动应用
CMD ["./ppu-device-plugin"]