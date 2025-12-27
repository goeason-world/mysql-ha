<template>
  <div class="cluster-list-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">集群管理</h1>
        <p class="page-subtitle">管理和监控您的 MySQL 高可用集群</p>
      </div>
      <button class="btn btn-primary btn-lg" @click="router.push('/clusters/create')">
        <span>➕</span>
        创建集群
      </button>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon primary">📊</div>
        <div class="stat-content">
          <div class="stat-value">{{ clusters.length }}</div>
          <div class="stat-label">集群总数</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon success">✅</div>
        <div class="stat-content">
          <div class="stat-value">{{ healthyClusters }}</div>
          <div class="stat-label">健康集群</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon info">🖥️</div>
        <div class="stat-content">
          <div class="stat-value">{{ totalNodes }}</div>
          <div class="stat-label">节点总数</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon warning">⚡</div>
        <div class="stat-content">
          <div class="stat-value">{{ totalMySQLNodes }}</div>
          <div class="stat-label">MySQL 节点</div>
        </div>
      </div>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading">
      <div class="spinner"></div>
      <span>加载中...</span>
    </div>

    <!-- 空状态 -->
    <div v-else-if="clusters.length === 0" class="card">
      <div class="empty-state">
        <div class="empty-icon">🗄️</div>
        <h3 class="empty-title">暂无集群</h3>
        <p class="empty-description">创建您的第一个 MySQL 高可用集群</p>
        <button class="btn btn-primary btn-lg" @click="router.push('/clusters/create')">
          创建集群
        </button>
      </div>
    </div>

    <!-- 集群列表 -->
    <div v-else class="cluster-grid">
      <div 
        v-for="cluster in clusters" 
        :key="cluster.id" 
        class="cluster-card"
        @click="router.push(`/clusters/${cluster.id}`)"
      >
        <div class="cluster-header">
          <div class="cluster-icon">🗄️</div>
          <div class="cluster-info">
            <div class="cluster-name">{{ cluster.name }}</div>
            <div class="cluster-id">{{ cluster.id.slice(0, 8) }}...</div>
          </div>
          <span class="tag" :class="getPhaseClass(cluster.phase)">
            {{ getPhaseText(cluster.phase) }}
          </span>
        </div>
        
        <div class="cluster-stats">
          <div class="stat-item">
            <div class="stat-value">{{ cluster.hosts?.length || 0 }}</div>
            <div class="stat-label">节点</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ getMySQLNodeCount(cluster) }}</div>
            <div class="stat-label">MySQL</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ getEtcdNodeCount(cluster) }}</div>
            <div class="stat-label">etcd</div>
          </div>
        </div>
        
        <div class="cluster-meta">
          <span class="tag tag-info">MySQL {{ cluster.mysql_version }}</span>
          <span class="tag tag-primary">etcd {{ cluster.etcd_version }}</span>
        </div>
        
        <div class="cluster-actions" @click.stop>
          <button class="btn btn-secondary btn-sm" @click="router.push(`/clusters/${cluster.id}`)">
            查看详情
          </button>
          <button class="btn btn-danger btn-sm" @click="handleDelete(cluster.id, cluster.name)">
            删除
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getClusters, deleteCluster, type Cluster } from '@/api'

const router = useRouter()
const loading = ref(true)
const clusters = ref<Cluster[]>([])

const healthyClusters = computed(() => {
  return clusters.value.filter(c => c.phase === 'completed').length
})

const totalNodes = computed(() => {
  return clusters.value.reduce((sum, c) => sum + (c.hosts?.length || 0), 0)
})

const totalMySQLNodes = computed(() => {
  return clusters.value.reduce((sum, c) => {
    return sum + (c.hosts?.filter(h => h.roles.includes('master') || h.roles.includes('slave')).length || 0)
  }, 0)
})

const getMySQLNodeCount = (cluster: Cluster) => {
  return cluster.hosts?.filter(h => h.roles.includes('master') || h.roles.includes('slave')).length || 0
}

const getEtcdNodeCount = (cluster: Cluster) => {
  return cluster.hosts?.filter(h => h.roles.includes('etcd')).length || 0
}

const getPhaseClass = (phase?: string) => {
  if (!phase || phase === 'pending') return 'tag-warning'
  if (phase === 'completed') return 'tag-success'
  if (phase.startsWith('failed')) return 'tag-danger'
  return 'tag-info'
}

const getPhaseText = (phase?: string) => {
  if (!phase || phase === 'pending') return '待安装'
  if (phase === 'completed') return '运行中'
  if (phase.startsWith('failed')) return '安装失败'
  if (phase.includes('phase1')) return '安装 etcd'
  if (phase.includes('phase2')) return '安装 MySQL'
  if (phase.includes('phase3')) return '配置复制'
  if (phase.includes('phase4')) return '安装 Agent'
  return phase
}

const loadClusters = async () => {
  loading.value = true
  try {
    const { data } = await getClusters()
    clusters.value = data.clusters || []
  } catch (e) {
    ElMessage.error('加载集群列表失败')
  } finally {
    loading.value = false
  }
}

const handleDelete = async (id: string, name: string) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除集群 "${name}" 吗？此操作不可恢复。`,
      '确认删除',
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
    )
    await deleteCluster(id)
    ElMessage.success('删除成功')
    loadClusters()
  } catch (e) {
    // 用户取消
  }
}

onMounted(loadClusters)
</script>

<style scoped lang="scss">
.cluster-list-page {
  max-width: 1400px;
  margin: 0 auto;
}
</style>
