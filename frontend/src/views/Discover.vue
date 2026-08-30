<template>
  <div class="discover-xhs">
    <!-- 移动端全局搜索（桌面端由顶栏承担） -->
    <div class="mobile-global-search">
      <el-icon class="ms-icon"><Search /></el-icon>
      <input
        v-model="globalKeyword"
        class="ms-input"
        type="text"
        placeholder="搜索用户、动态"
        @keyup.enter="goGlobalSearch"
      />
      <button class="ms-action" type="button" @click="goGlobalSearch">搜索</button>
    </div>

    <div class="discover-head card">
      <div class="head-title">发现</div>
      <div class="head-sub">探索热门创作者与热门动态</div>
      <el-input
        v-model="keyword"
        placeholder="搜索昵称 / 用户名"
        size="large"
        clearable
        @keyup.enter="searchUsers"
        @clear="clearSearch"
      >
        <template #append>
          <el-button :loading="searching" @click="searchUsers">搜索</el-button>
        </template>
      </el-input>
    </div>

    <!-- 搜索态：保留原有用户搜索结果 -->
    <template v-if="searched">
      <div v-if="cards.length === 0" class="card empty-wrap">
        <el-empty description="暂无相关用户" />
      </div>

      <div v-else class="waterfall">
        <article
          v-for="user in cards"
          :key="user.id"
          class="note-card"
          @click="goToProfile(user.id)"
        >
          <div class="note-cover" :style="coverStyle(user)">
            <el-avatar :size="64" :src="user.avatar || ''" class="note-avatar">
              {{ user.nickname?.charAt(0) || 'U' }}
            </el-avatar>
          </div>

          <div class="note-body">
            <div class="note-title">
              {{ user.nickname || user.username }}
              <el-tag v-if="user.is_big_v" size="small" type="warning" effect="plain">大V</el-tag>
            </div>
            <div class="note-desc">@{{ user.username }}</div>
            <div class="note-meta">
              <span>{{ user.follower_count || 0 }} 粉丝</span>
              <span>·</span>
              <span>{{ user.follow_count || 0 }} 关注</span>
            </div>
          </div>
        </article>
      </div>

      <div v-if="total > pageSize" class="pager-wrap">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          @current-change="searchUsers"
        />
      </div>
    </template>

    <!-- 推荐态：热门用户横滑条 + 热门帖子瀑布流 -->
    <template v-else>
      <section v-if="popularUsers.length" class="popular-users card">
        <div class="section-title">热门用户</div>
        <div class="user-strip">
          <div v-for="u in popularUsers" :key="u.id" class="user-chip" @click="goToProfile(u.id)">
            <el-avatar :size="52" :src="u.avatar || ''" class="chip-avatar">
              {{ u.nickname?.charAt(0) || 'U' }}
            </el-avatar>
            <div class="chip-name">
              <span class="chip-name-text">{{ u.nickname || u.username }}</span>
              <el-tag v-if="u.is_big_v" size="small" type="warning" effect="plain">V</el-tag>
            </div>
            <div class="chip-meta">{{ formatCount(u.follower_count) }} 粉丝</div>
          </div>
        </div>
      </section>

      <section class="popular-feeds">
        <div v-if="popularFeeds.length || feedLoading" class="section-title block">热门帖子</div>

        <div v-if="feedLoading && popularFeeds.length === 0" class="card empty-wrap">
          <el-skeleton :rows="4" animated />
        </div>
        <div v-else-if="!feedLoading && popularFeeds.length === 0 && feedsLoaded" class="card empty-wrap">
          <el-empty description="暂无热门动态" />
        </div>

        <template v-else>
          <div class="waterfall">
            <FeedWaterfallCard
              v-for="feed in popularFeeds"
              :key="feed.id"
              :feed="feed"
              @like="handleLike"
              @unlike="handleUnlike"
              @click-author="goToProfile"
              @click-feed="goToFeedDetail"
            />
          </div>

          <div class="load-more">
            <el-button v-if="feedHasMore" :loading="feedLoadingMore" text @click="loadMoreFeeds">加载更多</el-button>
            <span v-else>没有更多了</span>
          </div>
        </template>
      </section>
    </template>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { userApi, feedApi, discoverApi } from '../api'
