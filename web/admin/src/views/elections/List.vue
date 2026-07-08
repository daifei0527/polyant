<template>
  <div class="elections-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>选举管理</span>
          <div>
            <el-radio-group v-model="statusFilter" @change="fetchElections" style="margin-right: 12px;">
              <el-radio-button label="">全部</el-radio-button>
              <el-radio-button label="active">进行中</el-radio-button>
              <el-radio-button label="closed">已关闭</el-radio-button>
            </el-radio-group>
            <el-button type="primary" @click="showCreate = true">创建选举</el-button>
          </div>
        </div>
      </template>
      <el-table :data="elections" v-loading="loading">
        <el-table-column prop="title" label="标题" min-width="180" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }"><el-tag :type="row.status === 'active' ? 'success' : 'info'">{{ row.status === 'active' ? '进行中' : '已关闭' }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="voteThreshold" label="阈值" width="80" />
        <el-table-column prop="autoElect" label="自动当选" width="100"><template #default="{ row }">{{ row.autoElect ? '是' : '否' }}</template></el-table-column>
        <el-table-column label="操作" fixed="right" width="180">
          <template #default="{ row }">
            <el-button size="small" @click="router.push(`/elections/${row.id}`)">详情</el-button>
            <el-button v-if="row.status === 'active'" size="small" type="warning" @click="handleClose(row)">关闭</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showCreate" title="创建选举" width="500px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" /></el-form-item>
        <el-form-item label="当选阈值"><el-input-number v-model="form.vote_threshold" :min="1" /></el-form-item>
        <el-form-item label="持续天数"><el-input-number v-model="form.duration_days" :min="1" /></el-form-item>
        <el-form-item label="自动当选"><el-switch v-model="form.auto_elect" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listElections, createElection, closeElection } from '@/api/elections'

const router = useRouter()
const loading = ref(false)
const elections = ref([])
const statusFilter = ref('active')
const showCreate = ref(false)
const creating = ref(false)
const form = ref({ title: '', description: '', vote_threshold: 1, duration_days: 7, auto_elect: true })

const fetchElections = async () => {
  loading.value = true
  try {
    const res = await listElections({ status: statusFilter.value })
    elections.value = res.elections || []
  } catch (e) { console.error('fetch elections:', e) }
  finally { loading.value = false }
}
const handleCreate = async () => {
  creating.value = true
  try {
    await createElection(form.value)
    ElMessage.success('创建成功')
    showCreate.value = false
    fetchElections()
  } catch (e) { ElMessage.error('创建失败: ' + (e.message || e)) }
  finally { creating.value = false }
}
const handleClose = async (row) => {
  try {
    await ElMessageBox.confirm(`确认关闭选举「${row.title}」？`, '关闭选举', { type: 'warning' })
    await closeElection(row.id)
    ElMessage.success('已关闭')
    fetchElections()
  } catch (e) { if (e !== 'cancel') ElMessage.error('关闭失败: ' + (e.message || e)) }
}
onMounted(() => { fetchElections() })
</script>

<style scoped>.card-header { display: flex; justify-content: space-between; align-items: center; }</style>
