<!-- web/admin/src/views/users/List.vue -->
<template>
  <div class="users-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>用户列表</span>
          <el-input
            v-model="searchText"
            placeholder="搜索用户"
            style="width: 200px"
            clearable
            @clear="fetchUsers"
            @keyup.enter="fetchUsers"
          >
            <template #append>
              <el-button icon="Search" @click="fetchUsers" />
            </template>
          </el-input>
        </div>
      </template>

      <el-table :data="users" v-loading="loading">
        <el-table-column prop="public_key" label="公钥" width="200">
          <template #default="{ row }">
            <el-tooltip :content="row.public_key" placement="top">
              <span>{{ row.public_key.slice(0, 20) }}...</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="agent_name" label="名称" width="150" />
        <el-table-column prop="user_level" label="等级" width="80">
          <template #default="{ row }">
            <el-tag :type="getLevelType(row.user_level)">
              Lv{{ row.user_level }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'">
              {{ row.status === 'active' ? '正常' : '封禁' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="contribution_cnt" label="贡献数" width="100" />
        <el-table-column prop="rating_cnt" label="评分数" width="100" />
        <el-table-column label="操作" fixed="right" width="200">
          <template #default="{ row }">
            <el-button size="small" @click="showDetail(row)">详情</el-button>
            <el-button
              v-if="row.status === 'active'"
              size="small"
              type="danger"
              @click="handleBan(row)"
            >封禁</el-button>
            <el-button
              v-else
              size="small"
              type="success"
              @click="handleUnban(row)"
            >解封</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @size-change="fetchUsers"
          @current-change="fetchUsers"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listUsers, banUser, unbanUser } from '@/api/users'

const router = useRouter()

const loading = ref(false)
const users = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const searchText = ref('')

const fetchUsers = async () => {
  loading.value = true
  try {
    const res = await listUsers({
      page: currentPage.value,
      limit: pageSize.value,
      search: searchText.value
    })
    users.value = res.users || []
    total.value = res.total || 0
  } catch (error) {
    console.error('Failed to fetch users:', error)
  } finally {
    loading.value = false
  }
}

const showDetail = (row) => {
  router.push(`/users/${row.public_key}`)
}

const handleBan = async (row) => {
  const { value: reason } = await ElMessageBox.prompt('请输入封禁原因', '封禁用户', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputPattern: /\S+/,
    inputErrorMessage: '请输入封禁原因'
  }).catch(() => ({ value: null }))

  if (!reason) return

  try {
    await banUser(row.public_key, reason)
    ElMessage.success('封禁成功')
    fetchUsers()
  } catch (error) {
    console.error('Ban failed:', error)
  }
}

const handleUnban = async (row) => {
  try {
    await unbanUser(row.public_key)
    ElMessage.success('解封成功')
    fetchUsers()
  } catch (error) {
    console.error('Unban failed:', error)
  }
}

const getLevelType = (level) => {
  const types = { 0: 'info', 1: '', 2: 'success', 3: 'warning', 4: 'danger', 5: 'danger' }
  return types[level] || 'info'
}

onMounted(() => {
  fetchUsers()
})
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
