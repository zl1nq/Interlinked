<template>
  <article class="wf-card" @click="emit('click-feed', feed.id)">
    <!-- 封面：优先自己的首图，转发动态回退到原动态的封面 -->
    <div v-if="coverUrl" class="wf-cover">
      <img :src="coverUrl" loading="lazy" alt="" />
      <span v-if="coverImageTotal > 1" class="wf-media-count">{{ coverImageTotal }}图</span>
    </div>
    <div v-else-if="coverVideo" class="wf-cover">
      <video :src="coverVideo" muted preload="metadata" />
      <span class="wf-media-count">视频</span>
    </div>

    <div class="wf-body">
      <p v-if="feed.content" class="wf-text" :class="{ 'wf-text-only': !coverUrl && !coverVideo }">
        {{ feed.content }}
      </p>

      <div v-if="feed.feed_type === 1 && feed.original_feed" class="wf-repost">
        <span class="wf-repost-author">@{{ feed.original_feed.author?.nickname || '原作者' }}</span>
        <span class="wf-repost-content">{{ feed.original_feed.content || '转发了一条动态' }}</span>
      </div>

      <div class="wf-footer">
        <div class="wf-author" @click.stop="emit('click-author', feed.author?.id)">
          <el-avatar :size="22" :src="feed.author?.avatar || ''" class="wf-avatar">
            {{ feed.author?.nickname?.charAt(0) || 'U' }}
          </el-avatar>
          <span class="wf-nickname">{{ feed.author?.nickname || '未知用户' }}</span>
        </div>

        <div class="wf-actions">
          <span class="wf-time">{{ formatTime(feed.created_at) }}</span>
          <button
            class="wf-action"
            :class="{ liked: feed.is_liked }"
            type="button"
            :aria-label="feed.is_liked ? '取消点赞' : '点赞'"
            @click.stop="toggleLike"
          >
            <el-icon><StarFilled v-if="feed.is_liked" /><Star v-else /></el-icon>
            <span v-if="feed.like_count">{{ feed.like_count }}</span>
          </button>
          <button class="wf-action" type="button" aria-label="查看评论" @click.stop="emit('click-feed', feed.id)">
            <el-icon><ChatDotRound /></el-icon>
            <span v-if="feed.comment_count">{{ feed.comment_count }}</span>
          </button>
        </div>
      </div>
    </div>
  </article>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  feed: {
    type: Object,
    required: true,
  },
})

const emit = defineEmits(['like', 'unlike', 'click-author', 'click-feed'])

function parseMedia(raw) {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

const images = computed(() => parseMedia(props.feed.images))
const videos = computed(() => parseMedia(props.feed.videos))
const originalImages = computed(() => parseMedia(props.feed.original_feed?.images))
const originalVideos = computed(() => parseMedia(props.feed.original_feed?.videos))

// 有自己的媒体用自己，否则转发动态借用原动态的封面
const ownHasMedia = computed(() => images.value.length > 0 || videos.value.length > 0)
const coverImages = computed(() => (ownHasMedia.value ? images.value : originalImages.value))
const coverVideos = computed(() => (ownHasMedia.value ? videos.value : originalVideos.value))
const coverUrl = computed(() => coverImages.value[0] || '')
const coverVideo = computed(() => (coverUrl.value ? '' : coverVideos.value[0] || ''))
const coverImageTotal = computed(() => coverImages.value.length)

function toggleLike() {
  if (props.feed.is_liked) {
    emit('unlike', props.feed.id)
  } else {
    emit('like', props.feed.id)
  }
}

function formatTime(timeStr) {
  if (!timeStr) return ''
  const date = new Date(timeStr)
  const now = new Date()
  const diff = (now - date) / 1000

  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`

  const thisYear = now.getFullYear() === date.getFullYear()
  if (thisYear) return `${date.getMonth() + 1}月${date.getDate()}日`
  return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日`
}
</script>

<style scoped>
/* 瀑布流紧凑卡片：封面 + 摘要 + 作者 + 就地互动 */
.wf-card {
  break-inside: avoid;
  margin-bottom: 16px;
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-lg);
  overflow: hidden;
  cursor: pointer;
  box-shadow: var(--shadow-1);
  transition: transform var(--dur-med) var(--ease), box-shadow var(--dur-med) var(--ease);
}

.wf-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-3);
}

.wf-cover {
  position: relative;
  width: 100%;
  background: var(--surface-sunken);
}

.wf-cover img,
.wf-cover video {
  display: block;
  width: 100%;
  max-height: 320px;
  object-fit: cover;
}

.wf-media-count {
  position: absolute;
  top: 10px;
  right: 10px;
  padding: 2px 8px;
  border-radius: var(--r-pill);
  background: rgba(17, 17, 26, 0.55);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
}

.wf-body {
  padding: 12px 14px 12px;
}

.wf-text {
  color: var(--text-primary);
  font-size: 14px;
  line-height: 1.65;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-word;
}

/* 纯文字动态：无封面挤压，字号放大作为视觉主体 */
.wf-text-only {
  font-size: 15.5px;
  line-height: 1.75;
  -webkit-line-clamp: 8;
  padding: 4px 0 6px;
}

.wf-repost {
  margin-top: 8px;
  padding: 8px 10px;
  background: var(--surface-sunken);
  border-radius: var(--r-sm);
  font-size: 12.5px;
  color: var(--text-secondary);
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.wf-repost-author {
  color: var(--text-primary);
  font-weight: 600;
  margin-right: 4px;
}

.wf-footer {
  margin-top: 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.wf-author {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  cursor: pointer;
}

.wf-avatar {
  flex: 0 0 auto;
  border-radius: 7px;
  background: linear-gradient(135deg, #3a3a40 0%, #111113 100%);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}

.wf-nickname {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.wf-author:hover .wf-nickname {
  color: var(--text-primary);
  text-decoration: underline;
  text-underline-offset: 3px;
}

.wf-actions {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.wf-time {
  font-size: 11px;
  color: var(--text-tertiary);
}

.wf-action {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  border: 0;
  background: transparent;
  color: var(--text-tertiary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  padding: 2px;
  transition: color var(--dur-fast) var(--ease), transform var(--dur-fast) var(--ease);
}

.wf-action .el-icon {
  font-size: 15px;
}

.wf-action:hover {
  color: var(--text-primary);
}

.wf-action:active {
  transform: scale(0.9);
}

.wf-action.liked {
  color: var(--accent);
}
</style>
