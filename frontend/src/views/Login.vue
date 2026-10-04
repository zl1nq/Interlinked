<template>
  <AuthShell>
    <h1 class="login-title"><span class="accent">Inter</span><span class="accent-blue">Linked</span></h1>
    <!-- <p class="login-subtitle">千万级推拉混合 Feed 流</p> -->

    <el-form ref="formRef" :model="form" :rules="rules" label-position="top" size="large" @submit.prevent="handleLogin">
      <el-form-item label="用户名" prop="username">
        <el-input v-model="form.username" placeholder="用户名" prefix-icon="User" />
      </el-form-item>
      <el-form-item label="密码" prop="password">
        <el-input v-model="form.password" type="password" placeholder="密码" prefix-icon="Lock" show-password />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" native-type="submit" class="submit-btn" :loading="loading" style="width: 100%">
          登 录
        </el-button>
      </el-form-item>
    </el-form>

    <div class="login-footer">
      还没有账号？ <router-link to="/register" class="link">立即注册</router-link>
      <span class="footer-divider">·</span>
      <router-link to="/forgot" class="link">忘记密码？</router-link>
    </div>
  </AuthShell>
</template>

<script setup>
import AuthShell from '../components/AuthShell.vue'
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { ElMessage } from 'element-plus'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref(null)
const loading = ref(false)

const form = reactive({
  username: '',
  password: '',
})

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function handleLogin() {
  if (loading.value) return
  loading.value = true
  try {
    await formRef.value.validate()
    await userStore.login(form.username, form.password)
    ElMessage.success('登录成功')
    router.push('/')
  } catch (e) {
    // error handled by interceptor
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-title {
  text-align: center;
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -0.03em;
  margin-bottom: 28px;
  color: var(--text-primary);
}

.login-title .accent {
  color: var(--brand-inter);
}

.login-title .accent-blue {
  color: var(--brand-linked);
}

.login-subtitle {
  text-align: center;
  color: var(--text-tertiary);
  margin-bottom: 32px;
  font-size: 14px;
}

.submit-btn {
  border-radius: var(--r-pill);
  font-weight: 600;
  letter-spacing: 0.2em;
}

.login-footer {
  text-align: center;
  color: var(--text-tertiary);
  font-size: 14px;
}

.footer-divider {
  margin: 0 8px;
  color: var(--text-tertiary);
}

.link {
  color: var(--text-primary);
  font-weight: 600;
}

.link:hover {
  color: var(--accent);
  text-decoration: underline;
  text-underline-offset: 3px;
}
</style>
