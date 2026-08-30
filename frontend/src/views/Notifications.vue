<template>
  <div class="notifications-xhs">
    <section class="head card">
      <div class="title-row">
        <h2>通知</h2>
        <div class="title-actions">
          <el-button text @click="markAllRead">全部已读</el-button>
          <el-button text :loading="clearing" @click="clearRead">清空已读</el-button>
        </div>
      </div>

      <!-- 小红书风格分类标签 -->
      <el-tabs v-model="activeTab" class="tabs">
        <el-tab-pane label="全部" name="all" />
        <el-tab-pane label="赞" name="like" />
        <el-tab-pane label="评论" name="comment" />
        <el-tab-pane label="新增关注" name="follow" />
      </el-tabs>
    </section>

    <section class="list-wrap">
      <div v-if="loading" class="card state">加载中...</div>
      <div v-else-if="filteredList.length === 0" class="card state">暂无通知</div>

      <article
        v-else
        v-for="n in filteredList"
        :key="n.id"
        class="notice-item card"
      >
        <div class="left">
          <el-avatar :size="44" :src="n.actor?.avatar || ''">{{ n.actor?.nickname?.charAt(0) || 'U' }}</el-avatar>
        </div>

        <div class="main">
          <div class="line1">
            <span class="name">{{ n.actor?.nickname || '用户' }}</span>
            <span class="text">{{ normalizedContent(n) }}</span>
          </div>
          <div class="line2">{{ formatTime(n.created_at) }}</div>
        </div>

        <div class="right">
          <div class="right-meta">
            <span v-if="!n.is_read" class="unread-dot" />
            <el-tag size="small" effect="plain" class="type-tag">{{ typeLabel(n.type) }}</el-tag>
          </div>
          <button class="notice-delete" type="button" aria-label="删除通知" @click="handleDelete(n)">
            <el-icon><Delete /></el-icon>
          </button>
        </div>
      </article>

      <div class="more" v-if="hasMore">
        <el-button :loading="loadingMore" @click="loadMore">加载更多</el-button>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { notificationApi } from '../api'
import { useNotificationStore } from '../stores/notification'
import { ElMessage } from 'element-plus'

const notificationStore = useNotificationStore()

const list = ref([])
const page = ref(1)
const pageSize = 20
const hasMore = ref(false)
const loading = ref(false)
const loadingMore = ref(false)
const clearing = ref(false)
const activeTab = ref('all')

const filteredList = computed(() => {
  if (activeTab.value === 'all') return list.value
  if (activeTab.value === 'like') return list.value.filter((n) => n.type === 'like')
  if (activeTab.value === 'comment') return list.value.filter((n) => n.type === 'comment' || n.type === 'reply')
  if (activeTab.value === 'follow') return list.value.filter((n) => n.type === 'follow')
  return list.value
})

onMounted(() => {
  loadNotifications(true)
})

async function loadNotifications(reset = false) {
  if (reset) {
    page.value = 1
    list.value = []
  }

  loading.value = reset
  loadingMore.value = !reset
  try {
    const res = await notificationApi.getNotifications(page.value, pageSize)
    const rows = res.data.list || []
    list.value = page.value === 1 ? rows : [...list.value, ...rows]
    hasMore.value = !!res.data.has_more
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

async function loadMore() {
  if (!hasMore.value) return
  page.value += 1
  await loadNotifications(false)
}

async function markAllRead() {
  await notificationApi.markAllRead()
  list.value = list.value.map((n) => ({ ...n, is_read: true }))
  notificationStore.setUnreadCount(0)
}

// 单条删除：本地移除并用响应的 unread_count 覆盖角标
async function handleDelete(n) {
  try {
    const res = await notificationApi.deleteNotification(n.id)
    list.value = list.value.filter((item) => item.id !== n.id)
    if (res.data?.unread_count !== undefined) {
      notificationStore.setUnreadCount(res.data.unread_count)
    }
    ElMessage.success('通知已删除')
  } catch (e) {
    // handled by interceptor
  }
}

// 清空已读：未读保留所以角标不变；已读条目移除后当前页可能出现空洞，重新拉第一页
async function clearRead() {
  clearing.value = true
  try {
    const res = await notificationApi.clearReadNotifications()
    const count = Number(res.data?.deleted_count) || 0
    if (count === 0) {
      ElMessage.info('暂无可清理的已读通知')
      return
    }
    ElMessage.success(`已清理 ${count} 条已读通知`)
    await loadNotifications(true)
  } catch (e) {
    // handled by interceptor
  } finally {
    clearing.value = false
  }
}

function normalizedContent(n) {
  if (n.content) return n.content
  if (n.type === 'like') return '赞了你的动态'
  if (n.type === 'comment') return '评论了你的动态'
  if (n.type === 'reply') return '回复了你的评论'
  if (n.type === 'follow') return '关注了你'
  return '与你产生了互动'
}

function typeLabel(type) {
  if (type === 'like') return '赞'
  if (type === 'comment') return '评论'
  if (type === 'reply') return '回复'
  if (type === 'follow') return '关注'
  return '通知'
}

function formatTime(timeStr) {
  if (!timeStr) return ''
  const d = new Date(timeStr)
  if (Number.isNaN(d.getTime())) return ''

  const now = new Date()
  const diff = Math.floor((now - d) / 1000)
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
  return `${d.getMonth() + 1}月${d.getDate()}日 ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}
</script>

<style scoped>
.notifications-xhs {
  min-width: 0;
  max-width: 720px;
  margin: 0 auto;
}

.head {
  margin-bottom: 16px;
}

.title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.title-row h2 {
  margin: 0;
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.02em;
}

.tabs {
  margin-top: 8px;
}

.list-wrap {
  display: grid;
  gap: 10px;
}

.state {
  text-align: center;
  color: var(--text-tertiary);
}

.notice-item {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 12px;
}

.notice-item :deep(.el-avatar) {
  border-radius: 12px;
}

.main {
  min-width: 0;
}

.line1 {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  line-height: 1.6;
}

.name {
  font-weight: 700;
  color: var(--text-primary);
}

.text {
  color: var(--text-secondary);
}

.line2 {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.right-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.unread-dot {
  width: 8px;
  height: 8px;
  border-radius: var(--r-pill);
  background: var(--accent);
}

.type-tag {
  border-radius: var(--r-pill);
}

.notice-delete {
  flex: 0 0 auto;
  width: 26px;
  height: 26px;
  border: 0;
  border-radius: var(--r-pill);
  background: transparent;
  color: var(--text-tertiary);
  cursor: pointer;
  display: grid;
  place-items: center;
  transition: color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease);
}

.notice-delete:hover {
  color: var(--el-color-danger);
  background: var(--accent-soft);
}

.notice-delete .el-icon {
  font-size: 14px;
}

.title-actions {
  display: inline-flex;
  align-items: center;
}

.more {
  text-align: center;
}
</style>
