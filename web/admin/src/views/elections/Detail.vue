<template>
  <div class="election-detail">
    <el-page-header @back="router.back()" :content="election?.title || '选举详情'" style="margin-bottom: 16px;" />
    <el-card v-if="election" style="margin-bottom: 16px;">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="标题">{{ election.title }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ election.status === 'active' ? '进行中' : '已关闭' }}</el-descriptions-item>
        <el-descriptions-item label="描述">{{ election.description }}</el-descriptions-item>
        <el-descriptions-item label="当选阈值">{{ election.voteThreshold }}</el-descriptions-item>
        <el-descriptions-item label="自动当选">{{ election.autoElect ? '是' : '否' }}</el-descriptions-item>
        <el-descriptions-item label="创建者">{{ (election.createdBy || '').slice(0, 16) }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
    <el-card>
      <template #header><span>候选人</span></template>
      <el-table :data="candidates" v-loading="loading">
        <el-table-column prop="userName" label="名称" />
        <el-table-column prop="voteCount" label="票数" width="100" />
        <el-table-column prop="status" label="状态" width="120"><template #default="{ row }"><el-tag :type="row.status === 'elected' ? 'success' : row.status === 'rejected' ? 'danger' : ''">{{ row.status }}</el-tag></template></el-table-column>
        <el-table-column prop="confirmed" label="已确认" width="100"><template #default="{ row }">{{ row.confirmed ? '是' : '否' }}</template></el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getElection } from '@/api/elections'

const route = useRoute()
const router = useRouter()
const election = ref(null)
const candidates = ref([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    const res = await getElection(route.params.id)
    election.value = res.election
    candidates.value = res.candidates || []
  } catch (e) { console.error('fetch election:', e) }
  finally { loading.value = false }
})
</script>
