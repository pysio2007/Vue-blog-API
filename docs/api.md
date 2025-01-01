---
title: 博客所有的API
date: 2025-01-01
icon: fa-solid fa-webhook
category: develop
tag:
  - 服务
  - API
---

## 通用说明

> [!NOTE]
> 所有API均已部署至生产环境，请使用以下基础URL进行访问。

### 基础URL
```
https://blogapi.pysio.online
```

> [!IMPORTANT]
> 在进行API调用时，请注意以下事项：
> - 所有请求都需要使用HTTPS协议
> - API可能会有速率限制
> - 请妥善保管您的认证令牌

### 通用响应格式
所有JSON响应都遵循以下格式：
```json
{
  "status": "success/error",
  "data/error": "响应数据/错误信息"
}
```

> [!TIP]
> 调试API时，建议使用以下工具：
> - [Postman](https://www.postman.com/)
> - [Insomnia](https://insomnia.rest/)
> - [curl](https://curl.se/)

### 认证头格式
```
Authorization: Bearer {TOKEN}
```

> [!WARNING]
> 请勿在客户端代码中直接存储管理员令牌。
> 确保在生产环境中使用安全的令牌存储方式。

## 系统状态接口

> [!NOTE]
> 系统状态接口可用于监控服务健康状况。

### 1. 系统状态检查
- **请求路径**: `/`
- **请求方法**: GET
- **响应格式**: text/plain
- **响应示例**: `你来这里干啥 喵?`
- **调试示例**:
  ```bash
  curl https://blogapi.pysio.online/
  ```

> [!TIP]
> 建议使用此端点进行基础连通性测试。

### 2. FastFetch 系统信息
- **请求路径**: `/fastfetch`
- **请求方法**: GET
- **响应格式**: application/json
- **响应示例**:
  ```json
  {
    "status": "success",
    "output": "系统信息HTML格式输出"
  }
  ```
- **调试示例**:
  ```bash
  curl https://blogapi.pysio.online/fastfetch
  ```

### 3. 心跳检测
- **请求路径**: `/heartbeat`
- **请求方法**: POST
- **请求头**:
  ```
  Authorization: Bearer {TOKEN}
  ```
- **响应格式**: application/json
- **响应示例**:
  ```json
  {
    "message": "Heartbeat received"
  }
  ```
- **调试示例**:
  ```bash
  curl -X POST -H "Authorization: Bearer your_token" https://blogapi.pysio.online/heartbeat
  ```

### 4. 状态检查
- **请求路径**: `/check`
- **请求方法**: GET
- **响应格式**: application/json
- **响应示例**:
  ```json
  {
    "alive": true,
    "last_heartbeat": 1234567890
  }
  ```
- **调试示例**:
  ```bash
  curl https://blogapi.pysio.online/check
  ```

## 图片管理接口

> [!CAUTION]
> 图片上传接口仅供管理员使用。
> 上传的图片会自动优化为WebP格式。

### 1. 获取随机图片
- **请求路径**: `/random_image`
- **请求方法**: GET
- **响应格式**: image/webp
- **响应示例**: 直接返回图片数据
- **响应头**:
  ```
  Content-Type: image/webp
  Content-Disposition: inline; filename="{hash}.webp"
  ```
- **错误响应**:
  ```json
  {
    "error": "No images available"
  }
  ```
- **调试示例**:
  ```bash
  curl https://blogapi.pysio.online/random_image -o random.webp
  ```

### 2. 获取特定图片
- **请求路径**: `/images/:hash`
- **请求方法**: GET
- **参数说明**:
  - `hash`: 图片的哈希值
- **响应格式**: image/webp
- **响应头**:
  ```
  Content-Type: image/webp
  Content-Disposition: inline; filename="{hash}.webp"
  ```
- **错误响应**:
  ```json
  {
    "error": "Image not found"
  }
  ```
- **调试示例**:
  ```bash
  curl https://blogapi.pysio.online/images/your_image_hash -o image.webp
  ```

### 3. 获取图片（优化版）
- **请求路径**: `/i/:hash`
- **请求方法**: GET
- **参数说明**:
  - `hash`: 图片的哈希值
- **响应格式**: image/webp
- **响应头**:
  ```
  Content-Type: image/webp
  Content-Disposition: inline; filename="{hash}.webp"
  Cache-Control: public, max-age=31536000
  ETag: "{hash}"
  ```
- **特殊响应**:
  - 304 Not Modified (当浏览器缓存有效时)
- **错误响应**:
  ```json
  {
    "error": "Image not found"
  }
  ```
- **调试示例**:
  ```bash
  curl -H "If-None-Match: \"your_image_hash\"" https://blogapi.pysio.online/i/your_image_hash -o image.webp
  ```

### 4. 上传图片
- **请求路径**: `/images/add`
- **请求方法**: POST
- **请求头**:
  ```
  Authorization: Bearer {ADMIN_TOKEN}
  Content-Type: multipart/form-data
  ```
- **请求参数**:
  - `image`: 图片文件 (form-data)
- **响应格式**: application/json
- **成功响应**:
  ```json
  {
    "hash": "图片的hash值",
    "contentType": "image/webp",
    "size": 图片大小(字节)
  }
  ```
- **错误响应**:
  ```json
  {
    "error": "Image file is required"
  }
  ```
  或
  ```json
  {
    "error": "Image already exists",
    "existingHash": "已存在图片的hash"
  }
  ```
- **调试示例**:
  ```bash
  curl -X POST \
    -H "Authorization: Bearer your_admin_token" \
    -F "image=@/path/to/your/image.jpg" \
    https://blogapi.pysio.online/images/add
  ```

### 5. 获取图片列表
- **请求路径**: `/images/list`
- **请求方法**: GET
- **查询参数**:
  - `page`: 页码 (默认: 1)
  - `limit`: 每页数量 (默认: 10)
- **响应格式**: application/json
- **响应示例**:
  ```json
  {
    "images": [
      {
        "hash": "图片hash",
        "contentType": "image/webp",
        "createdAt": "创建时间"
      }
    ],
    "pagination": {
      "current": 1,
      "size": 10,
      "total": 100
    }
  }
  ```
- **调试示例**:
  ```bash
  curl "https://blogapi.pysio.online/images/list?page=1&limit=10"
  ```

### 6. 获取图片总数
- **请求路径**: `/images/count`
- **请求方法**: GET
- **响应格式**: application/json
- **响应示例**:
  ```json
  {
    "count": 100
  }
  ```
- **调试示例**:
  ```bash
  curl https://blogapi.pysio.online/images/count
  ```

### 7. 删除图片
- **请求路径**: `/images/:hash`
- **请求方法**: DELETE
- **请求头**:
  ```
  Authorization: Bearer {ADMIN_TOKEN}
  ```
- **参数说明**:
  - `hash`: 要删除的图片hash
- **响应格式**: application/json
- **成功响应**:
  ```json
  {
    "message": "Image deleted successfully",
    "hash": "被删除图片的hash"
  }
  ```
- **错误响应**:
  ```json
  {
    "error": "Image not found"
  }
  ```
- **调试示例**:
  ```bash
  curl -X DELETE \
    -H "Authorization: Bearer your_admin_token" \
    https://blogapi.pysio.online/images/your_image_hash
  ```

## Steam 状态接口

### 1. Steam 游戏状态
- **请求路径**: `/steam_status`
- **请求方法**: GET
- **响应格式**: application/json
- **响应示例（游戏中）**:
  ```json
  {
    "status": "在游戏中",
    "game": "游戏名称",
    "game_id": "游戏ID",
    "description": "游戏描述",
    "price": "游戏价格",
    "playtime": "游戏时长",
    "achievement_percentage": "成就完成度"
  }
  ```
- **响应示例（不在游戏中）**:
  ```json
  {
    "status": "在线"
  }
  ```
- **调试示例**:
  ```bash
  curl https://blogapi.pysio.online/steam_status
  ```

## IP 查询接口

### 1. IP 信息查询
- **请求路径**: `/ipcheck`
- **请求方法**: GET
- **查询参数**:
  - `ip`: IP地址
- **响应格式**: application/json
- **响应示例**:
  ```json
  {
    "ip": "IP地址",
    "hostname": "主机名",
    "city": "城市",
    "region": "地区",
    "country": "国家",
    "loc": "位置坐标",
    "org": "组织",
    "postal": "邮编",
    "timezone": "时区"
  }
  ```
- **调试示例**:
  ```bash
  curl "https://blogapi.pysio.online/ipcheck?ip=8.8.8.8"
  ```

## API 统计接口

### 1. 获取所有API调用统计
- **请求路径**: `/api_stats`
- **请求方法**: GET
- **响应格式**: application/json
- **响应示例**:
  ```json
  [
    {
      "key": "API路径",
      "count": 调用次数,
      "lastUpdated": "最后调用时间"
    }
  ]
  ```
- **调试示例**:
  ```bash
  curl https://blogapi.pysio.online/api_stats
  ```

### 2. 获取特定API调用统计
- **请求路径**: `/api_stats/:key`
- **请求方法**: GET
- **参数说明**:
  - `key`: API路径
- **响应格式**: application/json
- **响应示例**:
  ```json
  {
    "key": "API路径",
    "count": 调用次数,
    "lastUpdated": "最后调用时间"
  }
  ```
  或
  ```json
  {
    "key": "API路径",
    "count": 0,
    "lastUpdated": null
  }
  ```
- **调试示例**:
  ```bash
  curl https://blogapi.pysio.online/api_stats/random_image
  ```

## 错误处理

> [!WARNING]
> 请确保您的应用程序正确处理以下错误响应：

### 常见错误码
| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| 401 | 未授权 | 检查认证令牌 |
| 403 | 禁止访问 | 确认权限级别 |
| 404 | 资源未找到 | 验证请求路径 |
| 429 | 请求过多 | 实现速率限制处理 |
| 500 | 服务器错误 | 联系管理员 |

### 401 未授权
```json
{
  "error": "Unauthorized"
}
```

### 404 未找到
```json
{
  "error": "Resource not found"
}
```

### 500 服务器错误
```json
{
  "error": "Internal server error"
}
```

## 最佳实践

> [!TIP]
> 1. 实现请求重试机制
> 2. 使用适当的缓存策略
> 3. 监控API调用频率
> 4. 实现错误处理
> 5. 定期检查API更新

## 性能优化

> [!NOTE]
> - 图片接口支持HTTP缓存
> - 使用CDN加速静态资源
> - 支持图片压缩和格式转换
> - 批量请求时注意并发限制

## 问题反馈

> [!TIP]
> 如遇到API相关问题，请通过以下方式反馈：
> 1. 提交GitHub Issue
> 2. 发送邮件至管理员
> 3. 在博客评论区留言
