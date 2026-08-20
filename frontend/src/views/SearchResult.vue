<template>
  <div class="search-page">
    <div class="card search-head">
      <div class="keyword">“{{ keyword }}”</div>
      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <el-tab-pane label="用户" name="users" />
        <el-tab-pane label="动态" name="feeds" />
      </el-tabs>
    </div>

    <div v-if="activeTab === 'users'" class="card">
      <div v-if="loading" class="text-center">加载中...</div>
      <div v-else-if="users.length === 0" class="text-center">暂无相关用户</div>
      <div v-else class="user-list">
        <div class="user-item" v-for="u in users" :key="u.id" @click="goProfile(u.id)">
          <el-avatar :size="48" :src="u.avatar || ''">{{ u.nickname?.charAt(0) || 'U' }}</el-avatar>
          <div class="meta">
            <div class="name">{{ u.nickname }}</div>
            <div class="desc">@{{ u.username }}</div>
          </div>
        </div>
      </div>
    </div>

    <div v-else>
      <div v-if="loading" class="card text-center">加载中...</div>
      <div v-else-if="feeds.length === 0" class="card text-center">暂无相关动态</div>
      <template v-else>
        <div class="card" v-for="feed in feeds" :key="feed.id">
          <FeedCard
            :feed="feed"
            :can-delete-feed="false"
            @like="handleLike"
            @unlike="handleUnlike"
            @click-author="goProfile"
            @click-feed="goToFeedDetail"
          />
        </div>
      </template>
    </div>

    <div class="text-center mt-20" v-if="hasMore">
      <el-button :loading="loadingMore" @click="loadMore">加载更多</el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { feedApi, userApi } from '../api'
import FeedCard from '../components/FeedCard.vue'

const route = useRoute()
const router = useRouter()

const keyword = ref('')
const activeTab = ref('users')
const loading = ref(false)
const loadingMore = ref(false)
const page = ref(1)
const pageSize = 20
const hasMore = ref(false)

const users = ref([])
const feeds = ref([])

onMounted(() => {
  syncQuery()
  search(true)
})

watch(() => route.query, () => {
  syncQuery()
  search(true)
})

function syncQuery() {
  keyword.value = String(route.query.keyword || '').trim()
  activeTab.value = route.query.type === 'feeds' ? 'feeds' : 'users'
}

function onTabChange() {
  router.replace({ path: '/search_result', query: { keyword: keyword.value, type: activeTab.value } })
}

async function search(reset = false) {
  if (!keyword.value) return
  if (reset) {
    page.value = 1
    users.value = []
    feeds.value = []
  }

  loading.value = reset
  loadingMore.value = !reset
  try {
    if (activeTab.value === 'users') {
      const res = await userApi.searchUsers(keyword.value, page.value, pageSize)
      const list = res.data.list || []
      users.value = page.value === 1 ? list : [...users.value, ...list]
      hasMore.value = !!res.data.has_more
    } else {
      const res = await feedApi.searchFeeds(keyword.value, page.value, pageSize)
      const list = res.data.list || []
      feeds.value = page.value === 1 ? list : [...feeds.value, ...list]
      hasMore.value = !!res.data.has_more
    }
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

async function loadMore() {
  if (!hasMore.value) return
  page.value += 1
  await search(false)
}

function goProfile(userId) {
  router.push(`/profile/${userId}`)
}

async function handleLike(feedId) {
  await feedApi.like(feedId)
  const item = feeds.value.find((f) => f.id === feedId)
  if (item) {
    item.is_liked = true
    item.like_count = (item.like_count || 0) + 1
  }
}

async function handleUnlike(feedId) {
  await feedApi.unlike(feedId)
  const item = feeds.value.find((f) => f.id === feedId)
  if (item) {
    item.is_liked = false
    item.like_count = Math.max((item.like_count || 0) - 1, 0)
  }
}

function goToFeedDetail(feedId) {
  if (!feedId) return
  router.push(`/feed/${feedId}`)
}
</script>

<style scoped>
.search-page {
  min-width: 0;
}
.search-head {
  border-radius: 14px;
  margin-bottom: 12px;
}
.keyword {
  font-size: 20px;
  font-weight: 700;
  margin-bottom: 8px;
}
.user-list {
  display: grid;
  gap: 10px;
}
.user-item {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 8px;
  border-radius: 10px;
  cursor: pointer;
}
.user-item:hover {
  background: #f6f8fb;
}
.name {
  font-size: 15px;
  font-weight: 600;
}
.desc {
  color: #8a93a5;
  font-size: 12px;
}
</style>
