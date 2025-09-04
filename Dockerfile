FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ppu-device-plugin ./cmd/main.go

FROM alpine:latest

WORKDIR /root/
COPY --from=builder /app/ppu-device-plugin .
RUN chmod +x ./ppu-device-plugin

CMD ["./ppu-device-plugin"]