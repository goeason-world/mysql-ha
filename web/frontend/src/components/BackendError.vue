<template>
  <div class="backend-error">
    <div class="error-container">
      <div class="error-visual">
        <div class="error-icon-wrapper">
          <svg class="error-icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M12 9v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
        <div class="pulse-ring"></div>
        <div class="pulse-ring delay"></div>
      </div>
      
      <h1 class="error-title">服务连接失败</h1>
      <p class="error-message">{{ message || '无法连接到后端服务' }}</p>
      
      <div class="error-details">
        <div class="detail-item">
          <span class="detail-icon">🔗</span>
          <span class="detail-text">API 地址: <code>localhost:8888</code></span>
        </div>
        <div class="detail-item">
          <span class="detail-icon">📡</span>
          <span class="detail-text">请确保后端服务正在运行</span>
        </div>
        <div class="detail-item">
          <span class="detail-icon">💡</span>
          <span class="detail-text">运行命令: <code>./webadmin</code></span>
        </div>
      </div>
      
      <button 
        class="retry-button" 
        @click="retry" 
        :disabled="checking"
        :class="{ loading: checking }"
      >
        <span v-if="checking" class="button-spinner"></span>
        <span v-else class="button-icon">🔄</span>
        <span>{{ checking ? '检查中...' : '重试连接' }}</span>
      </button>
      
      <div v-if="retryCount > 0" class="retry-count">
        已尝试 {{ retryCount }} 次
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

<style scoped lang="scss">
.backend-error {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: var(--bg-app);
  padding: 20px;
}

.error-container {
  text-align: center;
  max-width: 440px;
}

.error-visual {
  position: relative;
  width: 110px;
  height: 110px;
  margin: 0 auto 32px;
}

.error-icon-wrapper {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--danger-bg);
  border-radius: 50%;
  z-index: 1;
  border: 2px solid rgba(239, 68, 68, 0.3);
}

.error-icon {
  width: 44px;
  height: 44px;
  color: var(--danger);
}

.pulse-ring {
  position: absolute;
  inset: 0;
  border: 2px solid var(--danger);
  border-radius: 50%;
  opacity: 0;
  animation: pulse 2s ease-out infinite;
  
  &.delay {
    animation-delay: 1s;
  }
}

@keyframes pulse {
  0% {
    transform: scale(1);
    opacity: 0.4;
  }
  100% {
    transform: scale(1.5);
    opacity: 0;
  }
}

.error-title {
  font-size: 26px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 12px;
}

.error-message {
  font-size: 15px;
  color: var(--text-secondary);
  margin-bottom: 32px;
}

.error-details {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 20px;
  margin-bottom: 32px;
  text-align: left;
}

.detail-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 0;
  
  &:not(:last-child) {
    border-bottom: 1px solid var(--border-color);
  }
  
  .detail-icon {
    font-size: 20px;
    width: 28px;
    text-align: center;
  }
  
  .detail-text {
    font-size: 14px;
    color: var(--text-secondary);
    
    code {
      background: var(--bg-app);
      padding: 3px 10px;
      border-radius: 6px;
      font-family: 'SF Mono', Monaco, monospace;
      color: var(--primary);
      font-size: 13px;
    }
  }
}

.retry-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 14px 32px;
  background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
  color: white;
  border: none;
  border-radius: var(--radius-md);
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  min-width: 180px;
  
  &:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: var(--shadow-glow);
  }
  
  &:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }
  
  &.loading {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    color: var(--text-secondary);
  }
  
  .button-icon {
    font-size: 18px;
  }
}

.button-spinner {
  width: 18px;
  height: 18px;
  border: 2px solid var(--border-color);
  border-top-color: var(--primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.retry-count {
  margin-top: 16px;
  font-size: 13px;
  color: var(--text-muted);
}
</style>
