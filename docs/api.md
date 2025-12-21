# MyPatroni API 文档

## 概述

MyPatroni 提供 RESTful API 用于集群管理和监控。

## 认证

所有 `/api/v1/*` 端点需要 API Key 认证。

### 请求头认证
```
X-API-Key: your-api-key
```

### 查询参数认证
```
?api_key=your-api-key
```

## 端点

### 健康检查

#### GET /health

检查服务健康状态（无需认证）。

**响应**
```json
{
  "status": "ok"
}
```

---

### 集群状态

#### GET /api/v1/cluster

获取集群整体状态。

**响应**
```json
{
  "cluster_name": "mysql-cluster",
  "leader": {
    "node_id": "mysql-node-1",
    "hostname": "mysql-1",
    "role": "leader",
    "mysql_version": "8.0.33",
    "gtid_executed": "uuid:1-1000",
    "replication_lag": 0,
    "is_healthy": true
  },
  "nodes": [
    {
      "node_id": "mysql-node-1",
      "hostname": "mysql-1",
      "role": "leader",
      "is_healthy": true
    },
    {
      "node_id": "mysql-node-2",
      "hostname": "mysql-2",
      "role": "replica",
      "replication_lag": 0,
      "is_healthy": true
    }
  ]
}
```

---

### 节点管理

#### GET /api/v1/nodes

获取所有节点列表。

**响应**
```json
{
  "nodes": [
    {
      "node_id": "mysql-node-1",
      "hostname": "mysql-1",
      "role": "leader",
      "mysql_version": "8.0.33",
      "gtid_executed": "uuid:1-1000",
      "replication_lag": 0,
      "is_healthy": true
    }
  ]
}
```

#### GET /api/v1/nodes/{id}

获取指定节点详情。

**参数**
- `id` (path): 节点 ID

**响应**
```json
{
  "node_id": "mysql-node-1",
  "hostname": "mysql-1",
  "role": "leader",
  "mysql_version": "8.0.33",
  "gtid_executed": "uuid:1-1000",
  "replication_lag": 0,
  "is_healthy": true,
  "replication_info": {
    "master_host": "",
    "master_port": 0,
    "slave_io_running": false,
    "slave_sql_running": false
  }
}
```

---

### 操作

#### POST /api/v1/switchover

发起计划内主从切换。

**请求体**
```json
{
  "target_node_id": "mysql-node-2",
  "reason": "planned maintenance"
}
```

**参数**
- `target_node_id` (required): 目标节点 ID
- `reason` (optional): 切换原因

**响应**
```json
{
  "status": "accepted",
  "message": "switchover initiated",
  "event_id": "uuid-xxx"
}
```

**错误响应**
```json
{
  "error": "target replica is not healthy"
}
```

---

### 历史记录

#### GET /api/v1/history

获取操作历史记录。

**查询参数**
- `limit` (optional): 返回记录数量，默认 10
- `type` (optional): 事件类型 (failover, switchover)

**响应**
```json
{
  "events": [
    {
      "id": "uuid-xxx",
      "type": "switchover",
      "old_leader": "mysql-node-1",
      "new_leader": "mysql-node-2",
      "reason": "planned maintenance",
      "start_time": "2024-01-01T10:00:00Z",
      "end_time": "2024-01-01T10:00:05Z",
      "success": true
    }
  ]
}
```

---

## 错误响应

所有错误响应格式：

```json
{
  "error": "error message",
  "code": "ERROR_CODE",
  "details": "additional details"
}
```

### HTTP 状态码

| 状态码 | 说明 |
|-------|------|
| 200 | 成功 |
| 202 | 已接受（异步操作） |
| 400 | 请求参数错误 |
| 401 | 认证失败 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

---

## 示例

### cURL

```bash
# 获取集群状态
curl -H "X-API-Key: your-api-key" \
  http://localhost:8080/api/v1/cluster

# 发起切换
curl -X POST \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"target_node_id": "mysql-node-2"}' \
  http://localhost:8080/api/v1/switchover
```

### Python

```python
import requests

API_URL = "http://localhost:8080"
API_KEY = "your-api-key"

headers = {"X-API-Key": API_KEY}

# 获取集群状态
resp = requests.get(f"{API_URL}/api/v1/cluster", headers=headers)
print(resp.json())

# 发起切换
resp = requests.post(
    f"{API_URL}/api/v1/switchover",
    headers=headers,
    json={"target_node_id": "mysql-node-2"}
)
print(resp.json())
```
