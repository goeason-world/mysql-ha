<template>
  <div class="cluster-detail-layout">
    <!-- 左侧菜单 -->
    <div class="sidebar">
      <div class="sidebar-header">
        <el-icon><DataBoard /></el-icon>
        <span>{{ cluster?.name || '集群管理' }}</span>
      </div>
      <el-menu :default-active="activeMenu" class="sidebar-menu" @select="handleMenuSelect">
        <el-menu-item index="overview">
          <el-icon><House /></el-icon>
          <span>集群概览</span>
        </el-menu-item>
        <el-menu-item index="mysql">
          <el-icon><Coin /></el-icon>
          <span>MySQL 控制</span>
        </el-menu-item>
        <el-menu-item index="etcd">
          <el-icon><Connection /></el-icon>
          <span>etcd 控制</span>
        </el-menu-item>
        <el-menu-item index="agent">
          <el-icon><Monitor /></el-icon>
          <span>HA Agent 控制</span>
        </el-menu-item>
        <el-menu-item index="logs">
          <el-icon><Document /></el-icon>
          <span>实时日志</span>
        </el-menu-item>
        <el-menu-item index="install">
          <el-icon><Download /></el-icon>
          <span>安装状态</span>
        </el-menu-item>
      </el-menu>
      <div class="sidebar-footer">
        <el-button text @click="router.push('/clusters')">
          <el-icon><Back /></el-icon>
          返回列表
        </el-button>
      </div>
    </div>

    <!-- 右侧内容区 -->
    <div class="main-content">
      <el-card v-if="loading" class="content-card">
        <div class="loading"><el-icon class="is-loading"><Loading /></el-icon> 加载中...</div>
      </el-card>

      <template v-else-if="cluster">
        <!-- 集群概览 -->
        <div v-show="activeMenu === 'overview'" class="content-section">
          <h2 class="section-title">📊 集群概览</h2>
          <el-row :gutter="20">
            <el-col :span="6">
              <el-card class="stat-card">
                <div class="stat-icon mysql-icon"><el-icon><Coin /></el-icon></div>
                <div class="stat-info">
                  <div class="stat-value">{{ mysqlNodeCount }}</div>
                  <div class="stat-label">MySQL 节点</div>
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card class="stat-card">
                <div class="stat-icon etcd-icon"><el-icon><Connection /></el-icon></div>
                <div class="stat-info">
                  <div class="stat-value">{{ etcdNodeCount }}</div>
                  <div class="stat-label">etcd 节点</div>
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card class="stat-card">
                <div class="stat-icon agent-icon"><el-icon><Monitor /></el-icon></div>
                <div class="stat-info">
                  <div class="stat-value">{{ agentHealthyCount }}/{{ mysqlNodeCount }}</div>
                  <div class="stat-label">Agent 在线</div>
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card class="stat-card">
                <div class="stat-icon status-icon" :class="{ healthy: isClusterHealthy }">
                  <el-icon><CircleCheck /></el-icon>
                </div>
                <div class="stat-info">
                  <div class="stat-value">{{ isClusterHealthy ? '正常' : '异常' }}</div>
                  <div class="stat-label">集群状态</div>
                </div>
              </el-card>
            </el-col>
          </el-row>
          <el-card class="info-card" style="margin-top: 20px;">
            <template #header><span>基本信息</span></template>
            <el-descriptions :column="2" border>
              <el-descriptions-item label="集群ID">{{ cluster.id }}</el-descriptions-item>
              <el-descriptions-item label="集群名称">{{ cluster.name }}</el-descriptions-item>
              <el-descriptions-item label="MySQL 版本">{{ cluster.mysql_version }}</el-descriptions-item>
              <el-descriptions-item label="etcd 版本">{{ cluster.etcd_version }}</el-descriptions-item>
              <el-descriptions-item label="安装路径">{{ cluster.install_path }}</el-descriptions-item>
              <el-descriptions-item label="数据目录">{{ cluster.data_path }}</el-descriptions-item>
              <el-descriptions-item label="MySQL 端口">{{ cluster.settings?.mysql_port }}</el-descriptions-item>
              <el-descriptions-item label="HA Agent 端口">{{ cluster.settings?.ha_agent_port }}</el-descriptions-item>
            </el-descriptions>
          </el-card>
        </div>

        <!-- MySQL 控制 -->
        <div v-show="activeMenu === 'mysql'" class="content-section">
          <div class="section-header">
            <h2 class="section-title">🗄️ MySQL 集群控制</h2>
            <div>
              <el-button @click="refreshMySQLStatus" :loading="refreshing" type="primary">
                <el-icon><Refresh /></el-icon> 刷新状态
              </el-button>
              <el-button @click="showRepairDialog" type="danger" style="margin-left: 10px;">
                <el-icon><WarnTriangleFilled /></el-icon> 一键修复主从
              </el-button>
            </div>
          </div>
          <el-card class="control-card">
            <template #header>
              <div class="card-header">
                <span>👑 主节点 (Leader)</span>
                <el-tag v-if="clusterStatus?.leader" type="success">运行中</el-tag>
                <el-tag v-else type="danger">无主节点</el-tag>
              </div>
            </template>
            <div v-if="clusterStatus?.leader" class="node-detail">
              <el-descriptions :column="2" border>
                <el-descriptions-item label="节点名称">{{ clusterStatus.leader.name }}</el-descriptions-item>
                <el-descriptions-item label="IP 地址">{{ clusterStatus.leader.ip }}</el-descriptions-item>
                <el-descriptions-item label="健康状态">
                  <el-tag :type="clusterStatus.leader.is_healthy ? 'success' : 'danger'">
                    {{ clusterStatus.leader.is_healthy ? '健康' : '异常' }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="安装状态">
                  <el-tag :type="getStatusType(clusterStatus.leader.install_status)">{{ clusterStatus.leader.install_status }}</el-tag>
                </el-descriptions-item>
              </el-descriptions>
            </div>
            <el-empty v-else description="暂无主节点信息" />
          </el-card>
          <el-card class="control-card" style="margin-top: 20px;">
            <template #header><span>🔄 从节点 (Replicas)</span></template>
            <el-table :data="replicaNodes" style="width: 100%">
              <el-table-column prop="name" label="节点名称" />
              <el-table-column prop="ip" label="IP 地址" />
              <el-table-column label="健康状态">
                <template #default="{ row }">
                  <el-tag :type="row.is_healthy ? 'success' : 'danger'">{{ row.is_healthy ? '健康' : '异常' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="复制延迟">
                <template #default="{ row }">{{ row.replication_lag !== undefined ? row.replication_lag + 's' : '-' }}</template>
              </el-table-column>
              <el-table-column label="操作" width="220">
                <template #default="{ row }">
                  <el-button size="small" type="primary" :disabled="!row.is_healthy || row.install_status !== 'completed'" @click="showSwitchoverDialog(row)">切换为主</el-button>
                  <el-button size="small" type="warning" :disabled="row.is_healthy" @click="showRepairNodeDialog(row)">修复</el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-if="replicaNodes.length === 0" description="暂无从节点" />
          </el-card>
        </div>

        <!-- etcd 控制 -->
        <div v-show="activeMenu === 'etcd'" class="content-section">
          <div class="section-header">
            <h2 class="section-title">🔗 etcd 集群控制</h2>
            <el-button @click="refreshEtcdStatus" :loading="refreshingEtcd" type="primary">
              <el-icon><Refresh /></el-icon> 刷新状态
            </el-button>
          </div>
          <el-card class="control-card">
            <template #header><span>etcd 节点列表</span></template>
            <el-table :data="etcdNodes" style="width: 100%">
              <el-table-column prop="name" label="节点名称" />
              <el-table-column prop="ip" label="IP 地址" />
              <el-table-column label="客户端端口"><template #default="{ row }">{{ row.ip }}:2379</template></el-table-column>
              <el-table-column label="Peer 端口"><template #default="{ row }">{{ row.ip }}:2380</template></el-table-column>
              <el-table-column label="安装状态">
                <template #default="{ row }"><el-tag :type="getStatusType(row.status)">{{ row.status }}</el-tag></template>
              </el-table-column>
            </el-table>
          </el-card>
          <el-card class="control-card" style="margin-top: 20px;">
            <template #header><span>etcd 集群信息</span></template>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="集群端点">
                <el-tag v-for="node in etcdNodes" :key="node.id" style="margin-right: 8px;">http://{{ node.ip }}:2379</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="集群 Token">mysql-ha-etcd-cluster</el-descriptions-item>
            </el-descriptions>
          </el-card>
        </div>

        <!-- HA Agent 控制 -->
        <div v-show="activeMenu === 'agent'" class="content-section">
          <div class="section-header">
            <h2 class="section-title">🤖 HA Agent 控制</h2>
            <div>
              <el-button @click="refreshAgentStatus" :loading="refreshingAgent" type="primary">
                <el-icon><Refresh /></el-icon> 刷新状态
              </el-button>
              <el-button @click="showUpgradeDialog" type="warning" style="margin-left: 10px;">
                <el-icon><Upload /></el-icon> 批量更新 Agent
              </el-button>
            </div>
          </div>
          <el-card class="control-card">
            <template #header><span>Agent 节点状态</span></template>
            <el-table :data="agentNodes" style="width: 100%">
              <el-table-column prop="name" label="节点名称" />
              <el-table-column prop="ip" label="IP 地址" />
              <el-table-column label="Agent 状态">
                <template #default="{ row }">
                  <el-tag :type="row.is_healthy ? 'success' : 'danger'">{{ row.is_healthy ? '运行中' : '离线' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="API 端口">
                <template #default>{{ cluster.settings?.ha_agent_port || 8080 }}</template>
              </el-table-column>
              <el-table-column label="操作" width="200">
                <template #default="{ row }">
                  <el-button size="small" type="primary" @click="restartAgent(row)">重启</el-button>
                  <el-button size="small" type="warning" @click="upgradeAgent(row)">更新</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
          <el-card class="control-card" style="margin-top: 20px;">
            <template #header><span>Agent 配置信息</span></template>
            <el-descriptions :column="2" border>
              <el-descriptions-item label="配置文件路径">/etc/mypatroni/config.yaml</el-descriptions-item>
              <el-descriptions-item label="日志文件路径">/var/log/mypatroni/mypatroni.log</el-descriptions-item>
              <el-descriptions-item label="TTL">{{ cluster.settings?.ha_agent_port ? '30s' : '-' }}</el-descriptions-item>
              <el-descriptions-item label="Loop Wait">10s</el-descriptions-item>
            </el-descriptions>
          </el-card>
        </div>

        <!-- 实时日志 -->
        <div v-show="activeMenu === 'logs'" class="content-section">
          <div class="section-header">
            <h2 class="section-title">📋 实时日志</h2>
            <div>
              <el-select v-model="logSource" style="width: 150px; margin-right: 10px;">
                <el-option label="全部节点" value="all" />
                <el-option v-for="node in agentNodes" :key="node.node_id" :label="node.name" :value="node.node_id" />
              </el-select>
              <el-button @click="fetchLogs" :loading="fetchingLogs" type="primary">
                <el-icon><Refresh /></el-icon> 刷新日志
              </el-button>
              <el-button @click="clearLogs" type="info">清空</el-button>
              <el-switch v-model="autoRefreshLogs" active-text="自动刷新" style="margin-left: 15px;" />
            </div>
          </div>
          <el-card class="control-card log-card">
            <div class="log-container" ref="logContainer">
              <div v-for="(log, index) in logs" :key="index" class="log-line" :class="getLogClass(log)">
                <span class="log-time">{{ log.time }}</span>
                <span class="log-level">{{ log.level }}</span>
                <span class="log-node" v-if="log.node">[{{ log.node }}]</span>
                <span class="log-message">{{ log.message }}</span>
              </div>
              <div v-if="logs.length === 0" class="log-empty">暂无日志，点击刷新获取</div>
            </div>
          </el-card>
        </div>

        <!-- 安装状态 -->
        <div v-show="activeMenu === 'install'" class="content-section">
          <div class="section-header">
            <h2 class="section-title">📦 安装状态</h2>
            <el-button @click="refreshInstallStatus" :loading="refreshingInstall" type="primary">
              <el-icon><Refresh /></el-icon> 刷新状态
            </el-button>
          </div>
          <el-card class="control-card">
            <template #header>
              <div class="card-header">
                <span>节点安装进度</span>
                <el-tag :type="getPhaseType(cluster.phase)">{{ getPhaseText(cluster.phase) }}</el-tag>
              </div>
            </template>
            <el-table :data="cluster.hosts" style="width: 100%">
              <el-table-column prop="name" label="节点名称" />
              <el-table-column prop="ip" label="IP 地址" />
              <el-table-column label="角色">
                <template #default="{ row }">
                  <el-tag v-for="role in row.roles" :key="role" :type="getRoleType(role)" style="margin-right: 4px;">{{ role }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="安装状态">
                <template #default="{ row }"><el-tag :type="getStatusType(row.status)">{{ row.status || 'pending' }}</el-tag></template>
              </el-table-column>
            </el-table>
          </el-card>
        </div>
      </template>
    </div>

    <!-- Switchover 确认对话框 -->
    <el-dialog v-model="switchoverDialogVisible" title="主从切换确认" width="500px">
      <div v-if="selectedNode">
        <el-alert type="warning" :closable="false" style="margin-bottom: 20px;">
          <p>您即将执行主从切换操作，这将会：</p>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>将当前主节点切换为从节点</li>
            <li>将 <strong>{{ selectedNode.name }}</strong> 提升为新的主节点</li>
            <li>可能会有短暂的服务中断</li>
          </ul>
        </el-alert>
      </div>
      <template #footer>
        <el-button @click="switchoverDialogVisible = false">取消</el-button>
        <el-button type="danger" @click="confirmSwitchover" :loading="switchoverLoading">确认切换</el-button>
      </template>
    </el-dialog>

    <!-- Agent 批量更新对话框 -->
    <el-dialog v-model="upgradeDialogVisible" title="批量更新 HA Agent" width="600px">
      <el-alert type="info" :closable="false" style="margin-bottom: 20px;">
        将使用本地的 mypatroni 二进制文件更新所有节点的 HA Agent。更新过程中 Agent 会短暂重启。
      </el-alert>
      <el-form label-width="100px">
        <el-form-item label="选择节点">
          <el-checkbox-group v-model="upgradeTargets">
            <el-checkbox v-for="node in agentNodes" :key="node.node_id" :label="node.node_id">
              {{ node.name }} ({{ node.ip }})
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="upgradeDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="performBatchUpgrade" :loading="upgradeLoading" :disabled="upgradeTargets.length === 0">
          开始更新 ({{ upgradeTargets.length }} 个节点)
        </el-button>
      </template>
    </el-dialog>

    <!-- 一键修复主从对话框 -->
    <el-dialog v-model="repairDialogVisible" title="一键修复主从" width="550px">
      <el-alert type="warning" :closable="false" style="margin-bottom: 20px;">
        <p><strong>此操作将执行以下步骤：</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>检测所有 MySQL 节点的实际状态（read_only、复制配置）</li>
          <li>自动识别真正的主节点（非只读且无复制配置的节点）</li>
          <li>更新 etcd 中的 leader_info 为实际主节点</li>
          <li>重新配置所有从节点的复制指向正确的主节点</li>
        </ul>
        <p style="color: #E6A23C; margin-top: 10px;">⚠️ 此操作会短暂中断复制，请在业务低峰期执行</p>
      </el-alert>
      <template #footer>
        <el-button @click="repairDialogVisible = false">取消</el-button>
        <el-button type="danger" @click="confirmRepair" :loading="repairLoading">确认修复</el-button>
      </template>
    </el-dialog>

    <!-- 单节点修复对话框 -->
    <el-dialog v-model="repairNodeDialogVisible" title="修复异常节点" width="550px">
      <div v-if="selectedRepairNode">
        <el-alert type="info" :closable="false" style="margin-bottom: 20px;">
          <p><strong>将对节点 {{ selectedRepairNode.name }} ({{ selectedRepairNode.ip }}) 执行以下修复操作：</strong></p>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>检查并启动 MySQL 服务（如果未运行）</li>
            <li>检查并启动 mypatroni HA Agent 服务（如果未运行）</li>
            <li>自动查找当前集群的主节点</li>
            <li>配置该节点为从节点，复制指向主节点</li>
            <li>设置为只读模式</li>
          </ul>
          <p style="color: #409EFF; margin-top: 10px;">💡 修复过程约需 15-30 秒，请耐心等待</p>
        </el-alert>
      </div>
      <template #footer>
        <el-button @click="repairNodeDialogVisible = false">取消</el-button>
        <el-button type="warning" @click="confirmRepairNode" :loading="repairNodeLoading">开始修复</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Loading, House, Coin, Connection, Download, Back, Refresh,
  DataBoard, CircleCheck, Monitor, Document, Upload, WarnTriangleFilled
} from '@element-plus/icons-vue'
import {
  getCluster, getInstallStatus, getClusterStatus, switchover, upgradeAgents, getAgentLogs, repairReplication,
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
  // 过滤掉 leader 节点（通过 node_id 匹配），其余都是从节点
  const leaderId = clusterStatus.value.leader?.node_id
  return clusterStatus.value.nodes.filter(node => node.node_id !== leaderId)
})

const etcdNodes = computed(() => {
  if (!cluster.value) return []
  return cluster.value.hosts.filter(h => h.roles.includes('etcd'))
})

const agentNodes = computed(() => {
  if (!clusterStatus.value) return []
  return clusterStatus.value.nodes
})

const agentHealthyCount = computed(() => {
  if (!clusterStatus.value) return 0
  return clusterStatus.value.nodes.filter(n => n.is_healthy).length
})

// 方法
const handleMenuSelect = (index: string) => { activeMenu.value = index }

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
  ElMessage.success('etcd 状态已刷新')
  refreshingEtcd.value = false
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
    await upgradeAgents(cluster.value.id, upgradeTargets.value)
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
    await upgradeAgents(cluster.value.id, [node.node_id])
    ElMessage.success(`正在重启 ${node.name} 的 Agent`)
    addLog('INFO', `重启 Agent: ${node.name}`)
  } catch (error: any) {
    ElMessage.error('重启失败')
  }
}

const upgradeAgent = async (node: NodeStatus) => {
  if (!cluster.value) return
  try {
    await upgradeAgents(cluster.value.id, [node.node_id])
    ElMessage.success(`正在更新 ${node.name} 的 Agent`)
    addLog('INFO', `更新 Agent: ${node.name}`)
  } catch (error: any) {
    ElMessage.error('更新失败')
  }
}

// 修复主从相关
const showRepairDialog = () => {
  repairDialogVisible.value = true
}

const confirmRepair = async () => {
  if (!cluster.value) return
  repairLoading.value = true
  try {
    await repairReplication(cluster.value.id)
    ElMessage.success('主从修复已开始，请查看日志了解进度')
    repairDialogVisible.value = false
    addLog('INFO', '一键修复主从已发起')
    // 延迟刷新状态
    setTimeout(() => loadClusterStatus(), 5000)
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '修复失败')
  }
  repairLoading.value = false
}

// 单节点修复相关
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
    // 延迟刷新状态，给修复过程一些时间
    setTimeout(() => loadClusterStatus(), 10000)
    setTimeout(() => loadClusterStatus(), 20000)
  } catch (error: any) {
    ElMessage.error(error.message || '修复失败')
  }
  repairNodeLoading.value = false
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

