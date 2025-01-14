FROM golang:1.20-alpine

WORKDIR /app

# 安装必要的系统依赖
RUN apk add --no-cache \
    build-base \
    fastfetch \
    libwebp-tools \
    libwebp-dev

# 设置环境变量
ENV WEBP_BIN_PATH=/usr/bin

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN go build -o main .

EXPOSE 5000

CMD ["./main"]
