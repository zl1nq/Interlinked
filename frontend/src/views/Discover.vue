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
      <div class="head-sub">搜索用户并浏览推荐卡片</div>
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

    <div v-if="cards.length === 0" class="card empty-wrap">
      <el-empty description="输入关键词开始探索" />
    </div>

    <!-- 小红书风格瀑布流：双列卡片 -->
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

    <div v-if="searched && total > pageSize" class="pager-wrap">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="searchUsers"
      />
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { userApi } from '../api'

const router = useRouter()
const route = useRoute()

const keyword = ref('')
const globalKeyword = ref('')
const cards = ref([])
const searching = ref(false)
const searched = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)

onMounted(() => {
  const q = String(route.query.keyword || '').trim()
  if (q) {
    keyword.value = q
    searchUsers()
  }
})

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

function goToProfile(userId) {
  router.push(`/profile/${userId}`)
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
