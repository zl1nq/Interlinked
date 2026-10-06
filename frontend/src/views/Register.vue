<template>
  <AuthShell>
    <h1 class="login-title"><span class="accent">Inter</span><span class="accent-blue">Linked</span></h1>
    <p class="login-subtitle">{{ step === 1 ? '1 / 2 · 验证邮箱' : '2 / 2 · 完善账号资料' }}</p>

    <el-form v-if="step === 1" ref="emailFormRef" :model="form" :rules="rules" label-position="top" size="large" @submit.prevent="handleVerify">
      <el-form-item label="邮箱" prop="email">
        <el-input v-model="form.email" :disabled="busy" placeholder="请输入邮箱" prefix-icon="Message" autocomplete="email" @input="resetCode" />
      </el-form-item>
      <el-form-item label="邮箱验证码">
        <el-input ref="codeRef" v-model="code" :disabled="busy" placeholder="6 位数字验证码" prefix-icon="Key" maxlength="6" inputmode="numeric" autocomplete="one-time-code" />
      </el-form-item>
      <div class="step2-actions">
        <el-button text :disabled="busy || countdown > 0" :loading="sending" @click="sendCode">
          {{ countdown > 0 ? countdown + 's 后可重新发送' : (sentEmail ? '重新发送验证码' : '获取验证码') }}
        </el-button>
      </div>
      <p v-if="sentEmail" class="code-hint">验证码已发送，请查收邮箱。</p>
      <el-form-item>
        <el-button type="primary" native-type="submit" class="submit-btn" :disabled="sending" :loading="verifying" style="width: 100%">验证邮箱，下一步</el-button>
      </el-form-item>
    </el-form>

    <template v-else>
      <p class="code-hint">邮箱已验证：<span class="code-email">{{ verifiedEmail }}</span></p>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" size="large" @submit.prevent="handleRegister">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="用户名 (3-50个字符)" prefix-icon="User" autocomplete="username" />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="form.nickname" placeholder="昵称" prefix-icon="UserFilled" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" placeholder="密码 (至少6位)" prefix-icon="Lock" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input v-model="form.confirmPassword" type="password" placeholder="再次输入密码" prefix-icon="Lock" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" native-type="submit" class="submit-btn" :loading="loading" style="width: 100%">完成注册</el-button>
        </el-form-item>
      </el-form>
      <div class="step2-actions"><el-button text :disabled="loading" @click="backToEmail">更换邮箱，重新验证</el-button></div>
    </template>
    <div class="login-footer">已有账号？ <router-link to="/login" class="link">立即登录</router-link></div>
  </AuthShell>
</template>

<script setup>
import AuthShell from '../components/AuthShell.vue'
import { ref, reactive, computed, nextTick, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { authApi } from '../api'
import { useUserStore } from '../stores/user'
import { ElMessage } from 'element-plus'

const router = useRouter()
const userStore = useUserStore()
const emailFormRef = ref(null)
const formRef = ref(null)
const codeRef = ref(null)
const step = ref(1)
const loading = ref(false)
const verifying = ref(false)
const sending = ref(false)
const busy = computed(() => loading.value || verifying.value || sending.value)
const form = reactive({ email: '', username: '', nickname: '', password: '', confirmPassword: '' })
const code = ref('')
const sentEmail = ref('')
const verifiedEmail = ref('')
const registrationToken = ref('')
const expiresAt = ref(0)
const countdown = ref(0)
let timer
const normalizeEmail = () => form.email.trim().toLowerCase()
const rules = {
  email: [{ required: true, message: '请输入邮箱', trigger: 'blur' }, { type: 'email', message: '邮箱格式不正确', trigger: 'blur' }],
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }, { min: 3, max: 50, message: '用户名长度为3-50个字符', trigger: 'blur' }],
  nickname: [{ required: true, message: '请输入昵称', trigger: 'blur' }, { max: 100, message: '昵称不能超过100个字符', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }, { min: 6, max: 50, message: '密码长度为6-50位', trigger: 'blur' }],
  confirmPassword: [{ required: true, message: '请确认密码', trigger: 'blur' }, {
    validator: (_, value, callback) => callback(value === form.password ? undefined : new Error('两次输入的密码不一致')), trigger: 'blur',
  }],
}
function startCountdown() {
  const end = Date.now() + 60000
  clearInterval(timer)
  countdown.value = 60
  timer = setInterval(() => {
    countdown.value = Math.max(0, Math.ceil((end - Date.now()) / 1000))
    if (!countdown.value) clearInterval(timer)
  }, 1000)
}
onUnmounted(() => clearInterval(timer))
function resetCode() { code.value = ''; sentEmail.value = '' }
function backToEmail() {
  step.value = 1
  registrationToken.value = ''
  verifiedEmail.value = ''
  expiresAt.value = 0
  code.value = ''
}
async function sendCode() {
  if (busy.value || countdown.value > 0) return
  sending.value = true
  try {
    form.email = normalizeEmail()
    await emailFormRef.value.validate()
    await authApi.sendEmailCode({ scene: 'register', email: form.email })
    sentEmail.value = form.email
    startCountdown()
    ElMessage.success('验证码已发送，请查收邮箱')
  } catch (e) {
    // 表单和请求拦截器展示错误。
  } finally {
    sending.value = false
    await nextTick()
    if (sentEmail.value) codeRef.value?.focus()
  }
}
async function handleVerify() {
  if (busy.value) return
  verifying.value = true
  try {
    form.email = normalizeEmail()
    await emailFormRef.value.validate()
    if (!/^\d{6}$/.test(code.value)) { ElMessage.warning('请输入 6 位数字验证码'); return }
    const res = await authApi.confirmRegister({ email: form.email, code: code.value })
    registrationToken.value = res.data.registration_token
    expiresAt.value = Date.now() + res.data.expires_in * 1000
    verifiedEmail.value = form.email
    step.value = 2
    ElMessage.success('邮箱验证成功，请完善账号资料')
  } catch (e) {
    // 验证失败时留在邮箱步骤。
  } finally { verifying.value = false }
}
async function handleRegister() {
  if (busy.value) return
  if (!registrationToken.value || Date.now() >= expiresAt.value) {
    backToEmail()
    ElMessage.warning('邮箱验证已过期，请重新验证')
    return
  }
  loading.value = true
  try {
    form.username = form.username.trim()
    form.nickname = form.nickname.trim()
    await formRef.value.validate()
    await userStore.register({ email: verifiedEmail.value, registration_token: registrationToken.value,
      username: form.username, nickname: form.nickname, password: form.password })
    registrationToken.value = ''
    ElMessage.success('注册成功')
    router.push('/')
  } catch (e) {
    if (e.response?.status === 410) backToEmail()
  } finally { loading.value = false }
}
</script>

<style scoped>
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
  overflow-wrap: anywhere;
  color: var(--text-primary);
  font-weight: 600;
}

.step2-actions {
  margin-bottom: 16px;
  margin-top: 6px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
