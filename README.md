# MySQL HA - High Availability Solution for MySQL

[English](#english) | [中文](#中文)

---

## English

### Overview

MySQL HA is a comprehensive high availability solution for MySQL databases, inspired by Patroni (PostgreSQL HA). It provides automatic failover, planned switchover, and cluster management through a web-based interface.

### Features

- **Automatic Failover**: Detects MySQL failures and automatically promotes a replica to leader
- **Planned Switchover**: Manual switchover through web UI with zero data loss
- **Web Management Console**: Modern Vue.js frontend for cluster management
- **Distributed Consensus**: Uses etcd for leader election and cluster state
- **Auto-Recovery**: HA Agent automatically repairs MySQL environment after failures
- **Real-time Monitoring**: Live cluster status, replication lag, and event logs

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Web Admin (webadmin)                      │
│                    Cluster Management & Monitoring               │
└─────────────────────────────────────────────────────────────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
            ┌───────────┐ ┌───────────┐ ┌───────────┐
            │  Node 1   │ │  Node 2   │ │  Node 3   │
            │ ┌───────┐ │ │ ┌───────┐ │ │ ┌───────┐ │
            │ │ MySQL │ │ │ │ MySQL │ │ │ │ MySQL │ │
            │ │(Leader)│ │ │(Replica)│ │ │(Replica)│ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            │ ┌───────┐ │ │ ┌───────┐ │ │ ┌───────┐ │
            │ │  HA   │ │ │ │  HA   │ │ │ │  HA   │ │
            │ │ Agent │ │ │ │ Agent │ │ │ │ Agent │ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            │ ┌───────┐ │ │ ┌───────┐ │ │ ┌───────┐ │
            │ │ etcd  │ │ │ │ etcd  │ │ │ │ etcd  │ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            └───────────┘ └───────────┘ └───────────┘
```

### Components

| Component | Description |
|-----------|-------------|
| **webadmin** | Web management server, handles cluster creation, installation, and monitoring |
| **mypatroni** | HA Agent running on each MySQL node, manages failover and replication |
| **etcd** | Distributed key-value store for leader election and cluster state |
| **MySQL** | Database server with GTID-based replication |

### Quick Start

#### Prerequisites

- Go 1.21+
- Linux servers (Ubuntu/CentOS) for MySQL nodes
- SSH access to target servers with sudo privileges

#### Build

```bash
# Build webadmin (runs on management server)
go build -o webadmin ./cmd/webadmin

# Build HA Agent (deployed to MySQL nodes via web UI)
GOOS=linux GOARCH=amd64 go build -o mypatroni ./cmd/mypatroni

# Build frontend (optional, for development)
cd web/frontend
npm install
npm run build  # Output to web/dist, served by webadmin
```

#### Run

**Option 1: Production Mode (Recommended)**

```bash
# Start webadmin (serves built frontend from web/dist)
./webadmin
# Access http://localhost:8888
```

**Option 2: Development Mode (Hot Reload)**

```bash
# Terminal 1: Start backend
./webadmin

# Terminal 2: Start frontend dev server
cd web/frontend
npm run dev
# Access http://localhost:3000 (proxies API to :8888)
```

#### Deploy a Cluster

1. Open http://localhost:8888 in your browser
2. Click "Create Cluster"
3. Add MySQL nodes with SSH credentials
4. Select MySQL and etcd versions
5. Click "Install" to deploy

### Switchover vs Failover

| Operation | Trigger | Data Loss | Description |
|-----------|---------|-----------|-------------|
| **Switchover** | Manual (Web UI) | None | Planned leader change, waits for replication sync |
| **Failover** | Automatic | Minimal | Triggered when leader becomes unavailable |

### API Reference

#### Web Admin API (Port 8888)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/clusters` | GET | List all clusters |
| `/api/v1/clusters` | POST | Create a new cluster |
| `/api/v1/clusters/{id}/install` | POST | Start cluster installation |
| `/api/v1/clusters/{id}/status` | GET | Get cluster status |
| `/api/v1/clusters/{id}/switchover` | POST | Initiate switchover |
| `/api/v1/clusters/{id}/upgrade-agents` | POST | Upgrade HA Agents |
| `/api/v1/clusters/{id}/logs` | GET | Get cluster logs |

#### HA Agent API (Port 8080)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/state` | GET | Get node state (role, health, replication info) |
| `/api/v1/switchover` | POST | Request this node to become leader |

### Configuration

#### HA Agent Config (`/etc/mypatroni/config.yaml`)

```yaml
name: mysql-node-1
namespace: mypatroni
scope: mysql-cluster
advertise_host: "10.211.55.32"  # External IP for this node

dcs:
  endpoints:
    - "http://10.211.55.32:2379"
    - "http://10.211.55.33:2379"
    - "http://10.211.55.34:2379"

mysql:
  host: "127.0.0.1"
  port: 3306
  user: "root"
  password: "your_password"
  replication_user: "replicator"
  replication_password: "repl_password"

api:
  listen: "0.0.0.0"
  port: 8080

ha:
  ttl: 30              # Leader lock TTL in seconds
  loop_wait: 10        # Main loop interval
  retry_timeout: 10    # Retry timeout for operations
  max_replication_lag: 10  # Max allowed replication lag
  failover_cooldown: 60    # Cooldown between failovers
```

### How It Works

1. **Leader Election**: All HA Agents compete for an etcd lock. The winner becomes the leader.
2. **Health Monitoring**: Each agent monitors local MySQL and reports status to etcd.
3. **Automatic Failover**: If the leader's MySQL fails, it releases the lock. Another agent acquires it and promotes its MySQL to leader.
4. **Replication Management**: Replica agents automatically configure replication to point to the current leader.
5. **Auto-Recovery**: If MySQL crashes, the agent attempts to repair and restart it.

### License

MIT License

---

## 中文

### 概述

MySQL HA 是一个完整的 MySQL 高可用解决方案，灵感来自 Patroni（PostgreSQL HA）。它提供自动故障转移、计划切换和基于 Web 的集群管理功能。

### 功能特性

- **自动故障转移**：检测 MySQL 故障并自动将副本提升为主节点
- **计划切换**：通过 Web UI 进行手动切换，零数据丢失
- **Web 管理控制台**：现代化的 Vue.js 前端，用于集群管理
- **分布式共识**：使用 etcd 进行领导者选举和集群状态管理
- **自动恢复**：HA Agent 在故障后自动修复 MySQL 环境
- **实时监控**：实时集群状态、复制延迟和事件日志

### 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      Web 管理端 (webadmin)                       │
│                      集群管理 & 监控                              │
└─────────────────────────────────────────────────────────────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
            ┌───────────┐ ┌───────────┐ ┌───────────┐
            │   节点 1   │ │   节点 2   │ │   节点 3   │
            │ ┌───────┐ │ │ ┌───────┐ │ │ ┌───────┐ │
            │ │ MySQL │ │ │ │ MySQL │ │ │ │ MySQL │ │
            │ │ (主)  │ │ │ │ (从)  │ │ │ │ (从)  │ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            │ ┌───────┐ │ │ ┌───────┐ │ │ ┌───────┐ │
            │ │  HA   │ │ │ │  HA   │ │ │ │  HA   │ │
            │ │ Agent │ │ │ │ Agent │ │ │ │ Agent │ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            │ ┌───────┐ │ │ ┌───────┐ │ │ ┌───────┐ │
            │ │ etcd  │ │ │ │ etcd  │ │ │ │ etcd  │ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            └───────────┘ └───────────┘ └───────────┘
```

### 组件说明

| 组件 | 说明 |
|------|------|
| **webadmin** | Web 管理服务器，处理集群创建、安装和监控 |
| **mypatroni** | 运行在每个 MySQL 节点上的 HA Agent，管理故障转移和复制 |
| **etcd** | 分布式键值存储，用于领导者选举和集群状态 |
| **MySQL** | 数据库服务器，使用 GTID 复制 |

### 快速开始

#### 前置条件

- Go 1.21+
- Linux 服务器（Ubuntu/CentOS）用于 MySQL 节点
- 具有 sudo 权限的 SSH 访问

#### 编译

```bash
# 编译 webadmin（在管理服务器上运行）
go build -o webadmin ./cmd/webadmin

# 编译 HA Agent（通过 Web UI 部署到 MySQL 节点）
GOOS=linux GOARCH=amd64 go build -o mypatroni ./cmd/mypatroni

# 编译前端（可选，用于开发）
cd web/frontend
npm install
npm run build  # 输出到 web/dist，由 webadmin 提供服务
```

#### 运行

**方式一：生产模式（推荐）**

```bash
# 启动 webadmin（从 web/dist 提供已构建的前端）
./webadmin
# 访问 http://localhost:8888
```

**方式二：开发模式（热重载）**

```bash
# 终端 1：启动后端
./webadmin

# 终端 2：启动前端开发服务器
cd web/frontend
npm run dev
# 访问 http://localhost:3000（API 代理到 :8888）
```

#### 部署集群

1. 在浏览器中打开 http://localhost:8888
2. 点击"创建集群"
3. 添加 MySQL 节点及 SSH 凭据
4. 选择 MySQL 和 etcd 版本
5. 点击"安装"开始部署

### 切换 vs 故障转移

| 操作 | 触发方式 | 数据丢失 | 说明 |
|------|----------|----------|------|
| **切换 (Switchover)** | 手动（Web UI） | 无 | 计划性主节点变更，等待复制同步 |
| **故障转移 (Failover)** | 自动 | 极少 | 当主节点不可用时触发 |

### API 参考

#### Web 管理端 API（端口 8888）

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/clusters` | GET | 列出所有集群 |
| `/api/v1/clusters` | POST | 创建新集群 |
| `/api/v1/clusters/{id}/install` | POST | 开始集群安装 |
| `/api/v1/clusters/{id}/status` | GET | 获取集群状态 |
| `/api/v1/clusters/{id}/switchover` | POST | 发起切换 |
| `/api/v1/clusters/{id}/upgrade-agents` | POST | 升级 HA Agent |
| `/api/v1/clusters/{id}/logs` | GET | 获取集群日志 |

#### HA Agent API（端口 8080）

| 接口 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/state` | GET | 获取节点状态（角色、健康状态、复制信息） |
| `/api/v1/switchover` | POST | 请求当前节点成为主节点 |

### 配置说明

#### HA Agent 配置 (`/etc/mypatroni/config.yaml`)

```yaml
name: mysql-node-1
namespace: mypatroni
scope: mysql-cluster
advertise_host: "10.211.55.32"  # 本节点的外部 IP

dcs:
  endpoints:
    - "http://10.211.55.32:2379"
    - "http://10.211.55.33:2379"
    - "http://10.211.55.34:2379"

mysql:
  host: "127.0.0.1"
  port: 3306
  user: "root"
  password: "your_password"
  replication_user: "replicator"
  replication_password: "repl_password"

api:
  listen: "0.0.0.0"
  port: 8080

ha:
  ttl: 30              # 领导者锁 TTL（秒）
  loop_wait: 10        # 主循环间隔
  retry_timeout: 10    # 操作重试超时
  max_replication_lag: 10  # 最大允许复制延迟
  failover_cooldown: 60    # 故障转移冷却时间
```

### 工作原理

1. **领导者选举**：所有 HA Agent 竞争 etcd 锁，获胜者成为领导者
2. **健康监控**：每个 Agent 监控本地 MySQL 并向 etcd 报告状态
3. **自动故障转移**：如果领导者的 MySQL 故障，它会释放锁。另一个 Agent 获取锁并将其 MySQL 提升为领导者
4. **复制管理**：副本 Agent 自动配置复制指向当前领导者
5. **自动恢复**：如果 MySQL 崩溃，Agent 会尝试修复并重启

### 许可证

MIT License
