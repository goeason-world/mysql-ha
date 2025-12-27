<template>
  <div v-if="!backendAvailable" class="error-wrapper">
    <BackendError :message="errorMessage || undefined" @connected="onBackendConnected" />
  </div>
  
  <div v-else class="app-layout">
    <Sidebar />
    
    <div class="main-content-wrapper">
      <TopHeader 
        :backend-available="backendAvailable" 
        :is-checking="isChecking"
        @check-health="checkBackendHealth"
      />
      
      <main class="main-container">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import BackendError from '@/components/BackendError.vue'
import Sidebar from '@/components/Sidebar.vue'
import TopHeader from '@/components/TopHeader.vue'
import { checkHealth, apiState } from '@/api'

const backendAvailable = ref(true)
const errorMessage = ref<string | null>(null)
const isChecking = ref(false)

onMounted(async () => {
  const isAvailable = await checkHealth()
  backendAvailable.value = isAvailable
  if (!isAvailable) {
    errorMessage.value = apiState.lastError
  }
})

watch(() => apiState.isBackendAvailable, (available) => {
  backendAvailable.value = available
  errorMessage.value = apiState.lastError
})

const onBackendConnected = () => {
  backendAvailable.value = true
  errorMessage.value = null
}

const checkBackendHealth = async () => {
  if (isChecking.value) return
  isChecking.value = true
  
  const isAvailable = await checkHealth()
  backendAvailable.value = isAvailable
  
  if (isAvailable) {
    ElMessage.success('Backend connection established')
  } else {
    ElMessage.error('Backend connection error')
  }
  
  isChecking.value = false
}
</script>

<style scoped lang="scss">
.app-layout {
  display: flex;
  min-height: 100vh;
  background: var(--bg-app);
}

.main-content-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0; // Prevent flex item overflow
}

.main-container {
  flex: 1;
  padding: 32px;
  max-width: 1600px; // Slightly wider for modern screens
  margin: 0 auto;
  width: 100%;
  overflow-x: hidden;
}

.error-wrapper {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-app);
}
</style>
