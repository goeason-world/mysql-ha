# MySQL HA - 高可用 MySQL 解决方案

[English](#english) | [中文](#中文)

---

## 中文

### 概述

MySQL HA 是一个完整的 MySQL 高可用解决方案，灵感来自 Patroni（PostgreSQL HA）。它提供自动故障转移、计划切换、Web 管理控制台和一键部署功能。

### 功能特性

- **自动故障转移**：检测 MySQL 故障并自动将副本提升为主节点
- **计划切换**：通过 Web UI 进行手动切换，零数据丢失
- **Web 管理控制台**：现代化的 Vue 3 + Element Plus 前端，暗色主题
- **一键部署**：通过 Web UI 自动安装 etcd、MySQL 和 HA Agent
- **分布式共识**：使用 etcd 进行领导者选举和集群状态管理
- **自动恢复**：HA Agent 在故障后自动修复 MySQL 环境
- **实时监控**：实时集群状态、复制延迟和事件日志
- **智能滚动更新**：Agent 更新时先更新从节点，最后更新主节点，避免不必要的选举
- **版本检查**：批量更新前检查版本，版本相同时提示是否强制更新
- **MySQL 服务管理**：支持通过 Web UI 重启单个节点的 MySQL 服务
- **一键修复主从**：自动检测并修复主从复制问题

### 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      Web 管理端 (webadmin)                       │
│                Vue 3 + Element Plus + TypeScript                 │
│                      集群管理 & 监控 (端口 8888)                   │
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
            │ │ :8080 │ │ │ │ :8080 │ │ │ │ :8080 │ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            │ ┌───────┐ │ │ ┌───────┐ │ │ ┌───────┐ │
            │ │ etcd  │ │ │ │ etcd  │ │ │ │ etcd  │ │
            │ │ :2379 │ │ │ │ :2379 │ │ │ │ :2379 │ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            └───────────┘ └───────────┘ └───────────┘
```

### 组件说明

| 组件 | 说明 | 端口 |
|------|------|------|
| **webadmin** | Web 管理服务器，处理集群创建、安装和监控 | 8888 |
| **mypatroni** | 运行在每个 MySQL 节点上的 HA Agent，管理故障转移和复制 | 8080 |
| **etcd** | 分布式键值存储，用于领导者选举和集群状态 | 2379/2380 |
| **MySQL** | 数据库服务器，使用 GTID 复制 | 3306 |

### 目录结构

```
mysql-ha/
├── cmd/
│   ├── mypatroni/          # HA Agent 入口
│   └── webadmin/           # Web 管理端入口
├── internal/
│   ├── agent/              # HA Agent 核心逻辑
│   ├── api/                # HA Agent REST API
│   ├── config/             # 配置管理
│   ├── dcs/                # 分布式协调服务 (etcd)
│   ├── election/           # 领导者选举
│   ├── ha/                 # 高可用逻辑
│   ├── installer/          # 自动安装器
│   ├── log/                # 日志模块
│   ├── mysql/              # MySQL 管理
│   └── webapi/             # Web 管理端 API
├── web/
│   └── frontend/           # Vue 3 前端
│       ├── src/
│       │   ├── api/        # API 调用
│       │   ├── components/ # 组件
│       │   ├── views/      # 页面
│       │   └── styles/     # 样式
│       └── dist/           # 构建输出
├── configs/                # 配置示例
├── docs/                   # 文档
├── scripts/                # 辅助脚本
├── cache/                  # 下载缓存
├── data/                   # 运行时数据
└── logs/                   # 日志文件
```

### 快速开始

#### 前置条件

- Go 1.21+
- Node.js 18+ (用于前端开发)
- Linux 服务器（Ubuntu/CentOS）用于 MySQL 节点
- 具有 sudo 权限的 SSH 访问

#### 编译

```bash
# 编译 webadmin（在管理服务器上运行）
go build -o webadmin ./cmd/webadmin

# 编译 HA Agent（通过 Web UI 部署到 MySQL 节点）
# 注意：必须交叉编译为 Linux 版本
GOOS=linux GOARCH=amd64 go build -o mypatroni ./cmd/mypatroni

# 编译前端（可选，用于开发）
cd web/frontend
npm install
npm run build  # 输出到 web/dist
```

#### 运行

**方式一：生产模式（推荐）**

```bash
# 启动 webadmin
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

### Web 管理功能

#### 集群概览
- 显示 MySQL 节点数、etcd 节点数、Agent 在线数
- 集群健康状态一览

#### MySQL 控制
- 主节点信息展示
- 从节点列表及复制延迟
- 主从切换（Switchover）
- 单节点 MySQL 重启
- 单节点修复
- 一键修复主从
- 自动刷新开关

#### etcd 控制
- etcd 节点列表
- 健康状态检查
- 集群端点信息

#### HA Agent 控制
- Agent 节点状态
- 版本信息
- 单节点重启/更新
- 批量更新（智能滚动更新）
- 版本检查（相同版本提示强制更新）

#### 实时日志
- 多节点日志聚合
- 按节点筛选
- 自动刷新

#### 安装状态
- 节点安装进度
- 失败节点重试

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
| `/api/v1/clusters/{id}` | GET | 获取集群详情 |
| `/api/v1/clusters/{id}` | DELETE | 删除集群 |
| `/api/v1/clusters/{id}/install` | POST | 开始集群安装 |
| `/api/v1/clusters/{id}/install/status` | GET | 获取安装状态 |
| `/api/v1/clusters/{id}/install/retry` | POST | 重试安装 |
| `/api/v1/clusters/{id}/status` | GET | 获取集群状态 |
| `/api/v1/clusters/{id}/etcd-status` | GET | 获取 etcd 状态 |
| `/api/v1/clusters/{id}/switchover` | POST | 发起切换 |
| `/api/v1/clusters/{id}/upgrade-agents` | POST | 升级 HA Agent |
| `/api/v1/clusters/{id}/restart-agents` | POST | 重启 HA Agent |
| `/api/v1/clusters/{id}/restart-mysql` | POST | 重启 MySQL |
| `/api/v1/clusters/{id}/repair-replication` | POST | 一键修复主从 |
| `/api/v1/clusters/{id}/nodes/{nodeId}/repair` | POST | 修复单个节点 |
| `/api/v1/clusters/{id}/logs` | GET | 获取集群日志 |
| `/health` | GET | 健康检查 |
| `/version` | GET | 版本信息 |

#### HA Agent API（端口 8080）

| 接口 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/state` | GET | 获取节点状态（角色、健康状态、复制信息、版本） |
| `/api/v1/cluster` | GET | 获取集群状态 |
| `/api/v1/nodes` | GET | 获取节点列表 |
| `/api/v1/switchover` | POST | 请求当前节点成为主节点 |
| `/api/v1/demote` | POST | 请求释放 leader 锁 |
| `/api/v1/restart-mysql` | POST | 重启 MySQL 服务 |

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

log:
  level: "info"
  file: "/var/log/mypatroni/mypatroni.log"

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
6. **Agent 重启保护**：Agent 重启时有 30 秒启动宽限期，避免不必要的选举

### 技术栈

**后端**
- Go 1.21+
- gorilla/mux (HTTP 路由)
- etcd client v3 (分布式协调)
- go-sql-driver/mysql (MySQL 驱动)
- pkg/sftp (SSH 文件传输)

**前端**
- Vue 3 + TypeScript
- Element Plus (UI 组件库)
- Vite (构建工具)
- Axios (HTTP 客户端)
- Vue Router (路由)
- Pinia (状态管理)
- SCSS (样式)

### 许可证

MIT License

---

## English

### Overview

MySQL HA is a comprehensive high availability solution for MySQL databases, inspired by Patroni (PostgreSQL HA). It provides automatic failover, planned switchover, web-based management console, and one-click deployment.

### Features

- **Automatic Failover**: Detects MySQL failures and automatically promotes a replica to leader
- **Planned Switchover**: Manual switchover through web UI with zero data loss
- **Web Management Console**: Modern Vue 3 + Element Plus frontend with dark theme
- **One-Click Deployment**: Automatically install etcd, MySQL, and HA Agent via web UI
- **Distributed Consensus**: Uses etcd for leader election and cluster state
- **Auto-Recovery**: HA Agent automatically repairs MySQL environment after failures
- **Real-time Monitoring**: Live cluster status, replication lag, and event logs
- **Smart Rolling Update**: Updates replicas first, then leader, avoiding unnecessary elections
- **Version Check**: Checks version before batch update, prompts for force update if same
- **MySQL Service Management**: Restart individual node's MySQL via web UI
- **One-Click Repair**: Automatically detect and fix replication issues

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Web Admin (webadmin)                      │
│                 Vue 3 + Element Plus + TypeScript                │
│                 Cluster Management & Monitoring (:8888)          │
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
            │ │ :8080 │ │ │ │ :8080 │ │ │ │ :8080 │ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            │ ┌───────┐ │ │ ┌───────┐ │ │ ┌───────┐ │
            │ │ etcd  │ │ │ │ etcd  │ │ │ │ etcd  │ │
            │ │ :2379 │ │ │ │ :2379 │ │ │ │ :2379 │ │
            │ └───────┘ │ │ └───────┘ │ │ └───────┘ │
            └───────────┘ └───────────┘ └───────────┘
```

### Components

| Component | Description | Port |
|-----------|-------------|------|
| **webadmin** | Web management server, handles cluster creation, installation, and monitoring | 8888 |
| **mypatroni** | HA Agent running on each MySQL node, manages failover and replication | 8080 |
| **etcd** | Distributed key-value store for leader election and cluster state | 2379/2380 |
| **MySQL** | Database server with GTID-based replication | 3306 |

### Quick Start

#### Prerequisites

- Go 1.21+
- Node.js 18+ (for frontend development)
- Linux servers (Ubuntu/CentOS) for MySQL nodes
- SSH access to target servers with sudo privileges

#### Build

```bash
# Build webadmin (runs on management server)
go build -o webadmin ./cmd/webadmin

# Build HA Agent (deployed to MySQL nodes via web UI)
# Note: Must cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o mypatroni ./cmd/mypatroni

# Build frontend (optional, for development)
cd web/frontend
npm install
npm run build  # Output to web/dist
```

#### Run

**Option 1: Production Mode (Recommended)**

```bash
# Start webadmin
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
| `/api/v1/clusters/{id}` | GET | Get cluster details |
| `/api/v1/clusters/{id}` | DELETE | Delete cluster |
| `/api/v1/clusters/{id}/install` | POST | Start cluster installation |
| `/api/v1/clusters/{id}/status` | GET | Get cluster status |
| `/api/v1/clusters/{id}/etcd-status` | GET | Get etcd status |
| `/api/v1/clusters/{id}/switchover` | POST | Initiate switchover |
| `/api/v1/clusters/{id}/upgrade-agents` | POST | Upgrade HA Agents |
| `/api/v1/clusters/{id}/restart-agents` | POST | Restart HA Agents |
| `/api/v1/clusters/{id}/restart-mysql` | POST | Restart MySQL |
| `/api/v1/clusters/{id}/repair-replication` | POST | One-click repair replication |
| `/api/v1/clusters/{id}/logs` | GET | Get cluster logs |

#### HA Agent API (Port 8080)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/state` | GET | Get node state (role, health, replication info, version) |
| `/api/v1/switchover` | POST | Request this node to become leader |
| `/api/v1/demote` | POST | Request to release leader lock |
| `/api/v1/restart-mysql` | POST | Restart MySQL service |

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

log:
  level: "info"
  file: "/var/log/mypatroni/mypatroni.log"

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
6. **Agent Restart Protection**: 30-second startup grace period when agent restarts to avoid unnecessary elections.

### Tech Stack

**Backend**
- Go 1.21+
- gorilla/mux (HTTP routing)
- etcd client v3 (distributed coordination)
- go-sql-driver/mysql (MySQL driver)
- pkg/sftp (SSH file transfer)

**Frontend**
- Vue 3 + TypeScript
- Element Plus (UI components)
- Vite (build tool)
- Axios (HTTP client)
- Vue Router (routing)
- Pinia (state management)
- SCSS (styling)

### License

MIT License
