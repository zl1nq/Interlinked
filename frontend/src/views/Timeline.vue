<template>
  <div class="timeline-page" :class="{ 'timeline-waterfall': !isNarrowScreen }">
    <div v-if="loading && feeds.length === 0" class="loading-wrap">
      <el-skeleton :rows="4" animated />
      <el-skeleton :rows="4" animated class="mt-20" />
    </div>

    <div v-else-if="feeds.length === 0" class="empty-wrap">
      <el-empty description="朋友圈还没有动态">
        <el-button type="primary" @click="router.push('/discover')">去发现朋友</el-button>
      </el-empty>
    </div>

    <template v-else>
      <!-- 窄屏：保持单列朋友圈式 FeedCard -->
      <section v-if="isNarrowScreen" class="moments-list">
        <FeedCard
          v-for="feed in feeds"
          :key="feed.id"
          :feed="feed"
          :can-delete-feed="false"
          @like="handleLike"
          @unlike="handleUnlike"
          @delete="handleDelete"
          @click-author="goToProfile"
          @click-feed="goToFeedDetail"
          @repost-success="loadTimeline"
        />
      </section>

      <!-- 桌面：双列瀑布流紧凑卡片 -->
      <section v-else class="waterfall">
        <FeedWaterfallCard
          v-for="feed in feeds"
          :key="feed.id"
          :feed="feed"
          @like="handleLike"
          @unlike="handleUnlike"
          @click-author="goToProfile"
          @click-feed="goToFeedDetail"
        />
      </section>

      <div class="load-more">
        <el-button v-if="hasMore" :loading="loadingMore" text @click="loadMore">加载更多</el-button>
        <span v-else>没有更多动态了</span>
      </div>
    </template>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { feedApi } from '../api'
import FeedCard from '../components/FeedCard.vue'
import FeedWaterfallCard from '../components/FeedWaterfallCard.vue'

const router = useRouter()

const feeds = ref([])
const loading = ref(false)
const loadingMore = ref(false)
// 游标分页状态：cursor 使用 "created_at_unix_Milli|feed_id" 格式，首次为空字符串。
const cursor = ref('')
const pageSize = 20
const hasMore = ref(true)

// 桌面双列瀑布流 / 窄屏单列切换；两套卡片不同时挂载，避免重复拉取评论数据
const narrowQuery = window.matchMedia('(max-width: 960px)')
const isNarrowScreen = ref(narrowQuery.matches)

function handleNarrowChange(event) {
  isNarrowScreen.value = event.matches
}

onMounted(() => {
  narrowQuery.addEventListener('change', handleNarrowChange)
  loadTimeline()
})

onUnmounted(() => {
  narrowQuery.removeEventListener('change', handleNarrowChange)
})

// loadTimeline 首次加载：从 cursor=0 拉取第一批。
async function loadTimeline() {
  loading.value = true
  try {
    const res = await feedApi.getTimeline('', pageSize)
    feeds.value = res.data.list || []
    hasMore.value = !!res.data.has_more
    cursor.value = res.data.next_cursor || ''
  } catch (e) {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

// loadMore 使用 next_cursor 继续拉取后续数据。
async function loadMore() {
  if (!hasMore.value || loadingMore.value) return
  loadingMore.value = true
  try {
    const res = await feedApi.getTimeline(cursor.value, pageSize)
    const list = res.data.list || []
    const existingIds = new Set(feeds.value.map((item) => item.id))
    const merged = list.filter((item) => !existingIds.has(item.id))
    feeds.value.push(...merged)
    hasMore.value = !!res.data.has_more
    cursor.value = res.data.next_cursor || cursor.value
  } catch (e) {
    // handled by interceptor
  } finally {
    loadingMore.value = false
  }
}


async function handleLike(feedId) {
  try {
    await feedApi.like(feedId)
    const feed = feeds.value.find((item) => item.id === feedId)
    if (feed) {
      feed.is_liked = true
      feed.like_count = (feed.like_count || 0) + 1
    }
  } catch (e) {
    // handled by interceptor
  }
}

async function handleUnlike(feedId) {
  try {
    await feedApi.unlike(feedId)
    const feed = feeds.value.find((item) => item.id === feedId)
    if (feed) {
      feed.is_liked = false
      feed.like_count = Math.max((feed.like_count || 0) - 1, 0)
    }
  } catch (e) {
    // handled by interceptor
  }
}

async function handleDelete(feedId) {
  try {
    await feedApi.deleteFeed(feedId)
    feeds.value = feeds.value.filter((item) => item.id !== feedId)
    if (feeds.value.length === 0) {
      cursor.value = ''
      hasMore.value = true
      await loadTimeline()
      return
    }
    ElMessage.success('删除成功')
  } catch (e) {
    // handled by interceptor
  }
}

function goToProfile(userId) {
  if (!userId) return
  router.push(`/profile/${userId}`)
}

function goToFeedDetail(feedId) {
  if (!feedId) return
  router.push(`/feed/${feedId}`)
}
</script>

<style scoped>
/* 窄屏单列：容器透明，FeedCard 各自以浮起卡片呈现 */
.timeline-page {
  max-width: 640px;
  margin: 0 auto;
}

/* 桌面双列瀑布流：加宽容器，CSS columns 让卡片按内容高度自然交错 */
.timeline-waterfall {
  max-width: 1080px;
}

.waterfall {
  column-count: 2;
  column-gap: 16px;
}

.loading-wrap,
.empty-wrap {
  padding: 24px;
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-1);
}

.load-more {
  text-align: center;
  color: var(--text-tertiary);
  font-size: 13px;
  padding: 8px 0 20px;
}

@media (max-width: 960px) {
  .waterfall {
    column-count: 1;
  }
}
</style>
