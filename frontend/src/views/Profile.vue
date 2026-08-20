<template>
  <div class="profile-page" v-if="user">
    <section class="profile-hero">
      <div class="hero-main card">
        <div class="hero-top">
          <el-avatar :size="170" :src="user.avatar || ''" class="hero-avatar">
            {{ user.nickname?.charAt(0) || 'U' }}
          </el-avatar>

          <div class="hero-right">
            <div class="hero-user-meta">
              <div class="hero-name-line">
                <h1 class="nickname-row">
                  <span class="nickname">{{ user.nickname }}</span>
                  <el-tag v-if="user.is_big_v" size="mid" effect="dark" type="warning">认证</el-tag>
                </h1>
                <el-button v-if="isMe" size="mid" plain @click="openProfileEditDialog">编辑信息</el-button>
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

    <section class="notes-section card mt-20">
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
  </div>

  <div v-else class="text-center mt-20">
    <el-skeleton :rows="5" animated />
  </div>
</template>

<script setup>
import { ref, onMounted, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { userApi, followApi, feedApi, uploadApi } from '../api'
import { ElMessage } from 'element-plus'
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

onMounted(() => {
  loadProfile()
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
  padding-bottom: 20px;
}

.profile-hero {
  position: relative;
  margin-top: 0;
}

.hero-main {
  border-radius: 18px;
  padding: 20px 20px 16px;
  border: 1px solid #eceef3;
  box-shadow: 0 8px 24px rgba(23, 29, 45, 0.06);
}

.hero-top {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.hero-avatar {
  flex: 0 0 auto;
  border: 2px solid #ffffff;
  box-shadow: 0 8px 20px rgba(17, 24, 39, 0.12);
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

.nickname-row {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.nickname {
  font-size: 24px;
  font-weight: 700;
  color: #1f2329;
}

.username {
  margin: 6px 0 0;
  color: #8c939f;
  font-size: 13px;
}

.bio {
  margin: 10px 0 6px;
  font-size: 14px;
  color: #38404d;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

.bio-empty {
  color: #9ba3b1;
}

.hero-stats {
  margin-top: 14px;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-start;
  gap: 8px;
}

.hero-stats-three {
  justify-content: flex-start;
}

.stat-item {
  width: 86px;
  text-align: center;
  cursor: pointer;
  padding: 10px 8px 9px;
  border-radius: 12px;
  background: #f7f8fa;
  border: 1px solid #f1f2f5;
  transition: all 0.2s ease;
}

.stat-item:hover {
  transform: translateY(-1px);
  background: #fff;
  border-color: #d9deea;
  box-shadow: 0 6px 14px rgba(31, 41, 55, 0.08);
}

.stat-item:hover .stat-value {
  color: #111827;
}

.stat-value {
  display: block;
  font-size: 16px;
  line-height: 1.15;
  font-weight: 700;
  color: #1f2329;
}

.stat-label {
  display: block;
  margin-top: 3px;
  font-size: 11px;
  color: #8c939f;
}

.hero-actions {
  margin-top: 14px;
  display: flex;
  gap: 10px;
}

.follow-btn,
.chat-btn {
  min-width: 106px;
}

.notes-section {
  border-radius: 16px;
  border: 1px solid #eceef3;
  box-shadow: 0 6px 20px rgba(23, 29, 45, 0.05);
}

.notes-header {
  padding-bottom: 12px;
  border-bottom: 1px solid #f2f3f5;
}

.notes-title {
  font-size: 16px;
  font-weight: 700;
  color: #1f2329;
}

.notes-list {
  padding-top: 4px;
}

.notes-loading,
.notes-empty {
  padding: 12px 0;
}

.edit-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.edit-avatar-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 4px;
}

.form-label {
  font-size: 13px;
  color: #6b7280;
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
  padding: 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
}

.user-list-item:hover {
  background: #f5f7fa;
}

:deep(.el-dialog) {
  border-radius: 14px;
}

.user-list-name {
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 6px;
}

.user-list-username {
  font-size: 12px;
  color: #999;
}

.user-list-visit-time {
  font-size: 12px;
  color: #b0b6c3;
  margin-top: 2px;
}

@media (max-width: 768px) {
  .hero-main {
    border-radius: 16px;
    padding: 16px 14px 14px;
  }

  .hero-top {
    gap: 12px;
  }

  .hero-avatar {
    transform: none;
  }

  .hero-name-line {
    align-items: flex-start;
    flex-direction: column;
    gap: 6px;
  }

  .nickname {
    font-size: 21px;
  }

  .hero-stats {
    gap: 6px;
  }

  .hero-stats-three {
    justify-content: flex-start;
  }

  .stat-item {
    width: 78px;
    padding: 9px 6px 8px;
    border-radius: 10px;
  }

  .stat-value {
    font-size: 15px;
  }
}

</style>
