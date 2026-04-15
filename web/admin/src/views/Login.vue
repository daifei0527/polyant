<!-- web/admin/src/views/Login.vue -->
<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <h2>Polyant 管理后台</h2>
      </template>

      <el-form :model="form" :rules="rules" ref="formRef" label-position="top">
        <el-form-item label="公钥" prop="publicKey">
          <el-input
            v-model="form.publicKey"
            type="textarea"
            :rows="3"
            placeholder="请输入您的 Ed25519 公钥"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="handleLogin" :loading="loading" style="width: 100%">
            登录
          </el-button>
        </el-form-item>
      </el-form>

      <el-divider />

      <p class="hint">
        管理后台仅限本地访问，请使用已注册的 Ed25519 公钥登录
      </p>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAdminStore } from '@/stores/admin'

const router = useRouter()
const adminStore = useAdminStore()

const formRef = ref(null)
const loading = ref(false)

const form = reactive({
  publicKey: ''
})

const rules = {
  publicKey: [
    { required: true, message: '请输入公钥', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const success = await adminStore.login(form.publicKey)
    if (success) {
      ElMessage.success('登录成功')
      router.push('/')
    } else {
      ElMessage.error('登录失败，请检查公钥是否正确')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: #f5f7fa;
}

.login-card {
  width: 400px;
}

.login-card :deep(.el-card__header) {
  text-align: center;
}

.hint {
  color: #909399;
  font-size: 12px;
  text-align: center;
}
</style>
