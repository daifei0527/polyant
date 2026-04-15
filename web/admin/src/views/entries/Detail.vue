<!-- web/admin/src/views/entries/Detail.vue -->
<template>
  <div class="entry-detail">
    <el-page-header @back="goBack" title="返回">
      <template #content>
        <span class="text-large font-600 mr-3">条目详情</span>
      </template>
    </el-page-header>

    <el-card style="margin-top: 20px">
      <el-alert type="info" :closable="false" style="margin-bottom: 16px">
        条目详情功能需要后端支持条目管理 API，当前为占位页面。
      </el-alert>

      <el-empty description="条目详情功能开发中" />
    </el-card>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'

const router = useRouter()

const goBack = () => {
  router.push('/entries')
}
</script>

<style scoped>
.entry-detail {
  padding: 20px;
}
</style>
