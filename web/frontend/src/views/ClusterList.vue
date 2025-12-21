<template>
  <div>
    <h1 class="page-title">集群管理</h1>
    
    <el-card class="cluster-card">
      <template #header>
        <div class="card-header">
          <span>我的集群</span>
          <el-button type="primary" @click="router.push('/clusters/create')">
            ➕ 创建集群
          </el-button>
        </div>
      </template>

      <div v-if="loading" class="loading">
        <el-icon class="is-loading"><Loading /></el-icon>
        加载中...
      </div>

      <div v-else-if="clusters.length === 0" class="empty">
        <el-empty description="暂无集群">
          <el-button type="primary" @click="router.push('/clusters/create')">
            创建第一个集群
          </el-button>
        </el-empty>
      </div>

      <div v-else class="cluster-grid">
        <div v-for="cluster in clusters" :key="cluster.id" class="cluster-item">
          <h3>🗄️ {{ cluster.name }}</h3>
          <div class="info-row">
            <span>MySQL</span>
            <span>{{ cluster.mysql_version }}</span>
          </div>
          <div class="info-row">
            <span>etcd</span>
            <span>{{ cluster.etcd_version }}</span>
          </div>
          <div class="info-row">
            <span>节点</span>
            <span>{{ cluster.hosts?.length || 0 }} 台</span>
          </div>
          <div class="actions">
            <el-button size="small" @click="router.push(`/clusters/${cluster.id}`)">
              详情
            </el-button>
            <el-button size="small" type="danger" @click="handleDelete(cluster.id)">
              删除
            </el-button>
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>


<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { getClusters, deleteCluster, type Cluster } from '@/api'

const router = useRouter()
const loading = ref(true)
const clusters = ref<Cluster[]>([])

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

const handleDelete = async (id: string) => {
  try {
    await ElMessageBox.confirm('确定要删除此集群吗？', '确认删除', {
      type: 'warning'
    })
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
.cluster-card {
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

.loading {
  text-align: center;
  padding: 40px;
  color: #909399;
}

.empty {
  padding: 40px;
}

.cluster-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 20px;
}

.cluster-item {
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  padding: 20px;
  transition: all 0.3s;

  &:hover {
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    transform: translateY(-2px);
  }

  h3 {
    margin-bottom: 16px;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .info-row {
    display: flex;
    justify-content: space-between;
    padding: 8px 0;
    border-bottom: 1px solid #f0f0f0;
    font-size: 14px;
  }

  .actions {
    margin-top: 16px;
    display: flex;
    gap: 8px;
  }
}
</style>