const getStatusType = (status?: string) => {
  if (status === 'completed') return 'success'
  if (status === 'pending') return 'warning'
  if (status?.startsWith('installing')) return 'primary'
  if (status?.startsWith('failed')) return 'danger'
  return 'info'
}

const getRoleType = (role: string) => {
  if (role === 'master') return 'warning'
  if (role === 'slave') return 'success'
  if (role === 'etcd') return 'info'
  return 'info'
}

const getPhaseType = (phase?: string) => {
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
  refreshTimer = window.setInterval(() => loadClusterStatus(), 15000)
  addLog('INFO', '集群详情页面已加载')
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (logTimer) clearInterval(logTimer)
})
</script>

<style scoped lang="scss">
.cluster-detail-layout { display: flex; min-height: calc(100vh - 100px); gap: 20px; }
.sidebar { width: 220px; background: linear-gradient(180deg, #1a1a2e 0%, #16213e 100%); border-radius: 16px; padding: 20px 0; display: flex; flex-direction: column; }
.sidebar-header { display: flex; align-items: center; gap: 10px; padding: 0 20px 20px; border-bottom: 1px solid rgba(255, 255, 255, 0.1); color: #fff; font-size: 16px; font-weight: 600; }
.sidebar-menu { background: transparent; border: none; flex: 1;
  :deep(.el-menu-item) { color: rgba(255, 255, 255, 0.7); margin: 4px 10px; border-radius: 8px;
    &:hover { background: rgba(255, 255, 255, 0.1); color: #fff; }
    &.is-active { background: linear-gradient(90deg, #667eea 0%, #764ba2 100%); color: #fff; }
  }
}
.sidebar-footer { padding: 20px; border-top: 1px solid rgba(255, 255, 255, 0.1);
  .el-button { color: rgba(255, 255, 255, 0.7); width: 100%; justify-content: flex-start; &:hover { color: #fff; } }
}
.main-content { flex: 1; min-width: 0; }
.content-card { border-radius: 16px; box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2); }
.loading { text-align: center; padding: 60px; color: #909399; font-size: 16px; }
.content-section { animation: fadeIn 0.3s ease; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.section-title { font-size: 22px; font-weight: 600; color: #303133; margin: 0 0 20px 0; }
.stat-card { border-radius: 12px; display: flex; align-items: center; padding: 20px; gap: 16px; }
.stat-icon { width: 60px; height: 60px; border-radius: 12px; display: flex; align-items: center; justify-content: center; font-size: 28px; color: #fff;
  &.mysql-icon { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); }
  &.etcd-icon { background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); }
  &.agent-icon { background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%); }
  &.status-icon { background: linear-gradient(135deg, #fa709a 0%, #fee140 100%); &.healthy { background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%); } }
}
.stat-info { .stat-value { font-size: 28px; font-weight: 700; color: #303133; } .stat-label { font-size: 14px; color: #909399; margin-top: 4px; } }
.info-card, .control-card { border-radius: 12px; :deep(.el-card__header) { font-weight: 600; font-size: 16px; } }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.node-detail { padding: 10px 0; }
.log-card { :deep(.el-card__body) { padding: 0; } }
.log-container { height: 500px; overflow-y: auto; background: #1e1e1e; border-radius: 0 0 12px 12px; padding: 15px; font-family: 'Monaco', 'Menlo', monospace; font-size: 13px; }
.log-line { padding: 4px 0; border-bottom: 1px solid #333; display: flex; gap: 10px; color: #d4d4d4; }
.log-time { color: #6a9955; min-width: 70px; }
.log-level { min-width: 50px; font-weight: bold; }
.log-node { color: #569cd6; }
.log-message { flex: 1; word-break: break-all; }
.log-error { color: #f14c4c; .log-level { color: #f14c4c; } }
.log-warn { color: #cca700; .log-level { color: #cca700; } }
.log-info { .log-level { color: #3794ff; } }
.log-empty { color: #666; text-align: center; padding: 50px; }
</style>