import FeedWaterfallCard from '../components/FeedWaterfallCard.vue'

const router = useRouter()
const route = useRoute()

// ==================== 搜索态 ====================
const keyword = ref('')
const globalKeyword = ref('')
const cards = ref([])
const searching = ref(false)
const searched = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)

// ==================== 推荐态 ====================
const popularUsers = ref([])
const popularFeeds = ref([])
const feedLoading = ref(false)
const feedLoadingMore = ref(false)
const feedPage = ref(1)
const feedHasMore = ref(false)
const feedsLoaded = ref(false)

onMounted(() => {
  loadPopular()
  const q = String(route.query.keyword || '').trim()
  if (q) {
    keyword.value = q
    searchUsers()
  }
})

// 首屏并发拉取热门用户与热门帖子；任一失败仅留空对应区块（拦截器统一提示）
async function loadPopular() {
  feedLoading.value = true
  try {
    const [usersRes, feedsRes] = await Promise.all([
      discoverApi.getPopularUsers(1, 10),
      discoverApi.getPopularFeeds(1, 10),
    ])
    popularUsers.value = usersRes.data.list || []
    popularFeeds.value = feedsRes.data.list || []
    feedHasMore.value = !!feedsRes.data.has_more
  } catch (e) {
    // handled by interceptor
  } finally {
    feedLoading.value = false
    feedsLoaded.value = true
  }
}

async function loadMoreFeeds() {
  if (!feedHasMore.value || feedLoadingMore.value) return
  feedLoadingMore.value = true
  try {
    const nextPage = feedPage.value + 1
    const res = await discoverApi.getPopularFeeds(nextPage, 10)
    const list = res.data.list || []
    if (list.length > 0) feedPage.value = nextPage
    popularFeeds.value.push(...list)
    feedHasMore.value = !!res.data.has_more
  } catch (e) {
    // handled by interceptor
  } finally {
    feedLoadingMore.value = false
  }
}

// 点赞进行中的动态 id 集合：防止快速连点产生 like/unlike 竞态请求
const likeInFlight = new Set()

async function handleLike(feedId) {
  if (likeInFlight.has(feedId)) return
  likeInFlight.add(feedId)
  try {
    await feedApi.like(feedId)
    const feed = popularFeeds.value.find((f) => f.id === feedId)
    if (feed) {
      feed.is_liked = true
      feed.like_count = (feed.like_count || 0) + 1
    }
  } catch (e) {
    // handled by interceptor
  } finally {
    likeInFlight.delete(feedId)
  }
}

async function handleUnlike(feedId) {
  if (likeInFlight.has(feedId)) return
  likeInFlight.add(feedId)
  try {
    await feedApi.unlike(feedId)
    const feed = popularFeeds.value.find((f) => f.id === feedId)
    if (feed) {
      feed.is_liked = false
      feed.like_count = Math.max((feed.like_count || 0) - 1, 0)
    }
  } catch (e) {
    // handled by interceptor
  } finally {
    likeInFlight.delete(feedId)
  }
}

// 数字缩写：1.2w / 986
function formatCount(value) {
  const v = Number(value) || 0
  if (v >= 10000) return `${(v / 10000).toFixed(1).replace(/\.0$/, '')}w`
  if (v >= 1000) return `${(v / 1000).toFixed(1).replace(/\.0$/, '')}k`
  return String(v)
}

// ==================== 搜索逻辑（原有功能保留） ====================
async function searchUsers() {
  const q = keyword.value.trim()
  if (!q) return

  searching.value = true
  searched.value = true
  try {
    const res = await userApi.searchUsers(q, page.value, pageSize)
    cards.value = res.data.list || []
    total.value = res.data.total || 0
  } catch (e) {
    // handled by interceptor
  } finally {
    searching.value = false
  }
}

function clearSearch() {
  searched.value = false
  cards.value = []
  page.value = 1
  total.value = 0
  if (popularFeeds.value.length === 0 && !feedLoading.value) {
    loadPopular()
  }
}

function goToProfile(userId) {
  router.push(`/profile/${userId}`)
}

function goToFeedDetail(feedId) {
  if (!feedId) return
  router.push(`/feed/${feedId}`)
}

