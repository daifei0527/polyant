<!-- web/admin/src/views/entries/List.vue -->
<template>
  <div class="entries-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>内容审核</span>
          <div>
            <el-select v-model="selectedCategory" placeholder="选择分类" clearable style="width: 150px; margin-right: 10px">
              <el-option v-for="cat in categories" :key="cat.id" :label="cat.name" :value="cat.id" />
            </el-select>
            <el-input
              v-model="searchText"
              placeholder="搜索内容"
              style="width: 200px"
              clearable
              @clear="fetchEntries"
              @keyup.enter="fetchEntries"
            >
              <template #append>
                <el-button icon="Search" @click="fetchEntries" />
              </template>
            </el-input>
          </div>
        </div>
      </template>

      <el-alert type="info" :closable="false" style="margin-bottom: 16px">
        内容审核功能需要后端支持条目管理 API，当前为占位页面。
      </el-alert>

      <el-empty description="内容审核功能开发中" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const searchText = ref('')
const selectedCategory = ref('')
const categories = ref([])

const fetchEntries = async () => {
  // Placeholder - would fetch entries from API
}

onMounted(() => {
  // Placeholder
})
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
