// web/admin/src/stores/admin.js
import { defineStore } from 'pinia'
import { login as loginApi, getCurrentUser } from '@/api/session'

// 持久化 token + user + userLevel 到 sessionStorage，使页面刷新后路由守卫仍能读到
// 正确的 userLevel（修复此前刷新后 userLevel 归零导致权限守卫重定向死循环）。
export const useAdminStore = defineStore('admin', {
  state: () => ({
    token: sessionStorage.getItem('admin_token') || '',
    user: JSON.parse(sessionStorage.getItem('admin_user') || 'null'),
    userLevel: Number(sessionStorage.getItem('admin_userLevel') || 0)
  }),

  getters: {
    isLoggedIn: (state) => !!state.token,
    publicKey: (state) => state.user?.public_key || ''
  },

  actions: {
    async login(identifier, password) {
      try {
        const res = await loginApi(identifier, password)
        // res = { token, expires_at, user: { public_key, agent_name, user_level } }
        this.token = res.token
        this.user = res.user
        this.userLevel = res.user.user_level
        sessionStorage.setItem('admin_token', res.token)
        sessionStorage.setItem('admin_user', JSON.stringify(res.user))
        sessionStorage.setItem('admin_userLevel', String(res.user.user_level))
        return true
      } catch (error) {
        console.error('Login failed:', error)
        return false
      }
    },

    // 刷新后用 token 重新拉取用户信息，校验会话有效性；失败则登出。
    async restoreSession() {
      if (!this.token) return false
      try {
        const u = await getCurrentUser()
        this.user = u
        this.userLevel = u.user_level
        sessionStorage.setItem('admin_user', JSON.stringify(u))
        sessionStorage.setItem('admin_userLevel', String(u.user_level))
        return true
      } catch (error) {
        this.logout()
        return false
      }
    },

    logout() {
      this.token = ''
      this.user = null
      this.userLevel = 0
      sessionStorage.removeItem('admin_token')
      sessionStorage.removeItem('admin_user')
      sessionStorage.removeItem('admin_userLevel')
    },

    hasPermission(level) {
      return this.userLevel >= level
    }
  }
})
