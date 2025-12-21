<template>
  <div class="app-wrapper">
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
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
</script>
