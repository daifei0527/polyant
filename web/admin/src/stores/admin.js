// web/admin/src/stores/admin.js
import { defineStore } from 'pinia'
import { createSession, getCurrentUser } from '@/api/session'

export const useAdminStore = defineStore('admin', {
  state: () => ({
    token: sessionStorage.getItem('admin_token') || '',
    user: null,
    userLevel: 0
  }),

  getters: {
    isLoggedIn: (state) => !!state.token,
    publicKey: (state) => state.user?.public_key || ''
  },

  actions: {
    async login(publicKey) {
      try {
        const res = await createSession(publicKey)
        this.token = res.token
        this.user = res.user
        this.userLevel = res.user.user_level
        sessionStorage.setItem('admin_token', res.token)
        return true
      } catch (error) {
        console.error('Login failed:', error)
        return false
      }
    },

    logout() {
      this.token = ''
      this.user = null
      this.userLevel = 0
      sessionStorage.removeItem('admin_token')
    },

    hasPermission(level) {
      return this.userLevel >= level
    }
  }
})
