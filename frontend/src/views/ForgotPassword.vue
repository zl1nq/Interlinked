<template>
  <div class="login-container">
    <div class="login-card">
      <h1 class="login-title"><span class="accent">Inter</span><span class="accent-blue">Linked</span></h1>
      <p class="login-subtitle">{{ step === 1 ? '找回密码' : '重置密码' }}</p>

      <template v-if="step === 1">
        <el-form ref="emailFormRef" :model="emailForm" :rules="emailRules" label-width="0" size="large">
          <el-form-item prop="email">
            <el-input v-model="emailForm.email" placeholder="注册时绑定的邮箱" prefix-icon="Message" @keyup.enter="sendCode" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" class="submit-btn" :loading="sending" style="width: 100%" @click="sendCode">
              发送验证码
            </el-button>
          </el-form-item>
        </el-form>
      </template>

      <template v-else>
        <p class="code-hint">
          验证码已发送至 <span class="code-email">{{ emailForm.email }}</span>，10 分钟内有效
        </p>

        <el-form ref="resetFormRef" :model="resetForm" :rules="resetRules" label-width="0" size="large">
          <el-form-item prop="code">
            <el-input v-model="resetForm.code" placeholder="6 位数字验证码" prefix-icon="Key" maxlength="6" />
          </el-form-item>
          <el-form-item prop="newPassword">
            <el-input v-model="resetForm.newPassword" type="password" placeholder="新密码 (至少6位)" prefix-icon="Lock" show-password />
          </el-form-item>
          <el-form-item prop="confirmPassword">
            <el-input v-model="resetForm.confirmPassword" type="password" placeholder="确认新密码" prefix-icon="Lock" show-password />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" class="submit-btn" :loading="resetting" style="width: 100%" @click="handleReset">
              重置密码
            </el-button>
          </el-form-item>
        </el-form>

        <div class="step2-actions">
          <el-button text :disabled="countdown > 0" :loading="resending" @click="resendCode">
            {{ countdown > 0 ? `${countdown}s 后可重新发送` : '重新发送验证码' }}
          </el-button>
          <el-button text @click="backToEmail">更换邮箱</el-button>
        </div>
      </template>

      <div class="login-footer">
        想起密码了？ <router-link to="/login" class="link">返回登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '../api'
import { ElMessage } from 'element-plus'

const router = useRouter()
const step = ref(1)

const emailFormRef = ref(null)
const emailForm = reactive({ email: '' })
const emailRules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' },
  ],
}

const resetFormRef = ref(null)
const resetForm = reactive({
  code: '',
  newPassword: '',
  confirmPassword: '',
})

const validateConfirmPassword = (rule, value, callback) => {
  if (value !== resetForm.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const resetRules = {
  code: [
    { required: true, message: '请输入验证码', trigger: 'blur' },
    { pattern: /^\d{6}$/, message: '验证码为 6 位数字', trigger: 'blur' },
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, max: 50, message: '密码长度为6-50位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' },
  ],
}

const sending = ref(false)
const resetting = ref(false)
const resending = ref(false)
const countdown = ref(0)
let timer = null

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

// 防账号枚举：无论邮箱是否注册，接口都返回成功，前端不做存在性预判
async function sendCode() {
  try {
    await emailFormRef.value.validate()
    sending.value = true
    await authApi.sendEmailCode({ scene: 'forgot_password', email: emailForm.email.trim() })
    ElMessage.success('验证码已发送，请查收邮箱')
    step.value = 2
    startCountdown()
  } catch (e) {
    // error handled by interceptor / validation
  } finally {
    sending.value = false
  }
}

async function resendCode() {
  resending.value = true
  try {
    await authApi.sendEmailCode({ scene: 'forgot_password', email: emailForm.email.trim() })
    ElMessage.success('验证码已重新发送')
    startCountdown()
  } catch (e) {
    // error handled by interceptor
  } finally {
    resending.value = false
  }
}

async function handleReset() {
  try {
    await resetFormRef.value.validate()
    resetting.value = true
    await authApi.resetPassword({
      email: emailForm.email.trim(),
      code: resetForm.code.trim(),
      new_password: resetForm.newPassword,
    })
    ElMessage.success('密码重置成功，请使用新密码登录')
    router.push('/login')
  } catch (e) {
    // error handled by interceptor / validation
  } finally {
    resetting.value = false
  }
}

function backToEmail() {
  step.value = 1
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
  margin-top: 18px;
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
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
