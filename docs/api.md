# API 文档

## 通用说明

### 基础URL
```
http://localhost:5000
```

### 通用响应格式
所有JSON响应都遵循以下格式：
```json
{
  "status": "success/error",
  "data/error": "响应数据/错误信息"
}
```

### 认证头格式
```
Authorization: Bearer {TOKEN}
```

## 系统状态接口

### 1. 系统状态检查
- **请求路径**: `/`
- **请求方法**: GET
- **响应格式**: text/plain
- **响应示例**: `你来这里干啥 喵?`
- **调试示例**:
  ```bash
  curl http://localhost:5000/
  ```

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
  curl http://localhost:5000/fastfetch
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
  curl -X POST -H "Authorization: Bearer your_token" http://localhost:5000/heartbeat
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
  curl http://localhost:5000/check
  ```

## 图片管理接口

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
  curl http://localhost:5000/random_image -o random.webp
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
  curl http://localhost:5000/images/your_image_hash -o image.webp
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
  curl -H "If-None-Match: \"your_image_hash\"" http://localhost:5000/i/your_image_hash -o image.webp
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
    http://localhost:5000/images/add
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
  curl "http://localhost:5000/images/list?page=1&limit=10"
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
  curl http://localhost:5000/images/count
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
    http://localhost:5000/images/your_image_hash
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
  curl http://localhost:5000/steam_status
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
  curl "http://localhost:5000/ipcheck?ip=8.8.8.8"
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
  curl http://localhost:5000/api_stats
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
  curl http://localhost:5000/api_stats/random_image
  ```

## 错误响应示例

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

## 注意事项

1. 所有图片自动转换为WebP格式存储和返回
2. 图片上传时进行重复检测
3. `/i/:hash` 接口支持浏览器缓存
4. 所有API调用自动记录到统计系统
5. 管理员操作需要ADMIN_TOKEN
6. 错误记录到日志系统
7. 建议使用工具如Postman进行API调试
8. 所有时间戳使用UTC时间
