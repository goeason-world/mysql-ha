<template>
  <div class="cluster-detail-page">
    <!-- 左侧菜单 -->
    <aside class="detail-sidebar">
      <div class="sidebar-header">
        <div class="cluster-badge">🗄️</div>
        <div class="cluster-title">
          <span class="name">{{ cluster?.name || '集群管理' }}</span>
          <span class="id">{{ cluster?.id?.slice(0, 8) }}</span>
        </div>
      </div>
      
      <nav class="sidebar-nav">
        <div class="nav-item" :class="{ active: activeMenu === 'overview' }" @click="activeMenu = 'overview'">
          <span class="nav-icon">📊</span>
          <span>集群概览</span>
        </div>
        <div class="nav-item" :class="{ active: activeMenu === 'mysql' }" @click="activeMenu = 'mysql'">
          <span class="nav-icon">🗄️</span>
          <span>MySQL 控制</span>
        </div>
        <div class="nav-item" :class="{ active: activeMenu === 'etcd' }" @click="activeMenu = 'etcd'">
          <span class="nav-icon">🔗</span>
          <span>etcd 控制</span>
        </div>
        <div class="nav-item" :class="{ active: activeMenu === 'agent' }" @click="activeMenu = 'agent'">
          <span class="nav-icon">🤖</span>
          <span>HA Agent</span>
        </div>
        <div class="nav-item" :class="{ active: activeMenu === 'logs' }" @click="activeMenu = 'logs'">
          <span class="nav-icon">📋</span>
          <span>实时日志</span>
        </div>
        <div class="nav-item" :class="{ active: activeMenu === 'install' }" @click="activeMenu = 'install'">
          <span class="nav-icon">📦</span>
          <span>安装状态</span>
        </div>
      </nav>
      
      <div class="sidebar-footer">
        <button class="back-btn" @click="router.push('/clusters')">
          <span>←</span>
          <span>返回列表</span>
        </button>
      </div>
    </aside>

    <!-- 主内容区 -->
    <main class="detail-content">
      <div v-if="loading" class="loading">
        <div class="spinner"></div>
        <span>加载中...</span>
      </div>

      <template v-else-if="cluster">
        <!-- 集群概览 -->
        <section v-show="activeMenu === 'overview'" class="content-section">
          <div class="section-header">
            <h2>📊 集群概览</h2>
          </div>
          
          <div class="stats-grid">
            <div class="stat-card">
              <div class="stat-icon primary">🗄️</div>
              <div class="stat-content">
                <div class="stat-value">{{ mysqlNodeCount }}</div>
                <div class="stat-label">MySQL 节点</div>
              </div>
            </div>
            <div class="stat-card">
              <div class="stat-icon info">🔗</div>
              <div class="stat-content">
                <div class="stat-value">{{ etcdNodeCount }}</div>
                <div class="stat-label">etcd 节点</div>
              </div>
            </div>
            <div class="stat-card">
              <div class="stat-icon warning">🤖</div>
              <div class="stat-content">
                <div class="stat-value">{{ agentHealthyCount }}/{{ mysqlNodeCount }}</div>
                <div class="stat-label">Agent 在线</div>
              </div>
            </div>
            <div class="stat-card">
              <div class="stat-icon" :class="isClusterHealthy ? 'success' : 'danger'">
                {{ isClusterHealthy ? '✅' : '❌' }}
              </div>
              <div class="stat-content">
                <div class="stat-value">{{ isClusterHealthy ? '正常' : '异常' }}</div>
                <div class="stat-label">集群状态</div>
              </div>
            </div>
          </div>

          <div class="card">
            <div class="card-header">
              <span class="card-title">基本信息</span>
            </div>
            <div class="card-body">
              <div class="info-grid">
                <div class="info-item">
                  <span class="info-label">集群ID</span>
                  <span class="info-value mono">{{ cluster.id }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">集群名称</span>
                  <span class="info-value">{{ cluster.name }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">MySQL 版本</span>
                  <span class="info-value">{{ cluster.mysql_version }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">etcd 版本</span>
                  <span class="info-value">{{ cluster.etcd_version }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">安装路径</span>
                  <span class="info-value mono">{{ cluster.install_path }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">数据目录</span>
                  <span class="info-value mono">{{ cluster.data_path }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">MySQL 端口</span>
                  <span class="info-value">{{ cluster.settings?.mysql_port }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">HA Agent 端口</span>
                  <span class="info-value">{{ cluster.settings?.ha_agent_port }}</span>
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- MySQL 控制 -->
        <section v-show="activeMenu === 'mysql'" class="content-section">
          <div class="section-header">
            <h2>🗄️ MySQL 集群控制</h2>
            <div class="header-actions">
              <label class="auto-refresh-toggle">
                <input type="checkbox" v-model="autoRefreshMySQL" />
                <span>自动刷新</span>
              </label>
              <button class="btn btn-danger" @click="showRepairDialog">
                ⚠️ 一键修复主从
              </button>
            </div>
          </div>

          <div class="card">
            <div class="card-header">
              <span class="card-title">👑 主节点 (Leader)</span>
              <span class="tag" :class="clusterStatus?.leader ? 'tag-success' : 'tag-danger'">
                {{ clusterStatus?.leader ? '运行中' : '无主节点' }}
              </span>
            </div>
            <div class="card-body">
              <div v-if="clusterStatus?.leader" class="info-grid">
                <div class="info-item">
                  <span class="info-label">节点名称</span>
                  <span class="info-value">{{ clusterStatus.leader.name }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">IP 地址</span>
                  <span class="info-value mono">{{ clusterStatus.leader.ip }}</span>
                </div>
                <div class="info-item">
                  <span class="info-label">健康状态</span>
                  <span class="tag" :class="clusterStatus.leader.is_healthy ? 'tag-success' : 'tag-danger'">
                    {{ clusterStatus.leader.is_healthy ? '健康' : '异常' }}
                  </span>
                </div>
                <div class="info-item">
                  <span class="info-label">安装状态</span>
                  <span class="tag" :class="'tag-' + getStatusColor(clusterStatus.leader.install_status)">
                    {{ clusterStatus.leader.install_status }}
                  </span>
                </div>
                <div class="info-item full-width">
                  <span class="info-label">操作</span>
                  <div class="info-value">
                    <button class="btn btn-sm btn-secondary" @click="restartMySQLNode(clusterStatus.leader)">重启 MySQL</button>
                  </div>
                </div>
              </div>
              <div v-else class="empty-state">
                <div class="empty-icon">👑</div>
                <div class="empty-title">暂无主节点信息</div>
              </div>
            </div>
          </div>

          <div class="card" style="margin-top: 20px;">
            <div class="card-header">
              <span class="card-title">🔄 从节点 (Replicas)</span>
            </div>
            <div class="card-body">
              <div class="table-container" v-if="replicaNodes.length > 0">
                <table class="table">
                  <thead>
                    <tr>
                      <th>节点名称</th>
                      <th>IP 地址</th>
                      <th>健康状态</th>
                      <th>复制延迟</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="node in replicaNodes" :key="node.node_id">
                      <td>{{ node.name }}</td>
                      <td class="mono">{{ node.ip }}</td>
                      <td>
                        <span class="tag" :class="node.is_healthy ? 'tag-success' : 'tag-danger'">
                          {{ node.is_healthy ? '健康' : '异常' }}
                        </span>
                      </td>
                      <td>{{ node.replication_lag !== undefined ? node.replication_lag + 's' : '-' }}</td>
                      <td>
                        <button class="btn btn-sm btn-primary" :disabled="!node.is_healthy || node.install_status !== 'completed'" @click="showSwitchoverDialog(node)">切换为主</button>
                        <button class="btn btn-sm btn-secondary" @click="restartMySQLNode(node)" style="margin-left: 8px;">重启</button>
                        <button class="btn btn-sm btn-secondary" :disabled="node.is_healthy" @click="showRepairNodeDialog(node)" style="margin-left: 8px;">修复</button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-else class="empty-state">
                <div class="empty-icon">🔄</div>
                <div class="empty-title">暂无从节点</div>
              </div>
            </div>
          </div>
        </section>

        <!-- etcd 控制 -->
        <section v-show="activeMenu === 'etcd'" class="content-section">
          <div class="section-header">
            <h2>🔗 etcd 集群控制</h2>
            <div class="header-actions">
              <button class="btn btn-primary" @click="refreshEtcdStatus" :disabled="refreshingEtcd">
                🔄 刷新状态
              </button>
            </div>
          </div>

          <div class="card">
            <div class="card-header">
              <span class="card-title">etcd 节点列表</span>
            </div>
            <div class="card-body">
              <div class="table-container">
                <table class="table">
                  <thead>
                    <tr>
                      <th>节点名称</th>
                      <th>IP 地址</th>
                      <th>客户端端口</th>
                      <th>Peer 端口</th>
                      <th>健康状态</th>
                      <th>安装状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="node in etcdNodesWithHealth" :key="node.id">
                      <td>{{ node.name }}</td>
                      <td class="mono">{{ node.ip }}</td>
                      <td class="mono">{{ node.ip }}:2379</td>
                      <td class="mono">{{ node.ip }}:2380</td>
                      <td>
                        <span class="tag" :class="node.is_healthy ? 'tag-success' : 'tag-danger'">
                          {{ node.is_healthy ? '健康' : '异常' }}
                        </span>
                      </td>
                      <td>
                        <span class="tag" :class="'tag-' + getStatusColor(node.status)">{{ node.status }}</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <div class="card" style="margin-top: 20px;">
            <div class="card-header">
              <span class="card-title">etcd 集群信息</span>
            </div>
            <div class="card-body">
              <div class="info-grid">
                <div class="info-item full-width">
                  <span class="info-label">集群端点</span>
                  <div class="info-value">
                    <span class="tag tag-info" v-for="node in etcdNodes" :key="node.id" style="margin-right: 8px;">
                      http://{{ node.ip }}:2379
                    </span>
                  </div>
                </div>
                <div class="info-item">
                  <span class="info-label">集群 Token</span>
                  <span class="info-value mono">mysql-ha-etcd-cluster</span>
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- HA Agent 控制 -->
        <section v-show="activeMenu === 'agent'" class="content-section">
          <div class="section-header">
            <h2>🤖 HA Agent 控制</h2>
            <div class="header-actions">
              <button class="btn btn-primary" @click="refreshAgentStatus" :disabled="refreshingAgent">
                🔄 刷新状态
              </button>
              <button class="btn btn-secondary" @click="showUpgradeDialog">
                ⬆️ 批量更新
              </button>
            </div>
          </div>

          <div class="card">
            <div class="card-header">
              <span class="card-title">Agent 节点状态</span>
            </div>
            <div class="card-body">
              <div class="table-container">
                <table class="table">
                  <thead>
                    <tr>
                      <th>节点名称</th>
                      <th>IP 地址</th>
                      <th>Agent 状态</th>
                      <th>版本</th>
                      <th>API 端口</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="node in agentNodes" :key="node.node_id">
                      <td>{{ node.name }}</td>
                      <td class="mono">{{ node.ip }}</td>
                      <td>
                        <span class="tag" :class="node.is_healthy ? 'tag-success' : 'tag-danger'">
                          {{ node.is_healthy ? '运行中' : '离线' }}
                        </span>
                      </td>
                      <td>{{ node.agent_version || '-' }}</td>
                      <td>{{ cluster.settings?.ha_agent_port || 8080 }}</td>
                      <td>
                        <button class="btn btn-sm btn-primary" @click="restartAgent(node)">重启</button>
                        <button class="btn btn-sm btn-secondary" @click="upgradeAgent(node)" style="margin-left: 8px;">更新</button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <div class="card" style="margin-top: 20px;">
            <div class="card-header">
              <span class="card-title">Agent 配置信息</span>
            </div>
            <div class="card-body">
              <div class="info-grid">
                <div class="info-item">
                  <span class="info-label">配置文件路径</span>
                  <span class="info-value mono">/etc/mypatroni/config.yaml</span>
                </div>
                <div class="info-item">
                  <span class="info-label">日志文件路径</span>
                  <span class="info-value mono">/var/log/mypatroni/mypatroni.log</span>
                </div>
                <div class="info-item">
                  <span class="info-label">TTL</span>
                  <span class="info-value">30s</span>
                </div>
                <div class="info-item">
                  <span class="info-label">Loop Wait</span>
                  <span class="info-value">10s</span>
                </div>
              </div>
            </div>
          </div>
        </section>

        <!-- 实时日志 -->
        <section v-show="activeMenu === 'logs'" class="content-section">
          <div class="section-header">
            <h2>📋 实时日志</h2>
            <div class="header-actions">
              <select v-model="logSource" class="form-select" style="width: 150px;">
                <option value="all">全部节点</option>
                <option v-for="node in agentNodes" :key="node.node_id" :value="node.node_id">{{ node.name }}</option>
              </select>
              <button class="btn btn-primary" @click="fetchLogs" :disabled="fetchingLogs">
                🔄 刷新日志
              </button>
              <button class="btn btn-secondary" @click="clearLogs">清空</button>
              <label class="auto-refresh-toggle">
                <input type="checkbox" v-model="autoRefreshLogs" />
                <span>自动刷新</span>
              </label>
            </div>
          </div>

          <div class="card log-card">
            <div class="log-container" ref="logContainer">
              <div v-for="(log, index) in logs" :key="index" class="log-line" :class="getLogClass(log)">
                <span class="log-time">{{ log.time }}</span>
                <span class="log-level">{{ log.level }}</span>
                <span class="log-node" v-if="log.node">[{{ log.node }}]</span>
                <span class="log-message">{{ log.message }}</span>
              </div>
              <div v-if="logs.length === 0" class="log-empty">暂无日志，点击刷新获取</div>
            </div>
          </div>
        </section>

        <!-- 安装状态 -->
        <section v-show="activeMenu === 'install'" class="content-section">
          <div class="section-header">
            <h2>📦 安装状态</h2>
            <div class="header-actions">
              <button class="btn btn-primary" @click="refreshInstallStatus" :disabled="refreshingInstall">
                🔄 刷新状态
              </button>
              <button class="btn btn-secondary" @click="showRetryDialog" :disabled="!hasFailedNodes">
                🔁 重试安装
              </button>
            </div>
          </div>

          <div class="card">
            <div class="card-header">
              <span class="card-title">节点安装进度</span>
              <span class="tag" :class="'tag-' + getPhaseColor(cluster.phase)">{{ getPhaseText(cluster.phase) }}</span>
            </div>
            <div class="card-body">
              <div class="table-container">
                <table class="table">
                  <thead>
                    <tr>
                      <th>节点名称</th>
                      <th>IP 地址</th>
                      <th>角色</th>
                      <th>安装状态</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="host in cluster.hosts" :key="host.id">
                      <td>{{ host.name }}</td>
                      <td class="mono">{{ host.ip }}</td>
                      <td>
                        <span v-for="role in host.roles" :key="role" class="tag" :class="'tag-' + getRoleColor(role)" style="margin-right: 4px;">
                          {{ role }}
                        </span>
                      </td>
                      <td>
                        <span class="tag" :class="'tag-' + getStatusColor(host.status)">{{ host.status || 'pending' }}</span>
                      </td>
                      <td>
                        <button class="btn btn-sm btn-secondary" v-if="host.status !== 'completed'" @click="retryNodeInstall(host)">重试</button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </section>
      </template>
    </main>

    <!-- 对话框 -->
    <!-- 重试安装对话框 -->
    <el-dialog v-model="retryDialogVisible" title="重试安装" width="550px">
      <div class="dialog-alert">
        <p>选择要重试安装的节点和阶段：</p>
        <ul>
          <li>如果下载文件已存在且完整，将跳过下载直接解压</li>
          <li>已完成的节点不会被重复安装</li>
        </ul>
      </div>
      <div class="form-group">
        <label class="form-label">选择节点</label>
        <div class="checkbox-group">
          <label v-for="host in failedHosts" :key="host.id" class="checkbox-item">
            <input type="checkbox" :value="host.id" v-model="retryTargets" />
            <span>{{ host.name }} ({{ host.ip }}) - {{ host.status || 'pending' }}</span>
          </label>
        </div>
      </div>
      <div class="form-group">
        <label class="form-label">重试阶段</label>
        <select v-model="retryPhase" class="form-select">
          <option value="">自动检测</option>
          <option value="etcd">etcd</option>
          <option value="mysql">MySQL</option>
          <option value="agent">HA Agent</option>
        </select>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="retryDialogVisible = false">取消</button>
        <button class="btn btn-primary" @click="performRetryInstall" :disabled="retryLoading || retryTargets.length === 0">
          开始重试 ({{ retryTargets.length }} 个节点)
        </button>
      </template>
    </el-dialog>

    <!-- Switchover 确认对话框 -->
    <el-dialog v-model="switchoverDialogVisible" title="主从切换确认" width="500px">
      <div v-if="selectedNode" class="dialog-alert warning">
        <p>您即将执行主从切换操作，这将会：</p>
        <ul>
          <li>将当前主节点切换为从节点</li>
          <li>将 <strong>{{ selectedNode.name }}</strong> 提升为新的主节点</li>
          <li>可能会有短暂的服务中断</li>
        </ul>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="switchoverDialogVisible = false">取消</button>
        <button class="btn btn-danger" @click="confirmSwitchover" :disabled="switchoverLoading">确认切换</button>
      </template>
    </el-dialog>

    <!-- Agent 批量更新对话框 -->
    <el-dialog v-model="upgradeDialogVisible" title="批量更新 HA Agent" width="600px">
      <div class="dialog-alert">
        将使用本地的 mypatroni 二进制文件更新所有节点的 HA Agent。更新过程中 Agent 会短暂重启。
      </div>
      <div class="form-group">
        <label class="form-label">选择节点</label>
        <div class="checkbox-group">
          <label v-for="node in agentNodes" :key="node.node_id" class="checkbox-item">
            <input type="checkbox" :value="node.node_id" v-model="upgradeTargets" />
            <span>{{ node.name }} ({{ node.ip }})</span>
          </label>
        </div>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="upgradeDialogVisible = false">取消</button>
        <button class="btn btn-primary" @click="performBatchUpgrade" :disabled="upgradeLoading || upgradeTargets.length === 0">
          开始更新 ({{ upgradeTargets.length }} 个节点)
        </button>
      </template>
    </el-dialog>

    <!-- 一键修复主从对话框 -->
    <el-dialog v-model="repairDialogVisible" title="一键修复主从" width="550px">
      <div class="dialog-alert warning">
        <p><strong>此操作将执行以下步骤：</strong></p>
        <ul>
          <li>检测所有 MySQL 节点的实际状态（read_only、复制配置）</li>
          <li>自动识别真正的主节点（非只读且无复制配置的节点）</li>
          <li>更新 etcd 中的 leader_info 为实际主节点</li>
          <li>重新配置所有从节点的复制指向正确的主节点</li>
        </ul>
        <p class="warning-text">⚠️ 此操作会短暂中断复制，请在业务低峰期执行</p>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="repairDialogVisible = false">取消</button>
        <button class="btn btn-danger" @click="confirmRepair" :disabled="repairLoading">确认修复</button>
      </template>
    </el-dialog>

    <!-- 单节点修复对话框 -->
    <el-dialog v-model="repairNodeDialogVisible" title="修复异常节点" width="550px">
      <div v-if="selectedRepairNode" class="dialog-alert">
        <p><strong>将对节点 {{ selectedRepairNode.name }} ({{ selectedRepairNode.ip }}) 执行以下修复操作：</strong></p>
        <ul>
          <li>检查并启动 MySQL 服务（如果未运行）</li>
          <li>检查并启动 mypatroni HA Agent 服务（如果未运行）</li>
          <li>自动查找当前集群的主节点</li>
          <li>配置该节点为从节点，复制指向主节点</li>
          <li>设置为只读模式</li>
        </ul>
        <p class="info-text">💡 修复过程约需 15-30 秒，请耐心等待</p>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="repairNodeDialogVisible = false">取消</button>
        <button class="btn btn-primary" @click="confirmRepairNode" :disabled="repairNodeLoading">开始修复</button>
      </template>
    </el-dialog>
  </div>
</template>


<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  getCluster, getInstallStatus, getClusterStatus, getEtcdStatus, switchover, upgradeAgents, restartAgents, restartMySQL, getAgentLogs, repairReplication, retryInstall,
  type Cluster, type ClusterStatus, type NodeStatus
} from '@/api'

const route = useRoute()
const router = useRouter()

// 状态
const loading = ref(true)
const refreshing = ref(false)
const refreshingEtcd = ref(false)
const refreshingInstall = ref(false)
const refreshingAgent = ref(false)
const cluster = ref<Cluster | null>(null)
const clusterStatus = ref<ClusterStatus | null>(null)
const activeMenu = ref('overview')

// 切换相关
const switchoverDialogVisible = ref(false)
const switchoverLoading = ref(false)
const selectedNode = ref<NodeStatus | null>(null)

// Agent 更新相关
const upgradeDialogVisible = ref(false)
const upgradeLoading = ref(false)
const upgradeTargets = ref<string[]>([])

// 修复主从相关
const repairDialogVisible = ref(false)
const repairLoading = ref(false)

// 单节点修复相关
const repairNodeDialogVisible = ref(false)
const repairNodeLoading = ref(false)
const selectedRepairNode = ref<NodeStatus | null>(null)

// MySQL 自动刷新
const autoRefreshMySQL = ref(false)
let mysqlRefreshTimer: number | null = null

// 重试安装相关
const retryDialogVisible = ref(false)
const retryLoading = ref(false)
const retryTargets = ref<string[]>([])
const retryPhase = ref('')

// 日志相关
const logs = ref<Array<{ time: string; level: string; node?: string; message: string }>>([])
const logSource = ref('all')
const fetchingLogs = ref(false)
const autoRefreshLogs = ref(false)
const logContainer = ref<HTMLElement | null>(null)

let refreshTimer: number | null = null
let logTimer: number | null = null

// 计算属性
const mysqlNodeCount = computed(() => {
  if (!cluster.value) return 0
  return cluster.value.hosts.filter(h => h.roles.includes('master') || h.roles.includes('slave')).length
})

const etcdNodeCount = computed(() => {
  if (!cluster.value) return 0
  return cluster.value.hosts.filter(h => h.roles.includes('etcd')).length
})

const isClusterHealthy = computed(() => clusterStatus.value?.leader?.is_healthy ?? false)

const replicaNodes = computed(() => {
  if (!clusterStatus.value) return []
  const leaderId = clusterStatus.value.leader?.node_id
  return clusterStatus.value.nodes.filter(node => node.node_id !== leaderId)
})

const etcdNodes = computed(() => {
  if (!cluster.value) return []
  return cluster.value.hosts.filter(h => h.roles.includes('etcd'))
})

// etcd 健康状态
const etcdHealthStatus = ref<Record<string, boolean>>({})

const etcdNodesWithHealth = computed(() => {
  if (!cluster.value) return []
  return cluster.value.hosts
    .filter(h => h.roles.includes('etcd'))
    .map(h => ({
      ...h,
      is_healthy: etcdHealthStatus.value[h.ip] ?? false
    }))
})

const agentNodes = computed(() => {
  if (!clusterStatus.value) return []
  return clusterStatus.value.nodes
})

const agentHealthyCount = computed(() => {
  if (!clusterStatus.value) return 0
  return clusterStatus.value.nodes.filter(n => n.is_healthy).length
})

const hasFailedNodes = computed(() => {
  if (!cluster.value) return false
  return cluster.value.hosts.some(h => h.status !== 'completed')
})

const failedHosts = computed(() => {
  if (!cluster.value) return []
  return cluster.value.hosts.filter(h => h.status !== 'completed')
})

// 方法
const loadCluster = async () => {
  loading.value = true
  try {
    const { data } = await getCluster(route.params.id as string)
    cluster.value = data
  } catch { ElMessage.error('加载集群信息失败') }
  loading.value = false
}

const loadClusterStatus = async () => {
  if (!cluster.value) return
  try {
    const { data } = await getClusterStatus(cluster.value.id)
    clusterStatus.value = data
  } catch (error) { console.error('加载集群状态失败:', error) }
}

const refreshMySQLStatus = async () => {
  refreshing.value = true
  await loadClusterStatus()
  ElMessage.success('MySQL 状态已刷新')
  refreshing.value = false
}

const refreshEtcdStatus = async () => {
  refreshingEtcd.value = true
  await loadCluster()
  await loadEtcdStatus()
  ElMessage.success('etcd 状态已刷新')
  refreshingEtcd.value = false
}

const loadEtcdStatus = async () => {
  if (!cluster.value) return
  try {
    const { data } = await getEtcdStatus(cluster.value.id)
    // 更新 etcd 健康状态
    const healthMap: Record<string, boolean> = {}
    for (const node of data.nodes) {
      healthMap[node.ip] = node.is_healthy
    }
    etcdHealthStatus.value = healthMap
  } catch (error) {
    console.error('加载 etcd 状态失败:', error)
  }
}

const refreshInstallStatus = async () => {
  if (!cluster.value) return
  refreshingInstall.value = true
  try {
    const { data } = await getInstallStatus(cluster.value.id)
    cluster.value.hosts = data.hosts
    ElMessage.success('安装状态已刷新')
  } catch { ElMessage.error('刷新失败') }
  refreshingInstall.value = false
}

const refreshAgentStatus = async () => {
  refreshingAgent.value = true
  await loadClusterStatus()
  ElMessage.success('Agent 状态已刷新')
  refreshingAgent.value = false
}

const showSwitchoverDialog = (node: NodeStatus) => {
  selectedNode.value = node
  switchoverDialogVisible.value = true
}

const confirmSwitchover = async () => {
  if (!selectedNode.value || !cluster.value) return
  switchoverLoading.value = true
  try {
    await switchover(cluster.value.id, selectedNode.value.node_id, '手动切换')
    ElMessage.success('切换请求已提交')
    switchoverDialogVisible.value = false
    addLog('INFO', `主从切换已发起，目标节点: ${selectedNode.value.name}`)
    setTimeout(() => loadClusterStatus(), 3000)
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '切换失败')
  }
  switchoverLoading.value = false
}

const showUpgradeDialog = () => {
  upgradeTargets.value = agentNodes.value.map(n => n.node_id)
  upgradeDialogVisible.value = true
}

const performBatchUpgrade = async () => {
  if (!cluster.value || upgradeTargets.value.length === 0) return
  upgradeLoading.value = true
  try {
    const { data } = await upgradeAgents(cluster.value.id, upgradeTargets.value, false)
    
    // 检查是否需要强制更新
    if (data.status === 'no_update_needed') {
      upgradeLoading.value = false
      // 显示确认对话框
      const confirmed = await ElMessageBox.confirm(
        `所有节点版本已是最新 (${data.current_version})，是否强制更新？`,
        '版本相同',
        {
          confirmButtonText: '强制更新',
          cancelButtonText: '取消',
          type: 'warning'
        }
      ).catch(() => false)
      
      if (confirmed) {
        upgradeLoading.value = true
        await upgradeAgents(cluster.value.id, upgradeTargets.value, true)
        ElMessage.success('Agent 强制更新已开始')
        upgradeDialogVisible.value = false
        addLog('INFO', `开始强制更新 ${upgradeTargets.value.length} 个节点的 HA Agent`)
        setTimeout(() => loadClusterStatus(), 5000)
        upgradeLoading.value = false
      }
      return
    }
    
    ElMessage.success('Agent 更新已开始')
    upgradeDialogVisible.value = false
    addLog('INFO', `开始更新 ${upgradeTargets.value.length} 个节点的 HA Agent`)
    setTimeout(() => loadClusterStatus(), 5000)
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '更新失败')
  }
  upgradeLoading.value = false
}

const restartAgent = async (node: NodeStatus) => {
  if (!cluster.value) return
  try {
    await restartAgents(cluster.value.id, [node.node_id])
    ElMessage.success(`正在重启 ${node.name} 的 Agent`)
    addLog('INFO', `重启 Agent: ${node.name}`)
  } catch (error: any) {
    ElMessage.error('重启失败')
  }
}

const upgradeAgent = async (node: NodeStatus) => {
  if (!cluster.value) return
  try {
    const { data } = await upgradeAgents(cluster.value.id, [node.node_id], false)
    
    // 检查是否需要强制更新
    if (data.status === 'no_update_needed') {
      const confirmed = await ElMessageBox.confirm(
        `节点 ${node.name} 版本已是最新 (${data.current_version})，是否强制更新？`,
        '版本相同',
        {
          confirmButtonText: '强制更新',
          cancelButtonText: '取消',
          type: 'warning'
        }
      ).catch(() => false)
      
      if (confirmed) {
        await upgradeAgents(cluster.value.id, [node.node_id], true)
        ElMessage.success(`正在强制更新 ${node.name} 的 Agent`)
        addLog('INFO', `强制更新 Agent: ${node.name}`)
      }
      return
    }
    
    ElMessage.success(`正在更新 ${node.name} 的 Agent`)
    addLog('INFO', `更新 Agent: ${node.name}`)
  } catch (error: any) {
    ElMessage.error('更新失败')
  }
}

// 修复主从相关
const showRepairDialog = () => { repairDialogVisible.value = true }

const confirmRepair = async () => {
  if (!cluster.value) return
  repairLoading.value = true
  try {
    const { data } = await repairReplication(cluster.value.id)
    repairDialogVisible.value = false
    
    // 检查返回状态
    if (data.status === 'healthy') {
      ElMessage.success(`集群复制状态正常，无需修复。主节点: ${data.master} (${data.master_ip})`)
      addLog('INFO', `集群状态检查完成，复制正常，主节点: ${data.master}`)
    } else {
      ElMessage.success('主从修复已开始，请查看日志了解进度')
      addLog('INFO', '一键修复主从已发起')
      setTimeout(() => loadClusterStatus(), 5000)
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '修复失败')
  }
  repairLoading.value = false
}

const showRepairNodeDialog = (node: NodeStatus) => {
  selectedRepairNode.value = node
  repairNodeDialogVisible.value = true
}

const confirmRepairNode = async () => {
  if (!selectedRepairNode.value || !cluster.value) return
  repairNodeLoading.value = true
  try {
    const response = await fetch(`/api/v1/clusters/${cluster.value.id}/nodes/${selectedRepairNode.value.node_id}/repair`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.error || '修复失败')
    }
    ElMessage.success(`节点 ${selectedRepairNode.value.name} 修复已开始，请稍候...`)
    repairNodeDialogVisible.value = false
    addLog('INFO', `开始修复节点: ${selectedRepairNode.value.name}`)
    setTimeout(() => loadClusterStatus(), 10000)
    setTimeout(() => loadClusterStatus(), 20000)
  } catch (error: any) {
    ElMessage.error(error.message || '修复失败')
  }
  repairNodeLoading.value = false
}

// 重启单个节点的 MySQL
const restartMySQLNode = async (node: NodeStatus) => {
  if (!cluster.value) return
  try {
    await restartMySQL(cluster.value.id, [node.node_id])
    ElMessage.success(`正在重启 ${node.name} 的 MySQL 服务`)
    addLog('INFO', `重启 MySQL: ${node.name}`)
    setTimeout(() => loadClusterStatus(), 5000)
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '重启失败')
  }
}

// 重试安装相关
const showRetryDialog = () => {
  retryTargets.value = failedHosts.value.map(h => h.id || '')
  retryPhase.value = ''
  retryDialogVisible.value = true
}

const retryNodeInstall = async (host: any) => {
  if (!cluster.value) return
  try {
    await retryInstall(cluster.value.id, [host.id])
    ElMessage.success(`正在重试安装节点 ${host.name}`)
    addLog('INFO', `重试安装节点: ${host.name}`)
    setTimeout(() => refreshInstallStatus(), 5000)
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '重试失败')
  }
}

const performRetryInstall = async () => {
  if (!cluster.value || retryTargets.value.length === 0) return
  retryLoading.value = true
  try {
    await retryInstall(cluster.value.id, retryTargets.value, retryPhase.value || undefined)
    ElMessage.success('重试安装已开始')
    retryDialogVisible.value = false
    addLog('INFO', `开始重试安装 ${retryTargets.value.length} 个节点`)
    setTimeout(() => refreshInstallStatus(), 5000)
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '重试失败')
  }
  retryLoading.value = false
}

// 日志相关
const addLog = (level: string, message: string, node?: string) => {
  const now = new Date()
  const time = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`
  logs.value.push({ time, level, node, message })
  if (logs.value.length > 500) logs.value.shift()
  nextTick(() => {
    if (logContainer.value) logContainer.value.scrollTop = logContainer.value.scrollHeight
  })
}

const fetchLogs = async () => {
  if (!cluster.value) return
  fetchingLogs.value = true
  try {
    const { data } = await getAgentLogs(cluster.value.id, logSource.value === 'all' ? undefined : logSource.value)
    if (data.logs && data.logs.length > 0) {
      data.logs.forEach((log: any) => {
        addLog(log.level || 'INFO', log.message, log.node)
      })
    }
  } catch {
    addLog('ERROR', '获取日志失败')
  }
  fetchingLogs.value = false
}

const clearLogs = () => { logs.value = [] }

const getLogClass = (log: { level: string }) => {
  if (log.level === 'ERROR') return 'log-error'
  if (log.level === 'WARN') return 'log-warn'
  if (log.level === 'INFO') return 'log-info'
  return ''
}

watch(autoRefreshLogs, (val) => {
  if (val) {
    logTimer = window.setInterval(fetchLogs, 5000)
  } else if (logTimer) {
    clearInterval(logTimer)
    logTimer = null
  }
})

watch(autoRefreshMySQL, (val) => {
  if (val) {
    mysqlRefreshTimer = window.setInterval(async () => {
      await loadClusterStatus()
    }, 5000)
  } else if (mysqlRefreshTimer) {
    clearInterval(mysqlRefreshTimer)
    mysqlRefreshTimer = null
  }
})

// 辅助方法
const getStatusColor = (status?: string) => {
  if (status === 'completed') return 'success'
  if (status === 'pending') return 'warning'
  if (status?.startsWith('installing')) return 'primary'
  if (status?.startsWith('failed')) return 'danger'
  return 'info'
}

const getRoleColor = (role: string) => {
  if (role === 'master') return 'warning'
  if (role === 'slave') return 'success'
  if (role === 'etcd') return 'info'
  return 'info'
}

const getPhaseColor = (phase?: string) => {
  if (phase === 'completed') return 'success'
  if (phase?.startsWith('failed')) return 'danger'
  if (phase?.startsWith('phase')) return 'primary'
  return 'info'
}

const getPhaseText = (phase?: string) => {
  const phaseMap: Record<string, string> = {
    'phase1_etcd': '阶段1: etcd部署中', 'phase2_mysql': '阶段2: MySQL部署中',
    'phase3_replication': '阶段3: 主从配置中', 'phase4_agent': '阶段4: Agent部署中',
    'phase5_verify': '阶段5: 验证中', 'completed': '安装完成', 'completed_no_agent': '完成(无Agent)',
  }
  if (phase?.startsWith('failed')) return '安装失败'
  return phaseMap[phase || ''] || phase || '未开始'
}

onMounted(async () => {
  await loadCluster()
  await loadClusterStatus()
  await loadEtcdStatus()
  refreshTimer = window.setInterval(() => loadClusterStatus(), 15000)
  addLog('INFO', '集群详情页面已加载')
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (logTimer) clearInterval(logTimer)
  if (mysqlRefreshTimer) clearInterval(mysqlRefreshTimer)
})
</script>


<style scoped lang="scss">
.cluster-detail-page {
  display: flex;
  height: calc(100vh - 64px - 64px);
  background: var(--bg-app);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

// 侧边栏
.detail-sidebar {
  width: 220px;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.sidebar-header {
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 14px;
  border-bottom: 1px solid var(--border-color);
  
  .cluster-badge {
    width: 44px;
    height: 44px;
    background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
    border-radius: var(--radius-md);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 22px;
    box-shadow: 0 4px 12px rgba(6, 182, 212, 0.3);
  }
  
  .cluster-title {
    flex: 1;
    min-width: 0;
    
    .name {
      display: block;
      font-size: 15px;
      font-weight: 600;
      color: var(--text-primary);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .id {
      display: block;
      font-size: 11px;
      color: var(--text-muted);
      font-family: monospace;
    }
  }
}

.sidebar-nav {
  flex: 1;
  padding: 16px 10px;
  overflow-y: auto;
  
  .nav-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 11px 14px;
    margin: 3px 0;
    border-radius: var(--radius-md);
    color: var(--text-secondary);
    cursor: pointer;
    transition: var(--transition);
    font-size: 14px;
    
    .nav-icon {
      font-size: 16px;
      width: 22px;
      text-align: center;
    }
    
    &:hover {
      background: var(--bg-hover);
      color: var(--text-primary);
    }
    
    &.active {
      background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
      color: white;
      box-shadow: 0 4px 12px rgba(6, 182, 212, 0.3);
    }
  }
}

.sidebar-footer {
  padding: 14px;
  border-top: 1px solid var(--border-color);
  
  .back-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 10px 14px;
    background: transparent;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    color: var(--text-secondary);
    cursor: pointer;
    transition: var(--transition);
    font-size: 14px;
    
    &:hover {
      background: var(--bg-hover);
      color: var(--text-primary);
      border-color: var(--border-light);
    }
  }
}

// 主内容区
.detail-content {
  flex: 1;
  padding: 28px;
  overflow-y: auto;
}

.content-section {
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

// 日志样式
.log-card .card-body {
  padding: 0 !important;
}

.log-container {
  height: 450px;
  overflow-y: auto;
  background: #0d0d12;
  border-radius: 0 0 var(--radius-lg) var(--radius-lg);
  padding: 16px;
  font-family: 'SF Mono', Monaco, Consolas, monospace;
  font-size: 12px;
}

.log-line {
  padding: 4px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  display: flex;
  gap: 12px;
  color: #b0b0c0;
  
  .log-time {
    color: var(--success-text);
    min-width: 70px;
  }
  
  .log-level {
    min-width: 50px;
    font-weight: 600;
  }
  
  .log-node {
    color: var(--primary);
  }
  
  .log-message {
    flex: 1;
    word-break: break-all;
  }
  
  &.log-error {
    color: var(--danger-text);
    .log-level { color: var(--danger-text); }
  }
  
  &.log-warn {
    color: var(--warning-text);
    .log-level { color: var(--warning-text); }
  }
  
  &.log-info {
    .log-level { color: var(--info-text); }
  }
}

.log-empty {
  color: var(--text-muted);
  text-align: center;
  padding: 60px;
}

// 表单选择框
.form-select {
  padding: 10px 14px;
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  color: var(--text-primary);
  font-size: 14px;
  cursor: pointer;
  
  &:focus {
    outline: none;
    border-color: var(--primary);
  }
  
  option {
    background: var(--bg-card);
    color: var(--text-primary);
  }
}

// Element Plus 对话框覆盖
:deep(.el-dialog) {
  background: var(--bg-card) !important;
  border-radius: var(--radius-lg) !important;
  border: 1px solid var(--border-color) !important;
  
  .el-dialog__header {
    border-bottom: 1px solid var(--border-color);
    padding: 20px 24px;
  }
  
  .el-dialog__title {
    color: var(--text-primary) !important;
    font-size: 18px;
    font-weight: 600;
  }
  
  .el-dialog__body {
    padding: 24px;
  }
  
  .el-dialog__footer {
    border-top: 1px solid var(--border-color);
    padding: 16px 24px;
    display: flex;
    justify-content: flex-end;
    gap: 12px;
  }
}
</style>
