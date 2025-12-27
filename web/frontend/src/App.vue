<template>
  <div v-if="!backendAvailable" class="error-wrapper">
    <BackendError :message="errorMessage" @connected="onBackendConnected" />
  </div>
  <div v-else class="app-wrapper">
    <aside class="sidebar">
      <div class="logo">🐬 MySQL HA</div>
      <div 
        class="nav-item" 
        :class="{ active: route.path === '/clusters' }"
        @click="router.push('/clusters')"
      >
        📋 集群列表
      </div>
      <div 
        class="nav-item" 
        :class="{ active: route.path === '/clusters/create' }"
        @click="router.push('/clusters/create')"
      >
        ➕ 创建集群
      </div>
    </aside>
    <main class="main-content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BackendError from '@/components/BackendError.vue'
import { checkHealth, apiState } from '@/api'

const route = useRoute()
const router = useRouter()

const backendAvailable = ref(true)
const errorMessage = ref<string | null>(null)

// 启动时检查后端
onMounted(async () => {
  const isAvailable = await checkHealth()
  backendAvailable.value = isAvailable
  if (!isAvailable) {
    errorMessage.value = apiState.lastError
  }
})

// 监听 API 状态变化
watch(() => apiState.isBackendAvailable, (available) => {
  backendAvailable.value = available
  errorMessage.value = apiState.lastError
})

const onBackendConnected = () => {
  backendAvailable.value = true
  errorMessage.value = null
}
</script>