function coverStyle(user) {
  // 使用稳定“伪随机”高度与渐变，让卡片更接近笔记流视觉。
  const seed = Number(user.id || 1)
  const h = 140 + (seed % 4) * 28
  const palette = [
    'linear-gradient(135deg,#f3dde2,#e6cbd4)',
    'linear-gradient(135deg,#dfe5ef,#ccd7e7)',
    'linear-gradient(135deg,#dee8e0,#c8d9cf)',
    'linear-gradient(135deg,#e9e3da,#d9d0c2)',
  ]
  return {
    height: `${h}px`,
    background: palette[seed % palette.length],
  }
}

// 全局搜索：与桌面顶栏行为一致，跳转到聚合搜索结果页
function goGlobalSearch() {
  const q = globalKeyword.value.trim()
  if (!q) {
    router.push('/search_result')
    return
  }
  router.push({ path: '/search_result', query: { keyword: q, type: 'users' } })
}
</script>

<style scoped>
.discover-xhs {
  min-width: 0;
}

/* 移动端全局搜索胶囊：默认隐藏，仅 ≤960px 显示 */
.mobile-global-search {
  display: none;
}

@media (max-width: 960px) {
  .mobile-global-search {
    display: grid;
    grid-template-columns: 20px 1fr auto;
    align-items: center;
    gap: 8px;
    height: 44px;
    padding: 0 6px 0 14px;
    margin-bottom: 12px;
    background: var(--surface-raised);
    border: 1px solid var(--border-subtle);
    border-radius: var(--r-pill);
    box-shadow: var(--shadow-1);
  }

  .ms-icon {
    color: var(--text-tertiary);
    font-size: 15px;
  }

  .ms-input {
    border: 0;
    outline: none;
    height: 100%;
    font-size: 16px; /* ≥16px 避免 iOS 聚焦自动放大 */
    color: var(--text-primary);
    background: transparent;
  }

  .ms-input::placeholder {
    color: var(--text-tertiary);
  }

  .ms-action {
    border: 0;
    height: 34px;
    padding: 0 16px;
    border-radius: var(--r-pill);
    background: var(--ink);
    color: var(--on-ink);
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
  }
}

.discover-head {
  margin-bottom: 16px;
}

.head-title {
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.02em;
  margin-bottom: 6px;
}

.head-sub {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-bottom: 14px;
}

/* ==================== 推荐态 ==================== */
.section-title {
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-primary);
  margin-bottom: 12px;
}

.section-title.block {
  padding: 4px;
}

.popular-users {
  margin-bottom: 16px;
}

.user-strip {
  display: flex;
  gap: 18px;
  overflow-x: auto;
  padding-bottom: 4px;
  scrollbar-width: thin;
}

.user-chip {
  flex: 0 0 auto;
  width: 84px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.chip-avatar {
  border-radius: 16px;
  background: linear-gradient(135deg, #3a3a40 0%, #111113 100%);
  color: #fff;
  font-weight: 700;
}

.chip-name {
  display: flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--text-primary);
}

.chip-name-text {
  max-width: 56px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chip-meta {
  font-size: 11px;
  color: var(--text-tertiary);
}

.empty-wrap {
  padding: 24px;
}

.load-more {
  text-align: center;
  color: var(--text-tertiary);
  font-size: 13px;
  padding: 8px 0 20px;
}

/* ==================== 搜索态（原有用户卡片） ==================== */
.waterfall {
  column-count: 2;
  column-gap: 16px;
}

.note-card {
  break-inside: avoid;
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-lg);
  overflow: hidden;
  margin-bottom: 16px;
  cursor: pointer;
  box-shadow: var(--shadow-1);
  transition: transform var(--dur-med) var(--ease), box-shadow var(--dur-med) var(--ease);
}

.note-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-3);
}

.note-cover {
  width: 100%;
  display: grid;
  place-items: center;
}

.note-avatar {
  border: 3px solid rgba(255, 255, 255, 0.85);
  box-shadow: var(--shadow-1);
  font-weight: 700;
}

.note-body {
  padding: 12px 14px 14px;
}

.note-title {
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-primary);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.note-desc {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.note-meta {
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-secondary);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.pager-wrap {
  margin-top: 12px;
  display: flex;
  justify-content: center;
}

@media (max-width: 900px) {
  .waterfall {
    column-count: 1;
  }
}
</style>
