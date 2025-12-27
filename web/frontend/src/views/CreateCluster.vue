<template>
  <div class="create-cluster-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">创建集群</h1>
        <p class="page-subtitle">配置并部署新的 MySQL 高可用集群</p>
      </div>
    </div>

    <!-- 步骤指示器 -->
    <div class="steps-container">
      <div class="step" :class="{ active: currentStep >= 0, completed: currentStep > 0 }">
        <div class="step-number">1</div>
        <div class="step-info">
          <div class="step-title">配置主机</div>
          <div class="step-desc">添加服务器</div>
        </div>
      </div>
      <div class="step-line" :class="{ active: currentStep > 0 }"></div>
      <div class="step" :class="{ active: currentStep >= 1, completed: currentStep > 1 }">
        <div class="step-number">2</div>
        <div class="step-info">
          <div class="step-title">选择版本</div>
          <div class="step-desc">软件版本</div>
        </div>
      </div>
      <div class="step-line" :class="{ active: currentStep > 1 }"></div>
      <div class="step" :class="{ active: currentStep >= 2, completed: currentStep > 2 }">
        <div class="step-number">3</div>
        <div class="step-info">
          <div class="step-title">集群设置</div>
          <div class="step-desc">参数配置</div>
        </div>
      </div>
      <div class="step-line" :class="{ active: currentStep > 2 }"></div>
      <div class="step" :class="{ active: currentStep >= 3 }">
        <div class="step-number">4</div>
        <div class="step-info">
          <div class="step-title">安装部署</div>
          <div class="step-desc">执行安装</div>
        </div>
      </div>
    </div>

    <!-- Step 1: 主机配置 -->
    <div v-if="currentStep === 0" class="card">
      <div class="card-header">
        <span class="card-title">🖥️ 配置主机</span>
        <button class="btn btn-primary btn-sm" @click="addHost">➕ 添加主机</button>
      </div>
      <div class="card-body">
        <div v-for="(host, index) in hosts" :key="index" class="host-card">
          <div class="host-header">
            <div class="host-info">
              <div class="host-num">{{ index + 1 }}</div>
              <span class="host-title">主机 {{ index + 1 }}</span>
            </div>
            <div class="host-actions">
              <span v-if="host.testOk" class="status-badge success">✓ 连接成功</span>
              <span v-if="host.testFail" class="status-badge danger">✗ {{ host.testMsg }}</span>
              <button v-if="hosts.length > 1" class="btn btn-danger btn-sm" @click="removeHost(index)">删除</button>
            </div>
          </div>

          <div class="form-grid">
            <div class="form-group">
              <label class="form-label">主机名</label>
              <input v-model="host.name" class="form-input" placeholder="mysql-node-1" />
            </div>
            <div class="form-group">
              <label class="form-label">IP 地址</label>
              <input v-model="host.ip" class="form-input" placeholder="192.168.1.100" />
            </div>
            <div class="form-group">
              <label class="form-label">SSH 端口</label>
              <input v-model.number="host.port" type="number" class="form-input" />
            </div>
            <div class="form-group">
              <label class="form-label">用户名 (需sudo)</label>
              <input v-model="host.username" class="form-input" placeholder="root" />
            </div>
            <div class="form-group">
              <label class="form-label">密码</label>
              <input v-model="host.password" type="password" class="form-input" />
            </div>
            <div class="form-group">
              <label class="form-label">连接测试</label>
              <button 
                class="btn btn-secondary" 
                @click="testHostConnection(host)" 
                :disabled="host.testing || !host.ip || !host.username || !host.password"
              >
                {{ host.testing ? '测试中...' : '🔗 测试连接' }}
              </button>
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">选择角色 (可多选)</label>
            <div class="role-selector">
              <div 
                class="role-tag" 
                :class="{ selected: host.roles?.includes('master') }"
                @click="toggleRole(host, 'master')"
              >
                <span class="role-icon">👑</span>
                <span>MySQL Master</span>
              </div>
              <div 
                class="role-tag" 
                :class="{ selected: host.roles?.includes('slave') }"
                @click="toggleRole(host, 'slave')"
              >
                <span class="role-icon">📦</span>
                <span>MySQL Slave</span>
              </div>
              <div 
                class="role-tag" 
                :class="{ selected: host.roles?.includes('etcd') }"
                @click="toggleRole(host, 'etcd')"
              >
                <span class="role-icon">🔧</span>
                <span>etcd 节点</span>
              </div>
            </div>
            <p class="form-hint">💡 一台主机可同时是 MySQL 和 etcd 节点</p>
          </div>
        </div>

        <div class="btn-row">
          <button class="btn btn-secondary" @click="router.push('/clusters')">取消</button>
          <button class="btn btn-primary" @click="currentStep = 1" :disabled="!canProceedStep1">下一步</button>
        </div>
      </div>
    </div>

    <!-- Step 2: 版本选择 -->
    <div v-if="currentStep === 1" class="card">
      <div class="card-header">
        <span class="card-title">📦 选择软件版本</span>
      </div>
      <div class="card-body">
        <div class="source-section">
          <h4>下载源配置</h4>
          <div class="source-options">
            <label class="radio-option" :class="{ selected: downloadSource.type === 'remote' }">
              <input type="radio" v-model="downloadSource.type" value="remote" />
              <span class="radio-label">官方远程下载</span>
            </label>
            <label class="radio-option" :class="{ selected: downloadSource.type === 'local' }">
              <input type="radio" v-model="downloadSource.type" value="local" />
              <span class="radio-label">本地文件服务器</span>
            </label>
          </div>
          <div v-if="downloadSource.type === 'local'" class="form-group" style="margin-top: 16px;">
            <input v-model="downloadSource.baseUrl" class="form-input" placeholder="http://your-server:8080/packages" />
          </div>
        </div>

        <h4 style="margin-top: 24px; color: var(--text-primary);">etcd 版本</h4>
        <div class="version-grid">
          <div 
            v-for="v in versions.etcd" 
            :key="v.version" 
            class="version-card"
            :class="{ selected: selectedEtcd === v.version }"
            @click="selectedEtcd = v.version"
          >
            <div class="version-name">etcd {{ v.version }}</div>
            <div class="version-url">{{ getDownloadUrl(v, 'etcd') }}</div>
          </div>
        </div>

        <h4 style="margin-top: 24px; color: var(--text-primary);">MySQL 版本</h4>
        <div class="version-grid">
          <div 
            v-for="v in allMysqlVersions" 
            :key="v.version" 
            class="version-card"
            :class="{ selected: selectedMysql === v.version }"
            @click="selectedMysql = v.version"
          >
            <div class="version-name">MySQL {{ v.version }}</div>
            <div class="version-url">{{ getDownloadUrl(v, 'mysql') }}</div>
          </div>
        </div>

        <div class="btn-row">
          <button class="btn btn-secondary" @click="currentStep = 0">上一步</button>
          <button class="btn btn-primary" @click="currentStep = 2" :disabled="!selectedEtcd || !selectedMysql">下一步</button>
        </div>
      </div>
    </div>

    <!-- Step 3: 集群设置 -->
    <div v-if="currentStep === 2" class="card">
      <div class="card-header">
        <span class="card-title">⚙️ 集群设置</span>
      </div>
      <div class="card-body">
        <div class="form-grid">
          <div class="form-group">
            <label class="form-label">集群名称</label>
            <input v-model="clusterConfig.name" class="form-input" placeholder="my-mysql-cluster" />
          </div>
          <div class="form-group">
            <label class="form-label">安装路径</label>
            <input v-model="clusterConfig.installPath" class="form-input" placeholder="/opt/mysql-ha" />
          </div>
          <div class="form-group">
            <label class="form-label">数据目录</label>
            <input v-model="clusterConfig.dataPath" class="form-input" placeholder="/var/lib/mysql" />
          </div>
          <div class="form-group">
            <label class="form-label">MySQL 端口</label>
            <input v-model.number="clusterConfig.mysqlPort" type="number" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">Root 密码</label>
            <input v-model="clusterConfig.rootPassword" type="password" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">复制用户</label>
            <input v-model="clusterConfig.replUser" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">复制密码</label>
            <input v-model="clusterConfig.replPassword" type="password" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">HA Agent 端口</label>
            <input v-model.number="clusterConfig.agentPort" type="number" class="form-input" />
          </div>
        </div>

        <div class="btn-row">
          <button class="btn btn-secondary" @click="currentStep = 1">上一步</button>
          <button class="btn btn-primary" @click="handleCreateCluster" :disabled="creating">
            {{ creating ? '创建中...' : '创建并安装' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Step 4: 安装进度 -->
    <div v-if="currentStep === 3" class="card">
      <div class="card-header">
        <span class="card-title">🚀 安装进度</span>
        <span class="tag" :class="getOverallStatusClass">{{ overallStatusText }}</span>
      </div>
      <div class="card-body">
        <div v-for="host in installHosts" :key="host.id" class="install-host">
          <div class="install-header">
            <div class="install-info">
              <span class="install-name">{{ host.name }}</span>
              <span class="install-ip">{{ host.ip }}</span>
            </div>
            <span class="tag" :class="getStatusClass(host.status)">{{ host.status || 'pending' }}</span>
          </div>
          <div class="install-roles">
            <span v-for="role in host.roles" :key="role" class="tag tag-info">{{ role }}</span>
          </div>
          <div v-if="host.status?.startsWith('failed')" class="error-message">
            ❌ {{ host.status }}
          </div>
        </div>

        <div class="btn-row">
          <button class="btn btn-secondary" @click="currentStep = 2">重新配置</button>
          <button class="btn btn-secondary" @click="refreshInstallStatus" :disabled="refreshing">
            {{ refreshing ? '刷新中...' : '刷新状态' }}
          </button>
          <button v-if="hasFailedHosts" class="btn btn-danger" @click="retryInstall" :disabled="retrying">
            {{ retrying ? '重试中...' : '重试失败节点' }}
          </button>
          <button class="btn btn-primary" @click="router.push('/clusters')">完成</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { 
  getVersions, testHost, createCluster, installCluster, getInstallStatus,
  type Host, type Versions, type SoftwareVersion 
} from '@/api'

interface HostForm extends Partial<Host> {
  testing?: boolean
  testOk?: boolean
  testFail?: boolean
  testMsg?: string
}

const router = useRouter()
const currentStep = ref(0)
const creating = ref(false)
const refreshing = ref(false)
const clusterId = ref('')

// Step 1
const hosts = ref<HostForm[]>([
  { name: '', ip: '', port: 22, username: 'root', password: '', roles: [] }
])

const addHost = () => {
  hosts.value.push({ name: '', ip: '', port: 22, username: 'root', password: '', roles: [] })
}

const removeHost = (index: number) => {
  hosts.value.splice(index, 1)
}

const toggleRole = (host: HostForm, role: string) => {
  if (!host.roles) host.roles = []
  if (role === 'master' && host.roles.includes('slave')) {
    host.roles = host.roles.filter(r => r !== 'slave')
  }
  if (role === 'slave' && host.roles.includes('master')) {
    host.roles = host.roles.filter(r => r !== 'master')
  }
  const idx = host.roles.indexOf(role)
  if (idx >= 0) {
    host.roles.splice(idx, 1)
  } else {
    host.roles.push(role)
  }
}

const testHostConnection = async (host: HostForm) => {
  host.testing = true
  host.testOk = false
  host.testFail = false
  try {
    const { data } = await testHost({
      name: host.name, ip: host.ip, port: host.port,
      username: host.username, password: host.password
    })
    if (data.success) {
      host.testOk = true
    } else {
      host.testFail = true
      host.testMsg = data.message || '连接失败'
    }
  } catch {
    host.testFail = true
    host.testMsg = '请求失败'
  }
  host.testing = false
}

const canProceedStep1 = computed(() => {
  return hosts.value.every(h => h.name && h.ip && h.username && h.password && h.roles && h.roles.length > 0)
})

// Step 2
const versions = ref<Versions>({ etcd: [], mysql_5_7: [], mysql_8_0: [] })
const selectedEtcd = ref('')
const selectedMysql = ref('')
const downloadSource = ref({ type: 'remote', baseUrl: '' })

const allMysqlVersions = computed<SoftwareVersion[]>(() => {
  return [...(versions.value.mysql_5_7 || []), ...(versions.value.mysql_8_0 || [])]
})

const getDownloadUrl = (v: SoftwareVersion, type: string) => {
  if (downloadSource.value.type === 'local' && downloadSource.value.baseUrl) {
    const base = downloadSource.value.baseUrl.replace(/\/$/, '')
    if (type === 'etcd') return `${base}/etcd-v${v.version}-linux-amd64.tar.gz`
    const ext = v.version.startsWith('8.') ? 'tar.xz' : 'tar.gz'
    return `${base}/mysql-${v.version}-linux-glibc2.17-x86_64.${ext}`
  }
  return v.download_url
}

const loadVersions = async () => {
  try {
    const { data } = await getVersions()
    versions.value = data
    if (data.etcd?.length) selectedEtcd.value = data.etcd[0].version
    if (data.mysql_8_0?.length) selectedMysql.value = data.mysql_8_0[0].version
  } catch {
    ElMessage.error('加载版本信息失败')
  }
}

// Step 3
const clusterConfig = ref({
  name: 'mysql-cluster',
  installPath: '/opt/mysql-ha',
  dataPath: '/var/lib/mysql',
  mysqlPort: 3306,
  rootPassword: 'Root@123456',
  replUser: 'replicator',
  replPassword: 'Repl@123456',
  agentPort: 8080
})

// Step 4
const installHosts = ref<Host[]>([])
const retrying = ref(false)

const overallStatus = computed(() => {
  if (installHosts.value.length === 0) return 'pending'
  const allCompleted = installHosts.value.every(h => h.status === 'completed')
  const hasFailed = installHosts.value.some(h => h.status?.startsWith('failed'))
  if (allCompleted) return 'completed'
  if (hasFailed) return 'failed'
  return 'running'
})

const overallStatusText = computed(() => {
  switch (overallStatus.value) {
    case 'completed': return '安装完成'
    case 'failed': return '部分失败'
    case 'running': return '安装中...'
    default: return '等待中'
  }
})

const getOverallStatusClass = computed(() => {
  switch (overallStatus.value) {
    case 'completed': return 'tag-success'
    case 'failed': return 'tag-danger'
    default: return 'tag-info'
  }
})

const hasFailedHosts = computed(() => {
  return installHosts.value.some(h => h.status?.startsWith('failed'))
})

const getStatusClass = (status?: string) => {
  if (status === 'completed') return 'tag-success'
  if (status === 'pending') return 'tag-warning'
  if (status?.startsWith('failed')) return 'tag-danger'
  return 'tag-info'
}

const retryInstall = async () => {
  if (!clusterId.value) return
  retrying.value = true
  try {
    await installCluster(clusterId.value)
    ElMessage.success('重试安装已启动')
    setTimeout(refreshInstallStatus, 2000)
  } catch {
    ElMessage.error('重试失败')
  }
  retrying.value = false
}

const handleCreateCluster = async () => {
  creating.value = true
  try {
    const config = {
      name: clusterConfig.value.name,
      hosts: hosts.value.map(h => ({
        name: h.name!, ip: h.ip!, port: h.port!,
        username: h.username!, password: h.password!, roles: h.roles!
      })),
      etcd_version: selectedEtcd.value,
      mysql_version: selectedMysql.value,
      install_path: clusterConfig.value.installPath,
      data_path: clusterConfig.value.dataPath,
      settings: {
        mysql_port: clusterConfig.value.mysqlPort,
        root_password: clusterConfig.value.rootPassword,
        replication_user: clusterConfig.value.replUser,
        replication_pass: clusterConfig.value.replPassword,
        ha_agent_port: clusterConfig.value.agentPort
      }
    }
    const { data } = await createCluster(config)
    clusterId.value = data.id
    installHosts.value = data.hosts
    await installCluster(data.id)
    currentStep.value = 3
    ElMessage.success('集群创建成功，开始安装')
  } catch (e: any) {
    ElMessage.error('创建失败: ' + (e.response?.data?.error || e.message))
  }
  creating.value = false
}

const refreshInstallStatus = async () => {
  if (!clusterId.value) return
  refreshing.value = true
  try {
    const { data } = await getInstallStatus(clusterId.value)
    installHosts.value = data.hosts
  } catch {
    ElMessage.error('刷新状态失败')
  }
  refreshing.value = false
}

onMounted(loadVersions)
</script>

<style scoped lang="scss">
.create-cluster-page {
  max-width: 1000px;
  margin: 0 auto;
}

.steps-container {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 32px;
  padding: 24px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
}

.step {
  display: flex;
  align-items: center;
  gap: 12px;
  opacity: 0.5;
  
  &.active {
    opacity: 1;
  }
  
  &.completed .step-number {
    background: var(--success);
  }
}

.step-number {
  width: 36px;
  height: 36px;
  background: var(--bg-input);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  color: var(--text-primary);
}

.step.active .step-number {
  background: var(--primary);
  color: white;
}

.step-info {
  .step-title {
    font-weight: 600;
    color: var(--text-primary);
  }
  .step-desc {
    font-size: 12px;
    color: var(--text-muted);
  }
}

.step-line {
  width: 60px;
  height: 2px;
  background: var(--border-color);
  margin: 0 16px;
  
  &.active {
    background: var(--primary);
  }
}

.host-card {
  background: var(--bg-app);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 20px;
  margin-bottom: 16px;
}

.host-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.host-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.host-num {
  width: 32px;
  height: 32px;
  background: var(--primary);
  color: white;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
}

.host-title {
  font-weight: 600;
  color: var(--text-primary);
}

.host-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.status-badge {
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  
  &.success {
    background: var(--success-bg);
    color: var(--success);
  }
  
  &.danger {
    background: var(--danger-bg);
    color: var(--danger);
  }
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.form-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 8px;
}

.role-selector {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.role-tag {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 20px;
  background: var(--bg-input);
  border: 2px solid var(--border-color);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: var(--transition);
  
  &:hover {
    border-color: var(--primary);
  }
  
  &.selected {
    border-color: var(--primary);
    background: rgba(99, 102, 241, 0.1);
    color: var(--primary-light);
  }
  
  .role-icon {
    font-size: 18px;
  }
}

.source-section {
  background: var(--bg-app);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 20px;
  margin-bottom: 24px;
  
  h4 {
    color: var(--text-primary);
    margin-bottom: 16px;
  }
}

.source-options {
  display: flex;
  gap: 16px;
}

.radio-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: var(--transition);
  
  &.selected {
    border-color: var(--primary);
    background: rgba(99, 102, 241, 0.1);
  }
  
  input {
    display: none;
  }
  
  .radio-label {
    color: var(--text-primary);
  }
}

.version-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
  margin-top: 12px;
}

.version-card {
  background: var(--bg-card);
  border: 2px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 16px;
  cursor: pointer;
  transition: var(--transition);
  
  &:hover {
    border-color: var(--primary);
  }
  
  &.selected {
    border-color: var(--primary);
    background: rgba(99, 102, 241, 0.1);
  }
  
  .version-name {
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 8px;
  }
  
  .version-url {
    font-size: 11px;
    color: var(--text-muted);
    word-break: break-all;
  }
}

.btn-row {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid var(--border-color);
}

.install-host {
  background: var(--bg-app);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 16px;
  margin-bottom: 12px;
}

.install-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.install-info {
  display: flex;
  align-items: center;
  gap: 12px;
  
  .install-name {
    font-weight: 600;
    color: var(--text-primary);
  }
  
  .install-ip {
    color: var(--text-muted);
  }
}

.install-roles {
  display: flex;
  gap: 8px;
}

.error-message {
  margin-top: 12px;
  padding: 12px;
  background: var(--danger-bg);
  border-radius: var(--radius-sm);
  color: var(--danger);
  font-size: 13px;
}
</style>
