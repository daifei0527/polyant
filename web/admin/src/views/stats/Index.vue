<!-- web/admin/src/views/stats/Index.vue -->
<template>
  <div class="stats-index">
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value">{{ userStats.total || 0 }}</div>
            <div class="stat-label">总用户数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value">{{ entryStats.total || 0 }}</div>
            <div class="stat-label">总条目数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value">{{ contributionStats.total || 0 }}</div>
            <div class="stat-label">总贡献数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value">{{ ratingStats.total || 0 }}</div>
            <div class="stat-label">总评分数</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>用户等级分布</span>
          </template>
          <div v-for="item in userStats.level_distribution" :key="item.level" class="level-item">
            <span>Lv{{ item.level }}</span>
            <el-progress :percentage="getPercentage(item.count)" :stroke-width="20" />
            <span>{{ item.count }} 人</span>
          </div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>活跃趋势 (近 7 天)</span>
          </template>
          <el-table :data="activityTrend" size="small">
            <el-table-column prop="date" label="日期" width="120" />
            <el-table-column prop="active_users" label="活跃用户" />
            <el-table-column prop="new_entries" label="新增条目" />
            <el-table-column prop="new_ratings" label="新增评分" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px">
      <el-col :span="24">
        <el-card>
          <template #header>
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <span>KV 数据备份</span>
              <el-button type="primary" :loading="backupLoading" @click="handleCreateBackup">
                创建备份
              </el-button>
            </div>
          </template>
          <el-table :data="backups" size="small" empty-text="暂无备份">
            <el-table-column prop="created_at" label="创建时间" width="180">
              <template #default="{ row }">
                {{ formatTime(row.created_at) }}
              </template>
            </el-table-column>
            <el-table-column prop="engine" label="引擎" width="80" />
            <el-table-column prop="key_count" label="Key 数量" width="100" />
            <el-table-column prop="size_bytes" label="大小">
              <template #default="{ row }">
                {{ formatSize(row.size_bytes) }}
              </template>
            </el-table-column>
            <el-table-column prop="dir" label="目录" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getUserStats, getActivityTrend, getContributionStats, getEntryStats } from '@/api/stats'
import { createBackup, listBackups } from '@/api/backup'

const userStats = ref({})
const activityTrend = ref([])
const contributionStats = ref({})

const entryStats = ref({ total: 0 })
const ratingStats = ref({ total: 0 })

const backups = ref([])
const backupLoading = ref(false)

const fetchData = async () => {
  try {
    const [userRes, activityRes, contribRes, entryRes] = await Promise.all([
      getUserStats(),
      getActivityTrend(7),
      getContributionStats({ limit: 1 }),
      getEntryStats()
    ])
    userStats.value = userRes || {}
    activityTrend.value = activityRes?.trend || []
    contributionStats.value = contribRes || {}
    entryStats.value = entryRes || {}
  } catch (error) {
    console.error('Failed to fetch stats:', error)
  }
}

const fetchBackups = async () => {
  try {
    const res = await listBackups()
    backups.value = res?.backups || []
  } catch (error) {
    console.error('Failed to fetch backups:', error)
  }
}

const handleCreateBackup = async () => {
  backupLoading.value = true
  try {
    await createBackup()
    ElMessage.success('备份创建成功')
    await fetchBackups()
  } catch (error) {
    ElMessage.error('备份创建失败: ' + (error.message || error))
  } finally {
    backupLoading.value = false
  }
}

const formatTime = (ts) => {
  if (!ts) return ''
  return new Date(ts).toLocaleString()
}

const formatSize = (bytes) => {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return size.toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

const getPercentage = (count) => {
  const total = userStats.value.total || 1
  return Math.round((count / total) * 100)
}

onMounted(() => {
  fetchData()
  fetchBackups()
})
</script>

<style scoped>
.stat-card {
  text-align: center;
  padding: 20px 0;
}

.stat-value {
  font-size: 36px;
  font-weight: bold;
  color: #409EFF;
}

.stat-label {
  margin-top: 10px;
  color: #909399;
}

.level-item {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
}

.level-item span:first-child {
  width: 40px;
}

.level-item span:last-child {
  width: 60px;
  text-align: right;
}

.level-item .el-progress {
  flex: 1;
  margin: 0 10px;
}
</style>
