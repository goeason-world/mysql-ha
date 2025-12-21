<template>
  <div>
    <h1 class="page-title">创建集群</h1>

    <!-- 步骤条 -->
    <el-card class="steps-card">
      <el-steps :active="currentStep" finish-status="success" align-center>
        <el-step title="配置主机" description="添加服务器" />
        <el-step title="选择版本" description="软件版本" />
        <el-step title="集群设置" description="参数配置" />
        <el-step title="安装部署" description="执行安装" />
      </el-steps>
    </el-card>

    <!-- Step 1: 主机配置 -->
    <el-card v-if="currentStep === 0" class="content-card">
      <template #header>
        <div class="card-header">
          <span>配置主机</span>
          <el-button type="primary" plain @click="addHost">➕ 添加主机</el-button>
        </div>
      </template>

      <div v-for="(host, index) in hosts" :key="index" class="host-card">
        <div class="host-header">
          <div class="host-info">
            <div class="host-num">{{ index + 1 }}</div>
            <strong>主机 {{ index + 1 }}</strong>
          </div>
          <div class="host-actions">
            <span v-if="host.testOk" class="conn-status ok">✓ 连接成功</span>
            <span v-if="host.testFail" class="conn-status fail">✗ {{ host.testMsg }}</span>
            <el-button v-if="hosts.length > 1" size="small" type="danger" plain @click="removeHost(index)">
              删除
            </el-button>
          </div>
        </div>

        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="主机名">
              <el-input v-model="host.name" placeholder="mysql-node-1" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="IP 地址">
              <el-input v-model="host.ip" placeholder="192.168.1.100" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="SSH 端口">
              <el-input-number v-model="host.port" :min="1" :max="65535" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="用户名 (需sudo)">
              <el-input v-model="host.username" placeholder="root" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="密码">
              <el-input v-model="host.password" type="password" show-password />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="连接测试">
              <el-button 
                type="primary" 
                plain 
                @click="testHostConnection(host)" 
                :loading="host.testing"
                :disabled="!host.ip || !host.username || !host.password"
              >
                🔗 测试连接
              </el-button>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="选择角色 (可多选)">
          <div class="role-selector">
            <div 
              class="role-tag" 
              :class="{ selected: host.roles?.includes('master'), master: host.roles?.includes('master') }"
              @click="toggleRole(host, 'master')"
            >
              👑 MySQL Master
            </div>
            <div 
              class="role-tag" 
              :class="{ selected: host.roles?.includes('slave'), slave: host.roles?.includes('slave') }"
              @click="toggleRole(host, 'slave')"
            >
              📦 MySQL Slave
            </div>
            <div 
              class="role-tag" 
              :class="{ selected: host.roles?.includes('etcd'), etcd: host.roles?.includes('etcd') }"
              @click="toggleRole(host, 'etcd')"
            >
              🔧 etcd 节点
            </div>
          </div>
          <div class="role-tip">💡 一台主机可同时是 MySQL 和 etcd 节点</div>
        </el-form-item>
      </div>

      <div class="btn-row">
        <el-button @click="router.push('/clusters')">取消</el-button>
        <el-button type="primary" @click="currentStep = 1" :disabled="!canProceedStep1">
          下一步
        </el-button>
      </div>
    </el-card>

    <!-- Step 2: 版本选择 -->
    <el-card v-if="currentStep === 1" class="content-card">
      <template #header>
        <span class="card-title">选择软件版本</span>
      </template>

      <!-- 下载源配置 -->
      <div class="download-source-section">
        <h4>📦 软件下载源</h4>
        <el-radio-group v-model="downloadSource.type" class="source-radio">
          <el-radio value="remote">官方远程下载</el-radio>
          <el-radio value="local">本地文件服务器</el-radio>
        </el-radio-group>
        <el-input 
          v-if="downloadSource.type === 'local'" 
          v-model="downloadSource.baseUrl" 
          placeholder="http://your-server:8080/packages"
          style="margin-top: 12px;"
        >
          <template #prepend>服务器地址</template>
        </el-input>
        <div class="source-tip">
          💡 本地服务器需要提供: /etcd-v{version}-linux-amd64.tar.gz 和 /mysql-{version}-linux-*.tar.gz
        </div>
      </div>

      <h4 style="margin-top: 24px;">etcd 版本</h4>
      <div class="version-grid">
        <div 
          v-for="v in versions.etcd" 
          :key="v.version" 
          class="version-card"
          :class="{ selected: selectedEtcd === v.version }"
          @click="selectedEtcd = v.version"
        >
          <h4>etcd {{ v.version }}</h4>
          <small>{{ getDownloadUrl(v, 'etcd') }}</small>
        </div>
      </div>

      <h4 style="margin-top: 24px;">MySQL 版本</h4>
      <div class="version-grid">
        <div 
          v-for="v in allMysqlVersions" 
          :key="v.version" 
          class="version-card"
          :class="{ selected: selectedMysql === v.version }"
          @click="selectedMysql = v.version"
        >
          <h4>MySQL {{ v.version }}</h4>
          <small>{{ getDownloadUrl(v, 'mysql') }}</small>
        </div>
      </div>

      <div class="btn-row">
        <el-button @click="currentStep = 0">上一步</el-button>
        <el-button type="primary" @click="currentStep = 2" :disabled="!selectedEtcd || !selectedMysql">
          下一步
        </el-button>
      </div>
    </el-card>

    <!-- Step 3: 集群设置 -->
    <el-card v-if="currentStep === 2" class="content-card">
      <template #header>
        <span class="card-title">集群设置</span>
      </template>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="集群名称">
            <el-input v-model="clusterConfig.name" placeholder="my-mysql-cluster" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="安装路径">
            <el-input v-model="clusterConfig.installPath" placeholder="/opt/mysql-ha" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="数据目录">
            <el-input v-model="clusterConfig.dataPath" placeholder="/var/lib/mysql" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="MySQL 端口">
            <el-input-number v-model="clusterConfig.mysqlPort" :min="1" :max="65535" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="Root 密码">
            <el-input v-model="clusterConfig.rootPassword" type="password" show-password />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="复制用户">
            <el-input v-model="clusterConfig.replUser" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="复制密码">
            <el-input v-model="clusterConfig.replPassword" type="password" show-password />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="HA Agent 端口">
            <el-input-number v-model="clusterConfig.agentPort" :min="1" :max="65535" />
          </el-form-item>
        </el-col>
      </el-row>

      <div class="btn-row">
        <el-button @click="currentStep = 1">上一步</el-button>
        <el-button type="primary" @click="handleCreateCluster" :loading="creating">
          创建并安装
        </el-button>
      </div>
    </el-card>

    <!-- Step 4: 安装进度 -->
    <el-card v-if="currentStep === 3" class="content-card">
      <template #header>
        <div class="card-header">
          <span>安装进度</span>
          <el-tag :type="overallStatus === 'completed' ? 'success' : overallStatus === 'failed' ? 'danger' : 'primary'">
            {{ overallStatusText }}
          </el-tag>
        </div>
      </template>

      <div v-for="host in installHosts" :key="host.id" class="install-host">
        <div class="install-header">
          <div>
            <strong>{{ host.name }}</strong>
            <span class="host-ip">({{ host.ip }})</span>
          </div>
          <el-tag :type="getStatusType(host.status)">{{ host.status }}</el-tag>
        </div>
        <div class="install-roles">
          <el-tag v-for="role in host.roles" :key="role" size="small" :type="getRoleType(role)">
            {{ role }}
          </el-tag>
        </div>
        <!-- 显示错误信息 -->
        <div v-if="host.status?.startsWith('failed')" class="error-msg">
          ❌ {{ host.status }}
        </div>
      </div>

      <div class="btn-row">
        <el-button @click="currentStep = 2">上一步 (重新配置)</el-button>
        <el-button @click="refreshInstallStatus" :loading="refreshing">刷新状态</el-button>
        <el-button v-if="hasFailedHosts" type="warning" @click="retryInstall" :loading="retrying">
          重试失败节点
        </el-button>
        <el-button type="primary" @click="router.push('/clusters')">完成</el-button>
      </div>
    </el-card>
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

