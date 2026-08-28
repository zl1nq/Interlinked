<template>
  <div class="login-container">
    <div class="login-card">
      <h1 class="login-title"><span class="accent">Inter</span><span class="accent-blue">Linked</span></h1>
      <p class="login-subtitle">创建你的账号</p>

      <el-form ref="formRef" :model="form" :rules="rules" label-width="0" size="large">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="用户名 (3-50个字符)" prefix-icon="User" />
        </el-form-item>
        <el-form-item prop="nickname">
          <el-input v-model="form.nickname" placeholder="昵称" prefix-icon="UserFilled" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="密码 (至少6位)" prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item prop="confirmPassword">
          <el-input v-model="form.confirmPassword" type="password" placeholder="确认密码" prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" class="submit-btn" :loading="loading" style="width: 100%" @click="handleRegister">
            注 册
          </el-button>
        </el-form-item>
      </el-form>

      <div class="login-footer">
        已有账号？ <router-link to="/login" class="link">立即登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
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
  nickname: '',
  password: '',
  confirmPassword: '',
})

const validateConfirmPassword = (rule, value, callback) => {
  if (value !== form.password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 50, message: '用户名长度为3-50个字符', trigger: 'blur' },
  ],
  nickname: [
    { required: true, message: '请输入昵称', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' },
  ],
}

async function handleRegister() {
  try {
    await formRef.value.validate()
    loading.value = true
    await userStore.register(form.username, form.password, form.nickname)
    ElMessage.success('注册成功')
    router.push('/')
  } catch (e) {
    // error handled by interceptor
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface-base);
  padding: 24px;
  position: relative;
  overflow: hidden;
}

/* 品牌双色光晕：Inter 红 / Linked 蓝，绕页面中心逆时针公转 */
.login-container::before,
.login-container::after {
  content: '';
  position: absolute;
  width: 50vmax;
  height: 50vmax;
  border-radius: 50%;
  filter: blur(28px);
  pointer-events: none;
}

.login-container::before {
  top: 50%;
  left: 50%;
  background: radial-gradient(circle, rgba(244, 63, 94, 0.5), transparent 55%);
  animation: aurora-orbit-a 40s linear infinite;
}

.login-container::after {
  top: 50%;
  left: 50%;
  background: radial-gradient(circle, rgba(37, 99, 235, 0.48), transparent 55%);
  animation: aurora-orbit-b 40s linear infinite;
}

/* 双光晕绕页面中心逆时针公转：rotate 定轨道角度，translateX 定轨道半径；
   两团相差 180° 相位，公转中各自缓慢胀缩 */
@keyframes aurora-orbit-a {
  from {
    transform: translate(-50%, -50%) rotate(0deg) translateX(30vmin) scale(1);
  }
  50% {
    transform: translate(-50%, -50%) rotate(-180deg) translateX(30vmin) scale(1.15);
  }
  to {
    transform: translate(-50%, -50%) rotate(-360deg) translateX(30vmin) scale(1);
  }
}

@keyframes aurora-orbit-b {
  from {
    transform: translate(-50%, -50%) rotate(180deg) translateX(30vmin) scale(1.1);
  }
  50% {
    transform: translate(-50%, -50%) rotate(0deg) translateX(30vmin) scale(0.92);
  }
  to {
    transform: translate(-50%, -50%) rotate(-180deg) translateX(30vmin) scale(1.1);
  }
}

:global(body.dark) .login-container::before,
:global(body.dark) .login-container::after {
  opacity: 0.6;
}

@media (prefers-reduced-motion: reduce) {
  .login-container::before,
  .login-container::after {
    animation: none;
  }
}

.login-card {
  position: relative;
  z-index: 1;
  width: 400px;
  max-width: 100%;
  padding: 44px 40px 36px;
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-xl);
  box-shadow: var(--shadow-3);
  animation: page-enter 0.55s var(--ease) both;
}

.login-title {
  text-align: center;
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -0.03em;
  margin-bottom: 8px;
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