<template>
  <div class="discover-xhs">
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

function goToProfile(userId) {
  router.push(`/profile/${userId}`)
}

function coverStyle(user) {
  // 使用稳定“伪随机”高度与渐变，让卡片更接近笔记流视觉。
  const seed = Number(user.id || 1)
  const h = 140 + (seed % 4) * 28
  const palette = [
    'linear-gradient(135deg,#ffe4ec,#ffd9d9)',
    'linear-gradient(135deg,#e4f3ff,#d7e6ff)',
    'linear-gradient(135deg,#e9ffe8,#d8f8d7)',
    'linear-gradient(135deg,#fff3dd,#ffe5c2)',
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

.discover-head {
  border-radius: 14px;
  margin-bottom: 14px;
}

.head-title {
  font-size: 24px;
  font-weight: 800;
  margin-bottom: 6px;
}

.head-sub {
  font-size: 13px;
  color: #8a93a5;
  margin-bottom: 12px;
}

.waterfall {
  column-count: 2;
  column-gap: 14px;
}

.note-card {
  break-inside: avoid;
  background: #fff;
  border-radius: 14px;
  overflow: hidden;
  margin-bottom: 14px;
  cursor: pointer;
  box-shadow: 0 2px 10px rgba(25, 35, 56, 0.08);
  transition: transform .2s ease, box-shadow .2s ease;
}

.note-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 22px rgba(25, 35, 56, 0.14);
}

.note-cover {
  width: 100%;
  display: grid;
  place-items: center;
}

.note-avatar {
  border: 3px solid rgba(255, 255, 255, 0.75);
}

.note-body {
  padding: 10px 12px 12px;
}

.note-title {
  font-size: 15px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.note-desc {
  margin-top: 4px;
  font-size: 12px;
  color: #8b94a7;
}

.note-meta {
  margin-top: 8px;
  font-size: 12px;
  color: #6f788a;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.empty-wrap {
  border-radius: 14px;
}

.pager-wrap {
  margin-top: 10px;
  display: flex;
  justify-content: center;
}

@media (max-width: 900px) {
  .waterfall {
    column-count: 1;
  }
}
</style>
