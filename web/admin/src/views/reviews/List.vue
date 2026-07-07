<template>
  <div class="reviews-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>内容审核</span>
          <el-radio-group v-model="statusFilter" @change="fetchQueue">
            <el-radio-button label="review">待审核</el-radio-button>
            <el-radio-button label="published">已发布</el-radio-button>
            <el-radio-button label="archived">已归档</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <el-table :data="entries" v-loading="loading">
        <el-table-column prop="title" label="标题" min-width="200" />
        <el-table-column prop="createdBy" label="创建者" width="180">
          <template #default="{ row }">
            <span>{{ (row.createdBy || '').slice(0, 16) }}...</span>
          </template>
        </el-table-column>
        <el-table-column prop="category" label="分类" width="120" />
        <el-table-column prop="updatedAt" label="更新时间" width="160" />
        <el-table-column label="操作" fixed="right" width="220">
          <template #default="{ row }">
            <template v-if="row.status === 'review'">
              <el-button size="small" type="success" @click="handleApprove(row)">通过</el-button>
              <el-button size="small" type="danger" @click="handleReject(row)">拒绝</el-button>
            </template>
            <el-button v-if="row.status === 'published'" size="small" type="warning" @click="handleTakedown(row)">下架</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @size-change="fetchQueue"
          @current-change="fetchQueue"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listReviewQueue, approveEntry, rejectEntry, takedownEntry } from '@/api/reviews'

const loading = ref(false)
const entries = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const statusFilter = ref('review')

const fetchQueue = async () => {
  loading.value = true
  try {
    const res = await listReviewQueue({ status: statusFilter.value, page: currentPage.value, limit: pageSize.value })
    entries.value = res.entries || []
    total.value = res.total || 0
  } catch (e) {
    console.error('fetch review queue failed:', e)
  } finally {
    loading.value = false
  }
}

const handleApprove = async (row) => {
  try {
    await approveEntry(row.id)
    ElMessage.success('已通过')
    fetchQueue()
  } catch (e) { console.error('approve failed:', e) }
}

const promptReason = (title) => ElMessageBox.prompt('请输入原因', title, {
  confirmButtonText: '确定', cancelButtonText: '取消',
  inputPattern: /\S+/, inputErrorMessage: '原因不能为空'
}).catch(() => ({ value: null }))

const handleReject = async (row) => {
  const { value: reason } = await promptReason('拒绝条目')
  if (!reason) return
  try {
    await rejectEntry(row.id, reason)
    ElMessage.success('已拒绝')
    fetchQueue()
  } catch (e) { console.error('reject failed:', e) }
}

const handleTakedown = async (row) => {
  const { value: reason } = await promptReason('下架条目')
  if (!reason) return
  try {
    await takedownEntry(row.id, reason)
    ElMessage.success('已下架')
    fetchQueue()
  } catch (e) { console.error('takedown failed:', e) }
}

onMounted(() => { fetchQueue() })
</script>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
.pagination { margin-top: 16px; }
</style>
