<template>
  <div class="backend-error">
    <div class="error-container">
      <div class="error-icon">
        <svg xmlns="http://www.w3.org/2000/svg" width="80" height="80" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"></circle>
          <line x1="12" y1="8" x2="12" y2="12"></line>
          <line x1="12" y1="16" x2="12.01" y2="16"></line>
        </svg>
      </div>
      <h1>服务不可用</h1>
      <p class="error-message">{{ message || '无法连接到后端服务' }}</p>
      <p class="error-hint">请检查后端服务是否正常运行在 <code>localhost:8888</code></p>
      <div class="actions">
        <button @click="retry" :disabled="checking" class="retry-btn">
          <span v-if="checking">检查中...</span>
          <span v-else>重试连接</span>
        </button>
      </div>
      <div v-if="retryCount > 0" class="retry-info">
        已重试 {{ retryCount }} 次
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { checkHealth } from '@/api'

defineProps<{
  message?: string
}>()

const emit = defineEmits<{
  (e: 'connected'): void
}>()

const checking = ref(false)
const retryCount = ref(0)

const retry = async () => {
  checking.value = true
  retryCount.value++
  
  const isAvailable = await checkHealth()
  
  checking.value = false
  
  if (isAvailable) {
    emit('connected')
  }
}
</script>

<style scoped>
.backend-error {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
  color: #fff;
}

.error-container {
  text-align: center;
  padding: 40px;
  max-width: 500px;
}

.error-icon {
  color: #ff6b6b;
  margin-bottom: 20px;
}

h1 {
  font-size: 2rem;
  margin-bottom: 16px;
  color: #fff;
}

.error-message {
  font-size: 1.1rem;
  color: #ccc;
  margin-bottom: 12px;
}

.error-hint {
  font-size: 0.9rem;
  color: #888;
  margin-bottom: 24px;
}

.error-hint code {
  background: rgba(255, 255, 255, 0.1);
  padding: 2px 8px;
  border-radius: 4px;
  font-family: monospace;
}

.actions {
  margin-top: 24px;
}

.retry-btn {
  background: #4a90d9;
  color: #fff;
  border: none;
  padding: 12px 32px;
  font-size: 1rem;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.retry-btn:hover:not(:disabled) {
  background: #5a9fe9;
  transform: translateY(-2px);
}

.retry-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.retry-info {
  margin-top: 16px;
  font-size: 0.85rem;
  color: #666;
}
</style>
