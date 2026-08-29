<template>
  <div class="profile-page" v-if="user">
    <section class="profile-hero">
      <div class="hero-main card">
        <!-- 窄屏账号管理入口：桌面隐藏，桌面由侧导航下拉承担 -->
        <button v-if="isMe" class="hero-gear-btn" type="button" aria-label="账号管理" @click="settingsSheetVisible = true">
          <el-icon><Setting /></el-icon>
        </button>

        <div class="hero-top">
          <el-avatar :size="isNarrowScreen ? 110 : 170" :src="user.avatar || ''" class="hero-avatar">
            {{ user.nickname?.charAt(0) || 'U' }}
          </el-avatar>

          <div class="hero-right">
            <div class="hero-user-meta">
              <div class="hero-name-line">
                <h1 class="nickname-row">
                  <span class="nickname">{{ user.nickname }}</span>
                  <el-tag v-if="user.is_big_v" effect="dark" type="warning" round>认证</el-tag>
                </h1>
                <div class="hero-name-actions" v-if="isMe">
                  <el-button plain round @click="openProfileEditDialog">编辑信息</el-button>
                  <el-button class="desktop-action" plain round @click="openPasswordDialog">修改密码</el-button>
                  <el-button class="desktop-action" plain round @click="openEmailDialog">{{ emailMode === 'change' ? '更换邮箱' : '绑定邮箱' }}</el-button>
                </div>
              </div>
              <p class="username">Feed号：{{ user.username }}</p>
            </div>
            <p class="bio" v-if="user.bio">{{ user.bio }}</p>
            <p class="bio bio-empty" v-else>还没有简介，快来认识一下 TA 吧</p>

            <div class="hero-stats" :class="{ 'hero-stats-three': !isMe }">
              <div class="stat-item">
                <span class="stat-value">{{ feedTotal }}</span>
                <span class="stat-label">笔记</span>
              </div>
              <div class="stat-item" @click="showFollowers">
                <span class="stat-value">{{ user.follower_count }}</span>
                <span class="stat-label">粉丝</span>
              </div>
              <div class="stat-item" @click="showFollowing">
                <span class="stat-value">{{ user.follow_count }}</span>
                <span class="stat-label">关注</span>
              </div>
              <div class="stat-item" v-if="isMe" @click="showVisits">
                <span class="stat-value">{{ visitTotal }}</span>
                <span class="stat-label">访客</span>
              </div>
            </div>
          </div>
        </div>

        
        <div class="hero-actions" v-if="!isMe">
          <el-button
            class="follow-btn"
            :type="user.is_followed ? 'default' : 'primary'"
            :loading="followLoading"
            round
            @click="handleFollowToggle"
          >
            {{ user.is_followed ? '已关注' : '关注' }}
          </el-button>
          <el-button class="chat-btn" round @click="goToChat">私信</el-button>
        </div>
      </div>
    </section>

    <section class="notes-section mt-20">
      <div class="notes-header">
        <span class="notes-title">TA的笔记</span>
      </div>

      <div class="notes-list">
        <div v-if="feedLoading && feeds.length === 0" class="notes-loading">
          <el-skeleton :rows="4" animated />
        </div>

        <div v-else-if="feeds.length === 0" class="text-center notes-empty">
          <el-empty description="暂无笔记" />
        </div>

        <div v-else>
          <FeedCard
            v-for="feed in feeds"
            :key="feed.id"
            :feed="feed"
            :can-delete-feed="isMe"
            @like="handleLike"
            @unlike="handleUnlike"
            @delete="handleDelete"
            @click-author="goToProfile"
            @click-feed="goToFeedDetail"
            @repost-success="loadFeeds"
          />

          <div class="load-more text-center mt-20">
            <el-button v-if="feedHasMore" :loading="feedLoadingMore" text @click="loadMoreFeeds">
              加载更多
            </el-button>
            <el-text v-else-if="feeds.length > 0" type="info">没有更多了</el-text>
          </div>
        </div>
      </div>
    </section>

    <!-- 窄屏账号管理：底部动作面板，避免被长帖子列表压到底部 -->
    <el-drawer
      v-model="settingsSheetVisible"
      direction="btt"
      size="320px"
      :with-header="false"
      append-to-body
      class="settings-sheet"
    >
      <div class="sheet-wrap">
        <div class="sheet-title">账号管理</div>
        <button class="sheet-row" type="button" @click="openSheetItem(openPasswordDialog)">
          <el-icon><Lock /></el-icon>
          <span>修改密码</span>
        </button>
        <button class="sheet-row" type="button" @click="openSheetItem(openEmailDialog)">
          <el-icon><Message /></el-icon>
          <span>{{ emailMode === 'change' ? '更换邮箱' : '绑定邮箱' }}</span>
        </button>
        <button class="sheet-row danger" type="button" @click="openSheetItem(confirmLogout)">
          <el-icon><SwitchButton /></el-icon>
          <span>退出登录</span>
        </button>
        <button class="sheet-row cancel" type="button" @click="settingsSheetVisible = false">取消</button>
      </div>
    </el-drawer>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="500px">
      <div v-if="dialogUsers.length === 0" class="text-center">
        <el-empty :description="dialogTitle + '列表为空'" />
      </div>
      <div v-else class="user-list">
        <div v-for="u in dialogUsers" :key="u.id" class="user-list-item" @click="goToProfile(u.id); dialogVisible = false;">
          <el-avatar :size="40" :src="u.avatar || ''">{{ u.nickname?.charAt(0) || 'U' }}</el-avatar>
          <div class="user-list-info">
            <div class="user-list-name">
              {{ u.nickname }}
              <el-tag v-if="u.is_big_v" size="small" type="warning" effect="plain">大V</el-tag>
            </div>
            <div class="user-list-username">@{{ u.username }}</div>
            <div v-if="dialogTitle === '最近访客' && u.visited_at" class="user-list-visit-time">访问于 {{ formatVisitTime(u.visited_at) }}</div>
          </div>
        </div>
      </div>
    </el-dialog>

    <el-dialog v-model="profileEditDialogVisible" title="编辑信息" width="560px">
      <div class="edit-form">
        <div class="edit-avatar-row">
          <el-avatar :size="72" :src="profileForm.avatar || user?.avatar || ''">
            {{ (profileForm.nickname || user?.nickname || 'U').charAt(0) }}
          </el-avatar>
          <el-upload
            class="avatar-uploader"
            :show-file-list="false"
            :auto-upload="false"
            accept="image/*"
            :on-change="handleProfileAvatarSelect"
          >
            <el-button plain :loading="avatarUploading">更换头像</el-button>
          </el-upload>
        </div>

        <div class="form-label">昵称</div>
        <el-input v-model="profileForm.nickname" maxlength="30" placeholder="请输入昵称" />

        <div class="form-label">个性签名</div>
        <el-input
          v-model="profileForm.bio"
          type="textarea"
          :rows="4"
          maxlength="500"
          show-word-limit
          placeholder="写点介绍自己的一句话吧..."
        />
      </div>

      <template #footer>
        <el-button @click="profileEditDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="profileSaving" @click="saveProfile">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="passwordDialogVisible" title="修改密码" width="480px" @closed="resetPasswordDialog">
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="0" size="large">
        <el-form-item prop="oldPassword">
          <el-input v-model="passwordForm.oldPassword" type="password" placeholder="当前密码" prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item prop="newPassword">
          <el-input v-model="passwordForm.newPassword" type="password" placeholder="新密码 (至少6位)" prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item prop="confirmPassword">
          <el-input v-model="passwordForm.confirmPassword" type="password" placeholder="确认新密码" prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item prop="code">
          <div class="code-input-row">
            <el-input v-model="passwordForm.code" placeholder="6 位数字验证码" prefix-icon="Key" maxlength="6" />
            <el-button class="code-send-btn" :disabled="pwdCountdown > 0" :loading="pwdCodeSending" @click="sendPasswordCode">
              {{ pwdCountdown > 0 ? `${pwdCountdown}s` : '发送验证码' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="passwordSubmitting" @click="handleChangePassword">确认修改</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="emailDialogVisible" :title="emailMode === 'change' ? '更换邮箱' : '绑定邮箱'" width="480px" @closed="resetEmailDialog">
      <div class="current-email-line">
        <template v-if="emailMode === 'change'">
          当前邮箱：<span class="current-email">{{ currentUserEmail }}</span>
        </template>
        <template v-else>当前账号尚未绑定邮箱，绑定后可用于找回密码</template>
      </div>

      <el-form ref="emailFormRef" :model="emailForm" :rules="emailFormRules" label-width="0" size="large">
        <el-form-item prop="newEmail">
          <el-input v-model="emailForm.newEmail" placeholder="新邮箱" prefix-icon="Message" />
        </el-form-item>
        <el-form-item v-if="emailMode === 'change'" prop="password">
          <el-input v-model="emailForm.password" type="password" placeholder="当前密码" prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item prop="code">
          <div class="code-input-row">
            <el-input v-model="emailForm.code" placeholder="6 位数字验证码" prefix-icon="Key" maxlength="6" />
            <el-button class="code-send-btn" :disabled="emailCodeCountdown > 0" :loading="emailCodeSending" @click="sendEmailChangeCode">
              {{ emailCodeCountdown > 0 ? `${emailCodeCountdown}s` : '发送验证码' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="emailDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="emailSubmitting" @click="handleUpdateEmail">
          {{ emailMode === 'change' ? '确认更换' : '确认绑定' }}
        </el-button>
      </template>
    </el-dialog>
  </div>

  <div v-else class="text-center mt-20">
    <el-skeleton :rows="5" animated />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { userApi, followApi, feedApi, uploadApi } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'
import FeedCard from '../components/FeedCard.vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const user = ref(null)
const feeds = ref([])
const feedLoading = ref(false)
const feedLoadingMore = ref(false)
const feedPage = ref(1)
const feedTotal = ref(0)
const feedHasMore = ref(false)
const followLoading = ref(false)
const avatarUploading = ref(false)
const profileEditDialogVisible = ref(false)
const profileSaving = ref(false)
const profileForm = ref({
  avatar: '',
  nickname: '',
  bio: '',
})

const dialogVisible = ref(false)
const dialogTitle = ref('')
const dialogUsers = ref([])
const visitTotal = ref(0)

const isMe = computed(() => userStore.userInfo?.id === user.value?.id)

// 窄屏形态：头像缩小、hero 齿轮入口（与底部 Tab 导航同一断点）
const narrowQuery = window.matchMedia('(max-width: 960px)')
const isNarrowScreen = ref(narrowQuery.matches)

function handleNarrowChange(event) {
  isNarrowScreen.value = event.matches
}

// 窄屏账号管理动作面板
const settingsSheetVisible = ref(false)

function openSheetItem(action) {
  settingsSheetVisible.value = false
  action()
}

onMounted(() => {
  narrowQuery.addEventListener('change', handleNarrowChange)
  loadProfile()
})

onUnmounted(() => {
  narrowQuery.removeEventListener('change', handleNarrowChange)
})

watch(() => route.params.id, () => {
  loadProfile()
})

async function loadProfile() {
  const userId = route.params.id
  try {
    const res = await userApi.getUserProfile(userId)
    user.value = res.data

    if (userStore.userInfo?.id === user.value?.id) {
      try {
        const visitRes = await userApi.getRecentVisits(1, 1)
        visitTotal.value = visitRes.data.total || 0
      } catch (e) {
        visitTotal.value = 0
      }
    } else {
      visitTotal.value = 0
    }

    loadFeeds()
  } catch (e) {}
}

async function loadFeeds() {
  feedLoading.value = true
  feedPage.value = 1
  try {
    const res = await feedApi.getUserFeeds(user.value.id, 1, 20)
    feeds.value = res.data.list || []
    feedTotal.value = res.data.total
    feedHasMore.value = res.data.has_more
  } catch (e) {}
  finally {
    feedLoading.value = false
  }
}

async function loadMoreFeeds() {
  feedLoadingMore.value = true
  feedPage.value++
  try {
    const res = await feedApi.getUserFeeds(user.value.id, feedPage.value, 20)
    feeds.value.push(...(res.data.list || []))
    feedHasMore.value = res.data.has_more
  } catch (e) {
    feedPage.value--
  } finally {
    feedLoadingMore.value = false
  }
}

async function handleFollowToggle() {
  followLoading.value = true
  try {
    if (user.value.is_followed) {
      await followApi.unfollow(user.value.id)
      user.value.is_followed = false
      user.value.follower_count = Math.max(0, user.value.follower_count - 1)
      ElMessage.success('已取消关注')
    } else {
      await followApi.follow(user.value.id)
      user.value.is_followed = true
      user.value.follower_count++
      ElMessage.success('关注成功')
    }
  } catch (e) {}
  finally {
    followLoading.value = false
  }
}

async function showFollowers() {
  dialogTitle.value = '粉丝'
  try {
    const res = await followApi.getFollowers(user.value.id, 1, 50)
    dialogUsers.value = res.data.list || []
    dialogVisible.value = true
  } catch (e) {}
}

async function showFollowing() {
  dialogTitle.value = '关注'
  try {
    const res = await followApi.getFollowing(user.value.id, 1, 50)
    dialogUsers.value = res.data.list || []
    dialogVisible.value = true
  } catch (e) {}
}

async function showVisits() {
  dialogTitle.value = '最近访客'
  try {
    const res = await userApi.getRecentVisits(1, 50)
    visitTotal.value = res.data.total || 0
    dialogUsers.value = (res.data.list || []).map(item => ({
      ...(item.visitor || {}),
      visited_at: item.visited_at,
    }))
    dialogVisible.value = true
  } catch (e) {}
}

async function handleLike(feedId) {
  try {
    await feedApi.like(feedId)
    const feed = feeds.value.find(f => f.id === feedId)
    if (feed) {
      feed.is_liked = true
      feed.like_count++
    }
  } catch (e) {}
}

async function handleUnlike(feedId) {
  try {
    await feedApi.unlike(feedId)
    const feed = feeds.value.find(f => f.id === feedId)
    if (feed) {
      feed.is_liked = false
      feed.like_count = Math.max(0, feed.like_count - 1)
    }
  } catch (e) {}
}

async function handleDelete(feedId) {
  try {
    await feedApi.deleteFeed(feedId)
    feeds.value = feeds.value.filter(f => f.id !== feedId)
    feedTotal.value = Math.max(0, feedTotal.value - 1)
    ElMessage.success('删除成功')
  } catch (e) {}
}

async function handleProfileAvatarSelect(uploadFile) {
  if (!isMe.value) return

  const rawFile = uploadFile?.raw
  if (!rawFile) return

  avatarUploading.value = true
  try {
    const uploadRes = await uploadApi.uploadImage(rawFile)
    const avatarUrl = uploadRes.data?.url || ''
    profileForm.value.avatar = avatarUrl
    ElMessage.success('头像上传成功')
  } catch (e) {
    // handled by interceptor
  } finally {
    avatarUploading.value = false
  }
}

function openProfileEditDialog() {
  profileForm.value = {
    avatar: user.value?.avatar || '',
    nickname: user.value?.nickname || '',
    bio: user.value?.bio || '',
  }
  profileEditDialogVisible.value = true
}

async function saveProfile() {
  profileSaving.value = true
  try {
    const payload = {
      avatar: profileForm.value.avatar,
      nickname: profileForm.value.nickname.trim(),
      bio: profileForm.value.bio.trim(),
    }

    const res = await userApi.updateProfile(payload)
    user.value = res.data

    if (userStore.userInfo?.id === user.value.id) {
      userStore.userInfo = { ...userStore.userInfo, ...res.data }
      sessionStorage.setItem('user', JSON.stringify(userStore.userInfo))
    }

    profileEditDialogVisible.value = false
    ElMessage.success('资料更新成功')
  } catch (e) {
    // handled by interceptor
  } finally {
    profileSaving.value = false
  }
}

function formatVisitTime(timeStr) {
  if (!timeStr) return ''
  const date = new Date(timeStr)
  if (Number.isNaN(date.getTime())) return ''

  const now = new Date()
  const diff = Math.floor((now - date) / 1000)
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
  return `${date.getMonth() + 1}月${date.getDate()}日 ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function goToProfile(userId) {
  router.push(`/profile/${userId}`)
}

function confirmLogout() {
  ElMessageBox.confirm('确定退出登录吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(() => {
    userStore.logout()
    router.push('/login')
  }).catch(() => {})
}

// ==================== 修改密码 ====================
const passwordDialogVisible = ref(false)
const passwordFormRef = ref(null)
const passwordSubmitting = ref(false)
const pwdCodeSending = ref(false)
const pwdCountdown = ref(0)
let pwdCodeTimer = null

const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
  code: '',
})

const validatePwdConfirm = (rule, value, callback) => {
  if (value !== passwordForm.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const validateNewPwdDiff = (rule, value, callback) => {
  if (passwordForm.oldPassword && value === passwordForm.oldPassword) {
    callback(new Error('新密码不能与当前密码相同'))
  } else {
    callback()
  }
}

const passwordRules = {
  oldPassword: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, max: 50, message: '密码长度为6-50位', trigger: 'blur' },
    { validator: validateNewPwdDiff, trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: validatePwdConfirm, trigger: 'blur' },
  ],
  code: [
    { required: true, message: '请输入验证码', trigger: 'blur' },
    { pattern: /^\d{6}$/, message: '验证码为 6 位数字', trigger: 'blur' },
  ],
}

function openPasswordDialog() {
  passwordForm.oldPassword = ''
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  passwordForm.code = ''
  passwordDialogVisible.value = true
}

function resetPasswordDialog() {
  passwordFormRef.value?.clearValidate()
}

function startPwdCountdown(seconds = 60) {
  pwdCountdown.value = seconds
  clearInterval(pwdCodeTimer)
  pwdCodeTimer = setInterval(() => {
    pwdCountdown.value -= 1
    if (pwdCountdown.value <= 0) clearInterval(pwdCodeTimer)
  }, 1000)
}

onUnmounted(() => {
  clearInterval(pwdCodeTimer)
  clearInterval(emailCodeTimer)
})

// 验证码发往当前账号已验证的邮箱，邮箱由服务端会话推导
async function sendPasswordCode() {
  pwdCodeSending.value = true
  try {
    await userApi.sendPasswordChangeCode()
    ElMessage.success('验证码已发送，请查收邮箱')
    startPwdCountdown()
  } catch (e) {
    // error handled by interceptor（未绑定邮箱等）
  } finally {
    pwdCodeSending.value = false
  }
}

// 修改成功后服务端使所有旧 token 失效，前端主动登出回登录页
async function handleChangePassword() {
  try {
    await passwordFormRef.value.validate()
    passwordSubmitting.value = true
    await userApi.updatePassword({
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword,
      code: passwordForm.code.trim(),
    })
    clearInterval(pwdCodeTimer)
    passwordDialogVisible.value = false
    ElMessage.success('密码修改成功，请重新登录')
    userStore.logout()
    router.push('/login')
  } catch (e) {
    // error handled by interceptor / validation
  } finally {
    passwordSubmitting.value = false
  }
}

// ==================== 绑定 / 更换邮箱 ====================
const emailDialogVisible = ref(false)
const emailFormRef = ref(null)
const emailSubmitting = ref(false)
const emailCodeSending = ref(false)
const emailCodeCountdown = ref(0)
let emailCodeTimer = null

// email_verified=true → 更换形态（需要当前密码）；否则为绑定形态（不提交 password）
const emailMode = computed(() => (userStore.userInfo?.email_verified ? 'change' : 'bind'))
const currentUserEmail = computed(() => userStore.userInfo?.email || '未绑定')

const emailForm = reactive({
  newEmail: '',
  password: '',
  code: '',
})

const validateNewEmailDiff = (rule, value, callback) => {
  if (emailMode.value === 'change' && userStore.userInfo?.email && value.trim() === userStore.userInfo.email) {
    callback(new Error('新邮箱与当前邮箱相同'))
  } else {
    callback()
  }
}

const emailFormRules = computed(() => ({
  newEmail: [
    { required: true, message: '请输入新邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' },
    { validator: validateNewEmailDiff, trigger: 'blur' },
  ],
  ...(emailMode.value === 'change'
    ? { password: [{ required: true, message: '请输入当前密码', trigger: 'blur' }] }
    : {}),
  code: [
    { required: true, message: '请输入验证码', trigger: 'blur' },
    { pattern: /^\d{6}$/, message: '验证码为 6 位数字', trigger: 'blur' },
  ],
}))

function openEmailDialog() {
  emailForm.newEmail = ''
  emailForm.password = ''
  emailForm.code = ''
  emailDialogVisible.value = true
}

function resetEmailDialog() {
  emailFormRef.value?.clearValidate()
}

function startEmailCountdown(seconds = 60) {
  emailCodeCountdown.value = seconds
  clearInterval(emailCodeTimer)
  emailCodeTimer = setInterval(() => {
    emailCodeCountdown.value -= 1
    if (emailCodeCountdown.value <= 0) clearInterval(emailCodeTimer)
  }, 1000)
}

// 密码在发码与确认两步都要提交，防 session 劫持后连邮箱一起换掉
function buildEmailPayload() {
  const payload = { new_email: emailForm.newEmail.trim() }
  if (emailMode.value === 'change') payload.password = emailForm.password
  return payload
}

async function sendEmailChangeCode() {
  try {
    const fields = emailMode.value === 'change' ? ['newEmail', 'password'] : ['newEmail']
    await emailFormRef.value.validateField(fields)
  } catch (e) {
    return
  }

  emailCodeSending.value = true
  try {
    await userApi.sendEmailChangeCode(buildEmailPayload())
    ElMessage.success('验证码已发送，请查收新邮箱')
    startEmailCountdown()
  } catch (e) {
    // error handled by interceptor（邮箱被占用 / 密码错误 / 限流等）
  } finally {
    emailCodeSending.value = false
  }
}

// 绑定/更换成功不吊销 token，本地直接用响应更新用户信息，无需重新登录
async function handleUpdateEmail() {
  try {
    await emailFormRef.value.validate()
    emailSubmitting.value = true
    const res = await userApi.updateEmail({ ...buildEmailPayload(), code: emailForm.code.trim() })

    if (res.data) {
      userStore.userInfo = { ...(userStore.userInfo || {}), ...res.data }
      sessionStorage.setItem('user', JSON.stringify(userStore.userInfo))
      if (user.value && Number(user.value.id) === Number(userStore.userInfo.id)) {
        user.value = {
          ...user.value,
          email: res.data.email,
          email_verified: res.data.email_verified,
        }
      }
    }

    clearInterval(emailCodeTimer)
    emailDialogVisible.value = false
    ElMessage.success(emailMode.value === 'change' ? '邮箱更换成功' : '邮箱绑定成功')
  } catch (e) {
    // error handled by interceptor / validation
  } finally {
    emailSubmitting.value = false
  }
}

function goToChat() {
  if (!user.value?.id) return
  router.push({ path: '/messages', query: { target: user.value.id, name: user.value.nickname || user.value.username || '私信' } })
}

function goToFeedDetail(feedId) {
  if (!feedId) return
  router.push(`/feed/${feedId}`)
}
</script>

<style scoped>
.profile-page {
  max-width: 720px;
  margin: 0 auto;
  padding-bottom: 20px;
}

.hero-main {
  padding: 28px 28px 24px;
}

.hero-top {
  display: flex;
  align-items: flex-start;
  gap: 20px;
}

.hero-avatar {
  flex: 0 0 auto;
  border-radius: 24px;
  border: 3px solid var(--surface-raised);
  box-shadow: var(--shadow-2);
  background: linear-gradient(135deg, #3a3a40 0%, #111113 100%);
  color: #fff;
  font-weight: 700;
}

.hero-right {
  min-width: 0;
  flex: 1;
}

.hero-user-meta {
  margin-top: 2px;
}

.hero-name-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.hero-name-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 0 0 auto;
}

.nickname-row {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 10px;
}

.nickname {
  font-size: 28px;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--text-primary);
}

.username {
  margin: 6px 0 0;
  color: var(--text-tertiary);
  font-size: 13px;
}

.bio {
  margin: 12px 0 6px;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.bio-empty {
  color: var(--text-tertiary);
}

.hero-stats {
  margin-top: 16px;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-start;
  gap: 10px;
}

.stat-item {
  min-width: 88px;
  text-align: center;
  cursor: pointer;
  padding: 12px 14px 10px;
  border-radius: var(--r-md);
  background: var(--surface-sunken);
  border: 1px solid transparent;
  transition: transform var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease),
    border-color var(--dur-fast) var(--ease), box-shadow var(--dur-fast) var(--ease);
}

.stat-item:hover {
  transform: translateY(-2px);
  background: var(--surface-raised);
  border-color: var(--border-subtle);
  box-shadow: var(--shadow-2);
}

.stat-value {
  display: block;
  font-size: 20px;
  line-height: 1.15;
  font-weight: 800;
  letter-spacing: -0.01em;
  color: var(--text-primary);
}

.stat-label {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.hero-actions {
  margin-top: 16px;
  display: flex;
  gap: 10px;
}

.follow-btn,
.chat-btn {
  min-width: 108px;
  font-weight: 600;
}

.notes-header {
  padding: 4px 4px 14px;
}

.notes-title {
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-primary);
}

.notes-loading,
.notes-empty {
  padding: 24px;
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-1);
}

.edit-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* 移动端账号管理齿轮：默认隐藏，仅 ≤960px 显示 */
.hero-gear-btn {
  display: none;
}

.hero-main {
  position: relative;
}

/* 桌面专属操作按钮：窄屏隐藏，管理项收进底部动作面板 */
@media (max-width: 960px) {
  .desktop-action {
    display: none;
  }

  .hero-gear-btn {
    display: grid;
    place-items: center;
    position: absolute;
    top: 16px;
    right: 14px;
    width: 36px;
    height: 36px;
    border: 1px solid var(--border-subtle);
    border-radius: var(--r-pill);
    background: var(--surface-sunken);
    color: var(--text-secondary);
    cursor: pointer;
    transition: background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease);
  }

  .hero-gear-btn:hover {
    background: var(--nav-hover-bg);
    color: var(--text-primary);
  }

  .hero-gear-btn .el-icon {
    font-size: 18px;
  }

  .hero-main {
    padding: 20px 18px;
  }

  .hero-top {
    gap: 14px;
  }

  .hero-name-line {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }

  .nickname {
    font-size: 22px;
  }

  .hero-stats {
    gap: 8px;
  }

  .stat-item {
    min-width: 70px;
    padding: 10px 10px 9px;
  }

  .stat-value {
    font-size: 17px;
  }
}

/* 账号管理底部动作面板 */
.sheet-wrap {
  padding: 6px 14px calc(16px + env(safe-area-inset-bottom, 0px));
}

.sheet-title {
  text-align: center;
  font-size: 13px;
  color: var(--text-tertiary);
  padding: 8px 0 6px;
}

.sheet-row {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 14px;
  border: 0;
  background: transparent;
  border-radius: var(--r-md);
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  cursor: pointer;
  transition: background var(--dur-fast) var(--ease);
}

.sheet-row:hover {
  background: var(--nav-hover-bg);
}

.sheet-row .el-icon {
  font-size: 17px;
  color: var(--text-secondary);
}

.sheet-row.danger {
  color: var(--el-color-danger);
}

.sheet-row.danger .el-icon {
  color: var(--el-color-danger);
}

.sheet-row.cancel {
  justify-content: center;
  margin-top: 4px;
  border-top: 1px solid var(--border-subtle);
  border-radius: 0;
  color: var(--text-secondary);
  font-weight: 500;
}

.code-input-row {
  width: 100%;
  display: flex;
  gap: 10px;
}

.code-send-btn {
  flex: 0 0 auto;
  min-width: 108px;
  border-radius: var(--r-md);
  font-weight: 600;
}

.current-email-line {
  margin-bottom: 14px;
  font-size: 13px;
  color: var(--text-tertiary);
  line-height: 1.6;
}

.current-email {
  color: var(--text-primary);
  font-weight: 600;
}

.edit-avatar-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 4px;
}

.form-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
  margin-top: 2px;
}

.user-list {
  max-height: 400px;
  overflow-y: auto;
}

.user-list-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: var(--r-md);
  cursor: pointer;
  transition: background var(--dur-fast) var(--ease);
}

.user-list-item:hover {
  background: var(--nav-hover-bg);
}

.user-list-item :deep(.el-avatar) {
  border-radius: 10px;
}

.user-list-name {
  font-weight: 600;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 6px;
}

.user-list-username {
  font-size: 12px;
  color: var(--text-tertiary);
}

.user-list-visit-time {
  font-size: 12px;
  color: var(--text-tertiary);
  margin-top: 2px;
}

@media (max-width: 480px) {
  .hero-stats {
    gap: 6px;
  }

  .stat-item {
    min-width: 0;
    flex: 1 1 40%;
    padding: 10px 6px 9px;
  }
}
</style>