// Step 1: 主机配置
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
  // master 和 slave 互斥
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
      name: host.name,
      ip: host.ip,
      port: host.port,
      username: host.username,
      password: host.password
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

// Step 2: 版本选择
const versions = ref<Versions>({ etcd: [], mysql_5_7: [], mysql_8_0: [] })
const selectedEtcd = ref('')
const selectedMysql = ref('')

// 下载源配置
const downloadSource = ref({
  type: 'remote', // 'remote' 或 'local'
  baseUrl: ''
})

const allMysqlVersions = computed<SoftwareVersion[]>(() => {
  return [...(versions.value.mysql_5_7 || []), ...(versions.value.mysql_8_0 || [])]
})

const getDownloadUrl = (v: SoftwareVersion, type: string) => {
  if (downloadSource.value.type === 'local' && downloadSource.value.baseUrl) {
    const base = downloadSource.value.baseUrl.replace(/\/$/, '')
    if (type === 'etcd') {
      return `${base}/etcd-v${v.version}-linux-amd64.tar.gz`
    } else {
      const ext = v.version.startsWith('8.') ? 'tar.xz' : 'tar.gz'
      return `${base}/mysql-${v.version}-linux-glibc2.17-x86_64.${ext}`
    }
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

// Step 3: 集群设置
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

// Step 4: 安装进度
const installHosts = ref<Host[]>([])
const retrying = ref(false)

// 计算整体状态
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

const hasFailedHosts = computed(() => {
  return installHosts.value.some(h => h.status?.startsWith('failed'))
})

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
        name: h.name!,
        ip: h.ip!,
        port: h.port!,
        username: h.username!,
        password: h.password!,
        roles: h.roles!
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

const getStatusType = (status?: string) => {
  if (status === 'completed') return 'success'
  if (status === 'pending') return 'warning'
  if (status?.startsWith('failed')) return 'danger'
  return 'primary'
}

const getRoleType = (role: string) => {
  if (role === 'master') return 'warning'
  if (role === 'slave') return 'success'
  return 'info'
}

onMounted(loadVersions)
</script>

<style scoped lang="scss">
.steps-card {
  border-radius: 16px;
  margin-bottom: 20px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
}

.content-card {
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 18px;
  font-weight: 600;
}

.card-title {
  font-size: 18px;
  font-weight: 600;
}

.host-card {
  border: 2px solid #e4e7ed;
  border-radius: 12px;
  padding: 20px;
  margin-bottom: 16px;
  transition: all 0.3s;

  &:hover {
    border-color: #409eff;
    box-shadow: 0 4px 12px rgba(64, 158, 255, 0.2);
  }
}

.host-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.host-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.host-num {
  width: 32px;
  height: 32px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
}

.host-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.conn-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 12px;

  &.ok {
    background: #f0f9eb;
    color: #67c23a;
  }

  &.fail {
    background: #fef0f0;
    color: #f56c6c;
  }
}

.role-selector {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.role-tag {
  padding: 10px 16px;
  border: 2px solid #dcdfe6;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s;
  display: flex;
  align-items: center;
  gap: 8px;

  &:hover {
    border-color: #409eff;
  }

  &.selected {
    border-color: #409eff;
    background: #ecf5ff;
    color: #409eff;

    &.master {
      border-color: #e6a23c;
      background: #fdf6ec;
      color: #e6a23c;
    }

    &.slave {
      border-color: #67c23a;
      background: #f0f9eb;
      color: #67c23a;
    }

    &.etcd {
      border-color: #909399;
      background: #f4f4f5;
      color: #606266;
    }
  }
}

.role-tip {
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
}

.version-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
  margin-top: 12px;
}

.version-card {
  border: 2px solid #e4e7ed;
  border-radius: 12px;
  padding: 20px;
  cursor: pointer;
  text-align: center;
  transition: all 0.3s;

  &:hover {
    border-color: #409eff;
    transform: translateY(-2px);
  }

  &.selected {
    border-color: #409eff;
    background: linear-gradient(135deg, #ecf5ff, #f5f9ff);
  }

  h4 {
    margin-bottom: 8px;
  }

  small {
    color: #909399;
    font-size: 11px;
    word-break: break-all;
  }
}

.btn-row {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  margin-top: 24px;
}

.install-host {
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 12px;
}

.install-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.host-ip {
  color: #909399;
  margin-left: 8px;
}

.install-roles {
  display: flex;
  gap: 8px;
}

.download-source-section {
  background: #f5f7fa;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 20px;

  h4 {
    margin-bottom: 12px;
  }
}

.source-radio {
  display: flex;
  gap: 20px;
}

.source-tip {
  margin-top: 12px;
  font-size: 12px;
  color: #909399;
}

.error-msg {
  margin-top: 8px;
  padding: 8px 12px;
  background: #fef0f0;
  border-radius: 4px;
  color: #f56c6c;
  font-size: 13px;
}
</style>
