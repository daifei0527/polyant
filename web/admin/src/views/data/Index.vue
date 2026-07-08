<template>
  <div class="data-page">
    <!-- 导出 -->
    <el-card style="margin-bottom: 16px;">
      <template #header>
        <div class="card-header">
          <span>数据导出</span>
          <el-button type="primary" :loading="exportLoading" @click="handleExport">导出 ZIP</el-button>
        </div>
      </template>
      <el-checkbox-group v-model="exportInclude">
        <el-checkbox label="entries">条目</el-checkbox>
        <el-checkbox label="categories">分类</el-checkbox>
        <el-checkbox label="users">用户</el-checkbox>
        <el-checkbox label="ratings">评分</el-checkbox>
      </el-checkbox-group>
    </el-card>

    <!-- 导入 -->
    <el-card>
      <template #header>
        <div class="card-header">
          <span>数据导入</span>
          <el-button type="primary" :loading="importLoading" :disabled="!importFile" @click="handleImport">导入</el-button>
        </div>
      </template>
      <el-upload
        :auto-upload="false"
        :limit="1"
        :on-change="handleFileChange"
        :on-exceed="() => ElMessage.warning('仅支持单个 ZIP 文件')"
        accept=".zip"
      >
        <el-button>选择 ZIP 文件</el-button>
      </el-upload>
      <div style="margin-top: 12px;">
        <span>冲突策略：</span>
        <el-radio-group v-model="conflict">
          <el-radio label="skip">跳过</el-radio>
          <el-radio label="overwrite">覆盖</el-radio>
          <el-radio label="merge">合并</el-radio>
        </el-radio-group>
      </div>
      <div v-if="importResult" style="margin-top: 16px;">
        <el-alert :title="importResult.success ? '导入成功' : '导入完成（有错误/跳过）'" :type="importResult.success ? 'success' : 'warning'" :closable="false" />
        <el-descriptions :column="2" border style="margin-top: 8px;">
          <el-descriptions-item label="条目导入">{{ importResult.summary?.entries_imported || 0 }}</el-descriptions-item>
          <el-descriptions-item label="条目跳过">{{ importResult.summary?.entries_skipped || 0 }}</el-descriptions-item>
          <el-descriptions-item label="用户导入">{{ importResult.summary?.users_imported || 0 }}</el-descriptions-item>
          <el-descriptions-item label="分类导入">{{ importResult.summary?.categories_imported || 0 }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="importResult.errors?.length" style="margin-top: 8px; max-height: 200px; overflow:auto;">
          <div v-for="(e, i) in importResult.errors" :key="i" style="color: #e6a23c; font-size: 12px;">
            [{{ e.type }}/{{ e.id }}] {{ e.message }}
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { exportData, importData } from '@/api/data'

const exportInclude = ref(['entries', 'categories'])
const exportLoading = ref(false)
const importFile = ref(null)
const conflict = ref('skip')
const importLoading = ref(false)
const importResult = ref(null)

const handleExport = async () => {
  exportLoading.value = true
  try {
    const res = await exportData(exportInclude.value)
    const blob = new Blob([res.data], { type: 'application/zip' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `polyant-export-${new Date().toISOString().slice(0, 10)}.zip`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch (error) {
    if (error.response?.status === 401) {
      sessionStorage.removeItem('admin_token')
      window.location.href = '/admin/login'
    }
    ElMessage.error('导出失败: ' + (error.message || error))
  } finally {
    exportLoading.value = false
  }
}

const handleFileChange = (file) => {
  importFile.value = file.raw
}

const handleImport = async () => {
  if (!importFile.value) return
  importLoading.value = true
  importResult.value = null
  try {
    const result = await importData(importFile.value, conflict.value)
    importResult.value = result
    ElMessage.success('导入完成')
  } catch (error) {
    ElMessage.error('导入失败: ' + (error.message || error))
  } finally {
    importLoading.value = false
  }
}
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
</style>
