# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS builder

WORKDIR /app

# 复制依赖配置文件并下载
COPY go.mod go.sum ./
RUN go mod download

# 复制项目源码
COPY . .

# 编译纯 Go 二进制程序 (CGO_ENABLED=0)
ENV CGO_ENABLED=0
RUN go build -o /app/bin/server src/main.go

# Stage 2: Robust debian runtime image
FROM debian:bookworm-slim

# 安装必要证书、时区和调试工具
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata curl && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 拷贝构建好的服务端程序
COPY --from=builder /app/bin/server ./bin/server

# 拷贝大模型预测所需的初始配置、数据库及前端静态资源
COPY --from=builder /app/data ./data
COPY --from=builder /app/src/frontend ./src/frontend

# 暴露 Gin 服务端口
EXPOSE 20260

# 默认环境变量配置
ENV PORT=20260
ENV OLLAMA_URL=http://host.docker.internal:11434
ENV OLLAMA_MODEL=qwen3.6:35b-q4

# 启动常运行服务
CMD ["./bin/server"]
