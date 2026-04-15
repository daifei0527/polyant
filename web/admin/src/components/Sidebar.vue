<template>
  <div class="sidebar">
    <div class="logo">
      <h1>Polyant</h1>
    </div>
    <el-menu
      :default-active="activeMenu"
      router
      background-color="#304156"
      text-color="#bfcbd9"
      active-text-color="#409EFF"
    >
      <el-menu-item index="/stats">
        <el-icon><DataLine /></el-icon>
        <span>数据统计</span>
      </el-menu-item>
      <el-menu-item index="/users" v-if="hasPermission(4)">
        <el-icon><User /></el-icon>
        <span>用户管理</span>
      </el-menu-item>
      <el-menu-item index="/entries" v-if="hasPermission(4)">
        <el-icon><Document /></el-icon>
        <span>内容审核</span>
      </el-menu-item>
    </el-menu>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { DataLine, User, Document } from '@element-plus/icons-vue'
import { useAdminStore } from '@/stores/admin'

const route = useRoute()
const adminStore = useAdminStore()

const activeMenu = computed(() => route.path)

const hasPermission = (level) => adminStore.hasPermission(level)
</script>

<style scoped>
.sidebar {
  height: 100%;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.logo h1 {
  font-size: 18px;
  margin: 0;
}

.el-menu {
  border-right: none;
}
</style>
