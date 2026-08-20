<template>
  <div class="feed-detail-page">
    <section class="card detail-head">
      <el-button text @click="router.back()">返回</el-button>
      <h2>动态详情</h2>
    </section>

    <section class="card detail-body" v-loading="loading">
      <el-empty v-if="!loading && !feed" description="动态不存在或已删除" />

      <FeedCard
        v-else-if="feed"
        :feed="feed"
        :can-delete-feed="Number(feed.user_id) === Number(currentUserId)"
        @like="handleLike"
        @unlike="handleUnlike"
        @delete="handleDelete"
        @click-author="goToProfile"
      />
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { feedApi } from '../api'
import FeedCard from '../components/FeedCard.vue'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const feed = ref(null)

const currentUser = computed(() => JSON.parse(sessionStorage.getItem('user') || 'null'))
const currentUserId = computed(() => currentUser.value?.id || 0)

onMounted(() => {
  loadFeed()
})

async function loadFeed() {
  const id = Number(route.params.id)
  if (!id) return

  loading.value = true
  try {
    const res = await feedApi.getFeed(id)
    feed.value = res.data || null
  } catch (e) {
    feed.value = null
  } finally {
    loading.value = false
  }
}

async function handleLike(feedId) {
  await feedApi.like(feedId)
  if (feed.value) {
    feed.value.is_liked = true
    feed.value.like_count = (feed.value.like_count || 0) + 1
  }
}

async function handleUnlike(feedId) {
  await feedApi.unlike(feedId)
  if (feed.value) {
    feed.value.is_liked = false
    feed.value.like_count = Math.max((feed.value.like_count || 0) - 1, 0)
  }
}

async function handleDelete(feedId) {
  await feedApi.deleteFeed(feedId)
  ElMessage.success('删除成功')
  router.back()
}

function goToProfile(userId) {
  if (!userId) return
  router.push(`/profile/${userId}`)
}
</script>

<style scoped>
.feed-detail-page {
  min-width: 0;
}

.detail-head {
  border-radius: 12px;
  margin-bottom: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.detail-head h2 {
  margin: 0;
  font-size: 18px;
}

.detail-body {
  border-radius: 12px;
}
</style>
