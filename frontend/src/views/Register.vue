<template>
  <div class="login-container">
    <div class="login-card">
      <h1 class="login-title"><span class="accent">Inter</span><span class="accent-blue">Linked</span></h1>
      <p class="login-subtitle">{{ step === 1 ? '创建你的账号' : '验证邮箱' }}</p>

      <template v-if="step === 1">
        <el-form ref="formRef" :model="form" :rules="rules" label-position="top" size="large" @submit.prevent="handleRegister">
          <el-form-item label="用户名" prop="username">
            <el-input v-model="form.username" placeholder="用户名 (3-50个字符)" prefix-icon="User" />
          </el-form-item>
          <el-form-item label="昵称" prop="nickname">
            <el-input v-model="form.nickname" placeholder="昵称" prefix-icon="UserFilled" />
          </el-form-item>
          <el-form-item label="邮箱" prop="email">
            <el-input v-model="form.email" placeholder="邮箱" prefix-icon="Message" />
          </el-form-item>
          <el-form-item label="密码" prop="password">
            <el-input v-model="form.password" type="password" placeholder="密码 (至少6位)" prefix-icon="Lock" show-password />
          </el-form-item>
          <el-form-item label="确认密码" prop="confirmPassword">
            <el-input v-model="form.confirmPassword" type="password" placeholder="确认密码" prefix-icon="Lock" show-password />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" native-type="submit" class="submit-btn" :loading="loading" style="width: 100%">
              下一步：验证邮箱
            </el-button>
          </el-form-item>
        </el-form>

        <div class="login-footer">
          已有账号？ <router-link to="/login" class="link">立即登录</router-link>
        </div>
      </template>

      <template v-else>
        <p class="code-hint">
          验证码已发送至 <span class="code-email">{{ form.email }}</span>，10 分钟内有效
        </p>

        <el-form label-position="top" size="large" @submit.prevent="handleConfirm">
          <el-form-item label="邮箱验证码">
            <el-input
              v-model="code"
              placeholder="6 位数字验证码"
              prefix-icon="Key"
              maxlength="6"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" native-type="submit" class="submit-btn" :loading="confirming" style="width: 100%">
              完成注册
            </el-button>
          </el-form-item>
        </el-form>

        <div class="step2-actions">
          <el-button text :disabled="countdown > 0" :loading="resending" @click="resendCode">
            {{ countdown > 0 ? `${countdown}s 后可重新发送` : '重新发送验证码' }}
          </el-button>
          <el-button text @click="backToStep1">返回修改资料</el-button>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '../api'
import { useUserStore } from '../stores/user'
import { ElMessage } from 'element-plus'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref(null)
const loading = ref(false)
const step = ref(1)

const form = reactive({
  username: '',
  nickname: '',
  email: '',
  password: '',
  confirmPassword: '',
})

const code = ref('')
const confirming = ref(false)
const resending = ref(false)
const countdown = ref(0)
let timer = null

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
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 50, message: '密码长度为6-50位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' },
  ],
}

function startCountdown(seconds = 60) {
  countdown.value = seconds
  clearInterval(timer)
  timer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) clearInterval(timer)
  }, 1000)
}

onUnmounted(() => {
  clearInterval(timer)
})

// 第一步：提交资料，服务端暂存并发送验证码（不创建账号）
async function handleRegister() {
  if (loading.value) return
  loading.value = true
  try {
    await formRef.value.validate()
    await authApi.register({
      username: form.username.trim(),
      nickname: form.nickname.trim(),
      email: form.email.trim(),
      password: form.password,
    })
    ElMessage.success('验证码已发送，请查收邮箱')
    step.value = 2
    startCountdown()
  } catch (e) {
    // error handled by interceptor
  } finally {
    loading.value = false
  }
}

// 第二步：验证码确认建号，成功即自动登录
async function handleConfirm() {
  if (confirming.value) return
  if (!/^\d{6}$/.test(code.value)) {
    ElMessage.warning('请输入 6 位数字验证码')
    return
  }

  confirming.value = true
  try {
    await userStore.registerConfirm(form.email.trim(), code.value)
    ElMessage.success('注册成功')
    router.push('/')
  } catch (e) {
    // 验证码错误不影响暂存资料，停留在当前步骤
  } finally {
    confirming.value = false
  }
}

async function resendCode() {
  resending.value = true
  try {
    await authApi.sendEmailCode({ scene: 'register', email: form.email.trim() })
    ElMessage.success('验证码已重新发送')
    startCountdown()
  } catch (e) {
    // error handled by interceptor
  } finally {
    resending.value = false
  }
}

function backToStep1() {
  step.value = 1
  code.value = ''
  clearInterval(timer)
  countdown.value = 0
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

.code-hint {
  margin-bottom: 20px;
  text-align: center;
  color: var(--text-secondary);
  font-size: 14px;
  line-height: 1.7;
}

.code-email {
  color: var(--text-primary);
  font-weight: 600;
}

.step2-actions {
  margin-top: 6px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
