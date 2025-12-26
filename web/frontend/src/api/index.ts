import axios from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000
})

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

export const switchover = (id: string, targetNodeId: string, reason?: string) =>
  api.post(`/clusters/${id}/switchover`, { target_node_id: targetNodeId, reason })

// Agent 管理
export const upgradeAgents = (id: string, nodeIds: string[]) =>
  api.post(`/clusters/${id}/upgrade-agents`, { node_ids: nodeIds })

export const restartAgents = (id: string, nodeIds: string[]) =>
  api.post(`/clusters/${id}/restart-agents`, { node_ids: nodeIds })

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
export const repairReplication = (id: string) =>
  api.post(`/clusters/${id}/repair-replication`)

export default api
