# Go Blog API

基于 Go 语言的博客 API 服务，提供图片存储、Steam 状态查询、系统信息获取等功能。

## 功能特性

- 图片上传和管理（支持 WebP 格式）
- Steam 游戏状态查询
- IP 信息查询
- 系统信息获取（使用 fastfetch）
- API 调用统计
- 心跳检测

## 环境要求

- Go 1.20 或更高版本
- MongoDB
- libvips（用于图片处理）
- fastfetch（用于系统信息获取）

## 安装

1. 克隆仓库
```bash
git clone https://github.com/yourusername/Go-Blog-Api.git
cd Go-Blog-Api
```

2. 安装依赖
```bash
go mod download
```

3. 配置环境变量
```bash
cp .env.example .env
# 编辑 .env 文件，填写必要的配置信息
```

4. 运行服务
```bash
go run main.go
```

## Docker 部署

```bash
docker build -t blog-api .
docker run -p 5000:5000 blog-api
```

## API 接口

### 基础接口
- `GET /` - 主页
- `GET /fastfetch` - 获取系统信息
- `POST /heartbeat` - 心跳检测
  ```bash
  # 请求示例
  curl -X POST http://api.example.com/heartbeat \
    -H "Authorization: Bearer YOUR_TOKEN" \
    -d "application=MyApp" \
    -d "introduce=My Application Description" \
    -d "rgba=233,30,99,0.17" \
    -d "applicationOnline=true"

  # 响应示例
  {
    "message": "Heartbeat received",
    "application": "MyApp",
    "introduce": "My Application Description",
    "rgba": "233,30,99,0.17",
    "applicationOnline": true
  }
  ```
- `GET /check` - 检查服务状态
  ```bash
  # 请求示例
  curl http://api.example.com/check

  # 响应示例（当应用在线时）
  {
    "alive": true,
    "last_heartbeat": 1698314159,
    "application": "MyApp",
    "introduce": "My Application Description",
    "rgba": "233,30,99,0.17",
    "applicationOnline": true
  }

  # 响应示例（当应用离线时）
  {
    "alive": true,
    "last_heartbeat": 1698314159,
    "applicationOnline": false
  }

  # 响应示例（从未收到心跳时）
  {
    "alive": false,
    "last_heartbeat": null,
    "applicationOnline": false
  }
  ```
- `GET /check` - 检查服务状态

### 图片相关
- `GET /random_image` - 随机获取图片
- `GET /images/count` - 获取图片总数
- `GET /images/list` - 获取图片列表
- `POST /images/add` - 添加新图片
- `DELETE /images/:hash` - 删除图片
- `GET /images/:hash` - 获取指定图片
- `GET /i/:hash` - 通过 hash 直接访问图片

### 其他功能
- `GET /steam_status` - 获取 Steam 状态
- `GET /ipcheck` - IP 信息查询
- `GET /api_stats` - 获取 API 调用统计
- `GET /api_stats/:key` - 获取特定接口调用次数

## 许可证

AGPLv3 License
