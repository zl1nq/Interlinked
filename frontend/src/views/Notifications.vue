<template>
  <div class="notifications-xhs">
    <section class="head card">
      <div class="title-row">
        <h2>通知</h2>
        <el-button text @click="markAllRead">全部已读</el-button>
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
        @click="goTarget(n)"
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
          <span v-if="!n.is_read" class="unread-dot" />
          <el-tag size="small" effect="plain" class="type-tag">{{ typeLabel(n.type) }}</el-tag>
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
import { useRouter } from 'vue-router'
import { notificationApi } from '../api'

const router = useRouter()

const list = ref([])
const page = ref(1)
const pageSize = 20
const hasMore = ref(false)
const loading = ref(false)
const loadingMore = ref(false)
const activeTab = ref('all')

const filteredList = computed(() => {
  if (activeTab.value === 'all') return list.value
  if (activeTab.value === 'like') return list.value.filter((n) => n.type === 'like')
  if (activeTab.value === 'comment') return list.value.filter((n) => n.type === 'comment')
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
}

function goTarget(n) {
  if (n.type === 'follow') {
    router.push(`/profile/${n.actor?.id}`)
    return
  }
  if (n.target_id) {
    router.push('/timeline')
  }
}

function normalizedContent(n) {
  if (n.content) return n.content
  if (n.type === 'like') return '赞了你的动态'
  if (n.type === 'comment') return '评论了你的动态'
  if (n.type === 'follow') return '关注了你'
  return '与你产生了互动'
}

function typeLabel(type) {
  if (type === 'like') return '赞'
  if (type === 'comment') return '评论'
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
}

.head {
  border-radius: 14px;
  margin-bottom: 12px;
}

.title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.title-row h2 {
  margin: 0;
  font-size: 24px;
}

.tabs {
  margin-top: 8px;
}

.list-wrap {
  display: grid;
  gap: 10px;
}

.state {
  border-radius: 14px;
  text-align: center;
  color: #8d97aa;
}

.notice-item {
  border-radius: 14px;
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 10px;
  cursor: pointer;
}

.notice-item:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 18px rgba(27, 36, 58, 0.08);
}

.main {
  min-width: 0;
}

.line1 {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  line-height: 1.5;
}

.name {
  font-weight: 700;
}

.text {
  color: #3f4654;
}

.line2 {
  margin-top: 4px;
  font-size: 12px;
  color: #97a1b5;
}

.right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.unread-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #ff2e4d;
}

.type-tag {
  border-radius: 999px;
}

.more {
  text-align: center;
}
</style>
