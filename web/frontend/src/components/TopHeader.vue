<template>
  <header class="top-header">
    <div class="header-left">
      <div class="breadcrumb">
        <span class="breadcrumb-icon">🏠</span>
        <span class="breadcrumb-text">控制台</span>
      </div>
    </div>
    
    <div class="header-right">
      <div 
        class="status-indicator" 
        :class="{ checking: isChecking, online: backendAvailable, offline: !backendAvailable }" 
        @click="checkBackendHealth"
        :title="backendAvailable ? '后端服务正常' : '后端服务离线，点击重试'"
      >
        <span class="status-dot"></span>
        <span class="status-text">{{ statusText }}</span>
        <span v-if="isChecking" class="status-spinner"></span>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  backendAvailable: boolean
  isChecking: boolean
}>()

const emit = defineEmits<{
  (e: 'check-health'): void
}>()

const statusText = computed(() => {
  if (props.isChecking) return '检查中...'
  return props.backendAvailable ? '已连接' : '已断开'
})

const checkBackendHealth = () => {
  emit('check-health')
}
</script>

<style scoped lang="scss">
.top-header {
  height: 64px;
  background: var(--bg-header);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 32px;
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--text-secondary);
  font-size: 14px;
  
  .breadcrumb-icon {
    font-size: 16px;
  }
  
  .breadcrumb-text {
    font-weight: 500;
  }
}

.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 16px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 24px;
  cursor: pointer;
  transition: var(--transition);
  
  &:hover {
    background: var(--bg-hover);
    border-color: var(--border-light);
    transform: translateY(-1px);
  }
  
  &.checking {
    opacity: 0.8;
    cursor: wait;
  }
  
  &.online {
    border-color: rgba(16, 185, 129, 0.3);
    
    .status-dot {
      background: var(--success);
      box-shadow: 0 0 10px var(--success);
    }
  }
  
  &.offline {
    border-color: rgba(239, 68, 68, 0.3);
    
    .status-dot {
      background: var(--danger);
      box-shadow: 0 0 10px var(--danger);
      animation: pulse-danger 2s infinite;
    }
  }
  
  .status-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--text-muted);
    transition: all 0.3s ease;
  }
  
  .status-text {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary);
  }
  
  .status-spinner {
    width: 14px;
    height: 14px;
    border: 2px solid var(--border-color);
    border-top-color: var(--primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
}

@keyframes pulse-danger {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
