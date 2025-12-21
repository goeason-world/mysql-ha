import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/clusters'
    },
    {
      path: '/clusters',
      name: 'clusters',
      component: () => import('@/views/ClusterList.vue')
    },
    {
      path: '/clusters/create',
      name: 'create-cluster',
      component: () => import('@/views/CreateCluster.vue')
    },
    {
      path: '/clusters/:id',
      name: 'cluster-detail',
      component: () => import('@/views/ClusterDetail.vue')
    }
  ]
})

export default router
