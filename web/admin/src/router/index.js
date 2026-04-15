import { createRouter, createWebHistory } from 'vue-router'
import { useAdminStore } from '@/stores/admin'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        redirect: '/stats'
      },
      {
        path: 'stats',
        name: 'Stats',
        component: () => import('@/views/stats/Index.vue'),
        meta: { permission: 4, title: '数据统计' }
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('@/views/users/List.vue'),
        meta: { permission: 4, title: '用户管理' }
      },
      {
        path: 'users/:publicKey',
        name: 'UserDetail',
        component: () => import('@/views/users/Detail.vue'),
        meta: { permission: 4, title: '用户详情' }
      },
      {
        path: 'entries',
        name: 'Entries',
        component: () => import('@/views/entries/List.vue'),
        meta: { permission: 4, title: '内容审核' }
      },
      {
        path: 'entries/:id',
        name: 'EntryDetail',
        component: () => import('@/views/entries/Detail.vue'),
        meta: { permission: 4, title: '条目详情' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  const adminStore = useAdminStore()

  if (to.meta.requiresAuth !== false && !adminStore.isLoggedIn) {
    next('/login')
    return
  }

  // 权限检查
  if (to.meta.permission && adminStore.userLevel < to.meta.permission) {
    next('/stats') // 跳转到权限允许的页面
    return
  }

  next()
})

export default router
