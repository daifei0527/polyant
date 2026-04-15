<!-- web/admin/src/views/users/Detail.vue -->
<template>
  <div class="user-detail">
    <el-page-header @back="goBack" title="返回">
      <template #content>
        <span class="text-large font-600 mr-3">用户详情</span>
      </template>
    </el-page-header>

    <el-card v-loading="loading" style="margin-top: 20px">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="公钥">
          <el-tooltip :content="user.public_key" placement="top">
            <span>{{ user.public_key }}</span>
          </el-tooltip>
        </el-descriptions-item>
        <el-descriptions-item label="名称">{{ user.agent_name || '-' }}</el-descriptions-item>
        <el-descriptions-item label="等级">
          <el-tag :type="getLevelType(user.user_level)">Lv{{ user.user_level }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="user.status === 'active' ? 'success' : 'danger'">
            {{ user.status === 'active' ? '正常' : '封禁' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="贡献数">{{ user.contribution_cnt || 0 }}</el-descriptions-item>
        <el-descriptions-item label="评分数">{{ user.rating_cnt || 0 }}</el-descriptions-item>
        <el-descriptions-item label="邮箱">{{ user.email || '-' }}</el-descriptions-item>
        <el-descriptions-item label="注册时间">{{ formatDate(user.created_at) }}</el-descriptions-item>
      </el-descriptions>

      <div style="margin-top: 20px">
        <el-button v-if="user.status === 'active'" type="danger" @click="handleBan">封禁用户</el-button>
        <el-button v-else type="success" @click="handleUnban">解封用户</el-button>
        <el-button @click="showLevelDialog = true">修改等级</el-button>
      </div>
    </el-card>

    <!-- 修改等级对话框 -->
    <el-dialog v-model="showLevelDialog" title="修改用户等级" width="400px">
      <el-form :model="levelForm" label-width="80px">
        <el-form-item label="等级">
          <el-select v-model="levelForm.level" style="width: 100%">
            <el-option v-for="i in 6" :key="i - 1" :label="`Lv${i - 1}`" :value="i - 1" />
          </el-select>
        </el-form-item>
        <el-form-item label="原因">
          <el-input v-model="levelForm.reason" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showLevelDialog = false">取消</el-button>
        <el-button type="primary" @click="handleSetLevel">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listUsers, banUser, unbanUser, setUserLevel } from '@/api/users'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const user = ref({})
const showLevelDialog = ref(false)
const levelForm = ref({ level: 0, reason: '' })

const publicKey = route.params.publicKey

const fetchUser = async () => {
  loading.value = true
  try {
    // 通过列表接口获取用户信息
    const res = await listUsers({ search: publicKey })
    const users = res.users || []
    const found = users.find(u => u.public_key === publicKey)
    if (found) {
      user.value = found
    } else {
      ElMessage.error('用户不存在')
      router.push('/users')
    }
  } catch (error) {
    console.error('Failed to fetch user:', error)
  } finally {
    loading.value = false
  }
}

const goBack = () => {
  router.push('/users')
}

const handleBan = async () => {
  const { value: reason } = await ElMessageBox.prompt('请输入封禁原因', '封禁用户', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputPattern: /\S+/,
    inputErrorMessage: '请输入封禁原因'
  }).catch(() => ({ value: null }))

  if (!reason) return

  try {
    await banUser(publicKey, reason)
    ElMessage.success('封禁成功')
    fetchUser()
  } catch (error) {
    console.error('Ban failed:', error)
  }
}

const handleUnban = async () => {
  try {
    await unbanUser(publicKey)
    ElMessage.success('解封成功')
    fetchUser()
  } catch (error) {
    console.error('Unban failed:', error)
  }
}

const handleSetLevel = async () => {
  try {
    await setUserLevel(publicKey, levelForm.value.level, levelForm.value.reason)
    ElMessage.success('等级修改成功')
    showLevelDialog.value = false
    fetchUser()
  } catch (error) {
    console.error('Set level failed:', error)
  }
}

const getLevelType = (level) => {
  const types = { 0: 'info', 1: '', 2: 'success', 3: 'warning', 4: 'danger', 5: 'danger' }
  return types[level] || 'info'
}

const formatDate = (date) => {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}

onMounted(() => {
  fetchUser()
})
</script>

<style scoped>
.user-detail {
  padding: 20px;
}
</style>
