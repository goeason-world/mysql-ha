# MyPatroni 部署指南

## 概述

MyPatroni 是一个 MySQL 高可用解决方案，参考 Patroni（PostgreSQL HA）架构设计。本文档介绍如何部署和配置 MyPatroni。

## 系统要求

### 软件要求
- Go 1.21+ (编译)
- MySQL 5.7.x 或 8.0.x
- etcd 3.5+
- Docker & Docker Compose (可选)

### 硬件要求
- 最少 3 个节点（1 主 2 从）
- 每节点至少 2GB 内存
- 稳定的网络连接

## 快速开始

### 使用 Docker Compose

1. 启动测试环境：
```bash
cd mysql-ha
docker-compose up -d
```

2. 检查服务状态：
```bash
docker-compose ps
```

### 手动部署

#### 1. 安装 etcd

```bash
# Ubuntu/Debian
apt-get install etcd

# CentOS/RHEL
yum install etcd

# 启动 etcd
systemctl start etcd
systemctl enable etcd
```

#### 2. 配置 MySQL

MySQL 5.7 配置 (`/etc/mysql/mysql.conf.d/mysqld.cnf`):
```ini
[mysqld]
server-id = 1
log-bin = mysql-bin
gtid-mode = ON
enforce-gtid-consistency = ON
log-slave-updates = ON
```

MySQL 8.0 配置:
```ini
[mysqld]
server-id = 1
log-bin = mysql-bin
gtid-mode = ON
enforce-gtid-consistency = ON
log-slave-updates = ON
# MySQL 8.0 特有
binlog_transaction_dependency_tracking = WRITESET
```

#### 3. 创建复制用户

```sql
-- 在主节点执行
CREATE USER 'replicator'@'%' IDENTIFIED BY 'repl-password';
GRANT REPLICATION SLAVE ON *.* TO 'replicator'@'%';

-- 创建 MyPatroni 管理用户
CREATE USER 'mypatroni'@'%' IDENTIFIED BY 'mypatroni-password';
GRANT ALL PRIVILEGES ON *.* TO 'mypatroni'@'%' WITH GRANT OPTION;
FLUSH PRIVILEGES;
```

#### 4. 编译 MyPatroni

```bash
cd mysql-ha
go build -o mypatroni ./cmd/mypatroni
```

#### 5. 配置 MyPatroni

复制示例配置：
```bash
mkdir -p /etc/mypatroni
cp configs/example.yaml /etc/mypatroni/config.yaml
```

编辑配置文件，修改以下关键参数：
- `name`: 节点唯一标识
- `dcs.endpoints`: etcd 地址
- `mysql.*`: MySQL 连接信息

#### 6. 启动 MyPatroni

```bash
./mypatroni --config /etc/mypatroni/config.yaml
```

## 配置说明

### DCS 配置

```yaml
dcs:
  endpoints:
    - "http://etcd1:2379"
    - "http://etcd2:2379"
    - "http://etcd3:2379"
```

### MySQL 配置

```yaml
mysql:
  host: "127.0.0.1"
  port: 3306
  user: "mypatroni"
  password: "your-password"
  replication_user: "replicator"
  replication_password: "repl-password"
```

### HA 配置

```yaml
ha:
  ttl: 30                    # Leader 锁 TTL
  loop_wait: 10              # 主循环间隔
  max_replication_lag: 10    # 最大复制延迟
  failover_cooldown: 60      # 故障转移冷却时间
```

## MySQL 版本差异

### MySQL 5.7
- 支持 GTID 和传统位置复制
- 使用 `CHANGE MASTER TO` 语法
- 使用 `SHOW SLAVE STATUS`

### MySQL 8.0
- 默认使用 GTID 复制
- 使用 `CHANGE REPLICATION SOURCE TO` 语法
- 使用 `SHOW REPLICA STATUS`

MyPatroni 会自动检测 MySQL 版本并使用正确的命令语法。

## 监控

### API 端点

- `GET /health` - 健康检查
- `GET /api/v1/cluster` - 集群状态
- `GET /api/v1/nodes` - 所有节点
- `POST /api/v1/switchover` - 发起切换

### 示例

```bash
# 检查健康状态
curl http://localhost:8080/health

# 获取集群状态
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/cluster

# 发起切换
curl -X POST -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"target_node_id": "mysql-node-2"}' \
  http://localhost:8080/api/v1/switchover
```

## 故障排除

### 常见问题

1. **无法连接 etcd**
   - 检查 etcd 服务是否运行
   - 检查防火墙设置
   - 验证 endpoints 配置

2. **MySQL 连接失败**
   - 检查 MySQL 服务状态
   - 验证用户权限
   - 检查网络连接

3. **复制延迟过高**
   - 检查网络带宽
   - 优化 MySQL 配置
   - 考虑增加从节点

### 日志位置

默认日志位置：`/var/log/mypatroni/mypatroni.log`

调整日志级别：
```yaml
log:
  level: "debug"  # debug, info, warn, error
```
