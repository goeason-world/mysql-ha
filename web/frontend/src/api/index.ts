import axios, { AxiosError } from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000
})

// API 错误状态（用于全局错误处理）
export const apiState = {
  isBackendAvailable: true,
  lastError: null as string | null
}

// 响应拦截器 - 处理后端不可用的情况
api.interceptors.response.use(
  (response) => {
    // 请求成功，标记后端可用
    apiState.isBackendAvailable = true
    apiState.lastError = null
    return response
  },
  (error: AxiosError) => {
    // 网络错误或后端不可用
    if (!error.response) {
      apiState.isBackendAvailable = false
      apiState.lastError = '无法连接到后端服务，请检查服务是否正常运行'
      console.error('[API] Backend unavailable:', error.message)
    } else if (error.response.status >= 500) {
      apiState.isBackendAvailable = false
      apiState.lastError = `后端服务错误: ${error.response.status}`
      console.error('[API] Backend error:', error.response.status)
    }
    return Promise.reject(error)
  }
)

// 健康检查
export const checkHealth = async (): Promise<boolean> => {
  try {
    await axios.get('/health', { timeout: 5000 })
    apiState.isBackendAvailable = true
    apiState.lastError = null
    return true
  } catch {
    apiState.isBackendAvailable = false
    apiState.lastError = '后端服务不可用'
    return false
  }
}

// 版本信息
export interface VersionInfo {
  backend_version: string
  agent_version: string
}

export const getVersionInfo = async (): Promise<VersionInfo | null> => {
  try {
    const response = await axios.get<VersionInfo>('/version', { timeout: 5000 })
    return response.data
  } catch {
    return null
  }
}

// 类型定义
export interface Host {
  id?: string
  name: string
  ip: string
  port: number
  username: string
  password: string
  roles: string[]
  status?: string
}

export interface ClusterSettings {
  mysql_port: number
  root_password: string
  replication_user: string
  replication_pass: string
  ha_agent_port: number
}

export interface DownloadSource {
  type: 'remote' | 'local'
  base_url: string
}

export interface Cluster {
  id: string
  name: string
  hosts: Host[]
  etcd_version: string
  mysql_version: string
  install_path: string
  data_path: string
  settings: ClusterSettings
  phase?: string
}

export interface SoftwareVersion {
  name: string
  version: string
  download_url: string
}

export interface Versions {
  etcd: SoftwareVersion[]
  mysql_5_7: SoftwareVersion[]
  mysql_8_0: SoftwareVersion[]
}

// API 方法
export const getVersions = () => api.get<Versions>('/versions')

export const testHost = (host: Partial<Host>) => 
  api.post<{ success: boolean; message: string }>('/hosts/test', host)

export const getClusters = () => 
  api.get<{ clusters: Cluster[] }>('/clusters')

export const getCluster = (id: string) => 
  api.get<Cluster>(`/clusters/${id}`)

export const createCluster = (config: Partial<Cluster>) => 
  api.post<Cluster>('/clusters', config)

export const deleteCluster = (id: string) => 
  api.delete(`/clusters/${id}`)

export const installCluster = (id: string) => 
  api.post(`/clusters/${id}/install`)

export const getInstallStatus = (id: string) =>
  api.get<{ cluster_id: string; hosts: Host[] }>(`/clusters/${id}/install/status`)

// 重试安装
export const retryInstall = (id: string, nodeIds?: string[], phase?: string) =>
  api.post(`/clusters/${id}/install/retry`, { node_ids: nodeIds, phase })

// 集群状态和操作
export interface NodeStatus {
  node_id: string
  name: string
  ip: string
  role: string
  is_healthy: boolean
  agent_version?: string
  replication_lag?: number
  install_status: string
}

export interface ClusterStatus {
  cluster_id: string
  cluster_name: string
  leader: NodeStatus | null
  nodes: NodeStatus[]
}

export const getClusterStatus = (id: string) =>
  api.get<ClusterStatus>(`/clusters/${id}/status`)

// etcd 状态
export interface EtcdNodeStatus {
  ip: string
  name: string
  is_healthy: boolean
  is_leader: boolean
}

export const getEtcdStatus = (id: string) =>
  api.get<{ nodes: EtcdNodeStatus[] }>(`/clusters/${id}/etcd-status`)

export const switchover = (id: string, targetNodeId: string, reason?: string) =>
  api.post(`/clusters/${id}/switchover`, { target_node_id: targetNodeId, reason })

// Agent 管理
export interface UpgradeResult {
  status: string
  message: string
  current_version?: string
}

export const upgradeAgents = (id: string, nodeIds: string[], force: boolean = false) =>
  api.post<UpgradeResult>(`/clusters/${id}/upgrade-agents`, { node_ids: nodeIds, force })

export const restartAgents = (id: string, nodeIds: string[]) =>
  api.post(`/clusters/${id}/restart-agents`, { node_ids: nodeIds })

// MySQL 服务管理
export const restartMySQL = (id: string, nodeIds: string[]) =>
  api.post(`/clusters/${id}/restart-mysql`, { node_ids: nodeIds })

// 日志
export interface LogEntry {
  time: string
  level: string
  node?: string
  message: string
}

export const getAgentLogs = (id: string, nodeId?: string) =>
  api.get<{ logs: LogEntry[] }>(`/clusters/${id}/logs`, { params: { node_id: nodeId } })

// 一键修复主从
export interface RepairResult {
  status: 'healthy' | 'started'
  message: string
  master?: string
  master_ip?: string
}

export const repairReplication = (id: string) =>
  api.post<RepairResult>(`/clusters/${id}/repair-replication`)

export default api
