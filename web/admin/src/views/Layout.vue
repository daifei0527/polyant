<template>
  <el-container class="layout-container">
    <el-aside width="200px">
      <Sidebar />
    </el-aside>
    <el-container>
      <el-header>
        <Header />
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import Sidebar from '@/components/Sidebar.vue'
import Header from '@/components/Header.vue'
</script>

<style scoped>
.layout-container {
  height: 100vh;
}

.el-header {
  padding: 0;
  background: #fff;
  border-bottom: 1px solid #e6e6e6;
}

.el-aside {
  background: #304156;
}

.el-main {
  background: #f5f7fa;
  padding: 20px;
}
</style>
