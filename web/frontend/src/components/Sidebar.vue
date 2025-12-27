<template>
  <aside class="sidebar">
    <div class="logo">
      <div class="logo-icon">
        <span>🐬</span>
      </div>
      <div class="logo-text">
        <span class="logo-title">MySQL HA</span>
        <span class="logo-subtitle">管理控制台</span>
      </div>
    </div>
    
    <nav class="nav-menu">
      <div class="nav-section">
        <div class="nav-label">集群管理</div>
        
        <router-link to="/clusters" class="nav-item" :class="{ active: isActive('/clusters') }">
          <span class="nav-icon">📊</span>
          <span class="nav-text">集群列表</span>
        </router-link>
        
        <router-link to="/clusters/create" class="nav-item" :class="{ active: route.path === '/clusters/create' }">
          <span class="nav-icon">➕</span>
          <span class="nav-text">创建集群</span>
        </router-link>
      </div>
    </nav>
  </aside>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'

const route = useRoute()

const isActive = (path: string) => {
  return route.path === path || (route.path === '/' && path === '/clusters') || route.path.startsWith('/clusters/') && path === '/clusters' && route.path !== '/clusters/create'
}
</script>

<style scoped lang="scss">
.sidebar {
  width: 240px;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  height: 100vh;
  flex-shrink: 0;
}

.logo {
  height: 72px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid var(--border-color);
  gap: 14px;
  
  .logo-icon {
    width: 42px;
    height: 42px;
    background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
    border-radius: var(--radius-md);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 22px;
    box-shadow: 0 4px 12px rgba(6, 182, 212, 0.3);
  }
  
  .logo-text {
    display: flex;
    flex-direction: column;
    
    .logo-title {
      font-weight: 700;
      font-size: 17px;
      background: linear-gradient(135deg, var(--primary) 0%, #22d3ee 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
      line-height: 1.2;
    }
    
    .logo-subtitle {
      font-size: 11px;
      color: var(--text-muted);
      letter-spacing: 0.5px;
    }
  }
}

.nav-menu {
  flex: 1;
  padding: 20px 14px;
  overflow-y: auto;
}

.nav-section {
  margin-bottom: 24px;
}

.nav-label {
  padding: 0 14px;
  margin-bottom: 12px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--text-muted);
  letter-spacing: 0.08em;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin: 4px 0;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 14px;
  transition: var(--transition);
  position: relative;
  
  .nav-icon {
    font-size: 18px;
    width: 24px;
    text-align: center;
  }
  
  .nav-text {
    font-weight: 500;
  }
  
  &:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  
  &.active {
    background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
    color: white;
    box-shadow: 0 4px 12px rgba(6, 182, 212, 0.3);
    
    .nav-icon {
      filter: brightness(1.2);
    }
  }
}
</style>
