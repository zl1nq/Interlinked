<template>
  <article class="moments-item">
    <div class="avatar-wrap" @click="$emit('click-author', feed.author?.id)">
      <el-avatar :size="42" :src="feed.author?.avatar || ''" class="avatar">
        {{ feed.author?.nickname?.charAt(0) || 'U' }}
      </el-avatar>
    </div>

    <div class="moments-main" @click="emit('click-feed', feed.id)">
      <div class="name-row" @click.stop="$emit('click-author', feed.author?.id)">
        <span class="nickname">{{ feed.author?.nickname || '未知用户' }}</span>
        <span v-if="feed.author?.is_big_v" class="v-badge">V</span>
      </div>

      <p v-if="feed.content" class="moments-text clickable-feed">{{ feed.content }}</p>

      <div v-if="images.length" class="moments-grid" :class="`grid-${Math.min(images.length, 9)}`" @click.stop>
        <el-image
          v-for="(img, idx) in images.slice(0, 9)"
          :key="img + idx"
          :src="img"
          fit="cover"
          class="grid-img"
          :preview-src-list="images"
          :initial-index="idx"
          preview-teleported
        />
      </div>

      <div v-if="videos.length" class="video-wrap" @click.stop>
        <video v-for="(video, idx) in videos" :key="video + idx" class="feed-video" :src="video" controls preload="metadata" />
      </div>

      <div v-if="feed.feed_type === 1 && feed.original_feed" class="repost-box">
        <div class="repost-author clickable-user" @click="$emit('click-author', feed.original_feed.author?.id)">@{{ feed.original_feed.author?.nickname || '原作者' }}</div>
        <div class="repost-content">{{ feed.original_feed.content || '转发了一条动态' }}</div>

        <div v-if="originalImages.length" class="repost-grid" :class="`grid-${Math.min(originalImages.length, 9)}`" @click.stop>
          <el-image
            v-for="(img, idx) in originalImages.slice(0, 9)"
            :key="img + idx"
            :src="img"
            fit="cover"
            class="grid-img"
            :preview-src-list="originalImages"
            :initial-index="idx"
            preview-teleported
          />
        </div>

        <div v-if="originalVideos.length" class="video-wrap repost-video-wrap" @click.stop>
          <video
            v-for="(video, idx) in originalVideos"
            :key="video + idx"
            class="feed-video"
            :src="video"
            controls
            preload="metadata"
          />
        </div>
      </div>

      <div class="meta-row" @click.stop>
        <div class="meta-left">
          <span class="time">{{ formatTime(feed.created_at) }}</span>
          <button v-if="canDeleteFeed" class="inline-delete-btn" type="button" @click="confirmDelete">删除</button>
        </div>
        <div class="meta-actions">
          <el-popover placement="left" trigger="click" width="178" popper-class="moments-action-pop" :show-arrow="false">
            <template #reference>
              <button class="dot-btn" type="button" aria-label="操作菜单">··</button>
            </template>

            <div class="actions-pop">
              <button class="action-btn" type="button" @click="handleLikeAction">
                {{ feed.is_liked ? '取消' : '赞' }}
              </button>
              <span class="divider"></span>
              <button class="action-btn" type="button" @click="toggleComments">评论</button>
              <template v-if="!canDeleteFeed">
                <span class="divider"></span>
                <button class="action-btn" type="button" @click="handleRepost">转发</button>
              </template>
            </div>
          </el-popover>
        </div>
      </div>

      <div v-if="hasInteractions" class="interactions-wrap">
        <div v-if="feed.like_count" class="likes-line">
          <el-icon class="heart"><StarFilled /></el-icon>
          <span v-if="likers.length">
            <template v-for="(u, idx) in likers" :key="u.id || idx">
              <span class="clickable-user" @click="$emit('click-author', u.author?.id || u.user_id)">{{ u.author?.nickname || u.nickname || '用户' }}</span><span v-if="idx < likers.length - 1">、</span>
            </template>
          </span>
          <span v-else>{{ feed.like_count }}人觉得很赞</span>
        </div>

        <div v-if="comments.length" class="comment-block">
          <div v-for="comment in comments" :key="comment.id" class="comment-line">
            <span class="comment-user clickable-user" @click="$emit('click-author', comment.user_id)">{{ comment.nickname || comment.username || '用户' }}</span>
            <span>：{{ comment.content }}</span>
            <button
              v-if="comment.user_id === currentUserId"
              class="comment-delete"
              type="button"
              @click="deleteComment(comment.id)"
            >
              删除
            </button>
          </div>
          <button v-if="commentsHasMore" class="more-comments" type="button" @click="loadComments">查看更多评论</button>
        </div>
      </div>

      <div v-if="showCommentEditor" class="comment-editor">
        <el-input
          v-model="commentContent"
          size="small"
          maxlength="500"
          placeholder="说点什么..."
          @keyup.enter="handleComment"
        >
          <template #append>
            <el-button :loading="commenting" @click="handleComment">发送</el-button>
          </template>
        </el-input>
      </div>
    </div>
  </article>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useUserStore } from '../stores/user'
import { feedApi } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const props = defineProps({
  feed: {
    type: Object,
    required: true,
  },
  canDeleteFeed: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['like', 'unlike', 'delete', 'click-author', 'click-feed', 'repost-success'])

const userStore = useUserStore()

const showCommentEditor = ref(false)
const commentContent = ref('')
const commenting = ref(false)
const comments = ref([])
const commentPage = ref(1)
const commentsHasMore = ref(false)
const likers = ref([])

const currentUserId = computed(() => userStore.userInfo?.id)
const hasInteractions = computed(() => (props.feed.like_count || 0) > 0 || comments.value.length > 0)

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

onMounted(async () => {
  try {
    if ((props.feed.like_count || 0) > 0) {
      await loadLikers()
    }
    if ((props.feed.comment_count || 0) > 0) {
      await loadComments(true)
    }
  } catch (e) {
    // handled by interceptor
  }
})

watch(
  () => props.feed.like_count,
  async (count) => {
    try {
      if ((count || 0) > 0) {
        await loadLikers()
      } else {
        likers.value = []
      }
    } catch (e) {
      // handled by interceptor
    }
  }
)

watch(
  () => props.feed.comment_count,
  async (count) => {
    try {
      if ((count || 0) > 0) {
        await loadComments(true)
      } else {
        comments.value = []
        commentsHasMore.value = false
        commentPage.value = 1
      }
    } catch (e) {
      // handled by interceptor
    }
  }
)

function toggleComments() {
  showCommentEditor.value = !showCommentEditor.value
}

async function loadLikers() {
  const res = await feedApi.getLikers(props.feed.id, 1, 20)
  likers.value = res.data.list || []
}

async function loadComments(reset = false) {
  if (reset) commentPage.value = 1

  const res = await feedApi.getComments(props.feed.id, commentPage.value, 10)
  const list = res.data.list || []
  comments.value = commentPage.value === 1 ? list : [...comments.value, ...list]
  commentsHasMore.value = res.data.has_more
  if (res.data.has_more) commentPage.value += 1
}

async function handleComment() {
  const content = commentContent.value.trim()
  if (!content) return

  commenting.value = true
  try {
    await feedApi.addComment(props.feed.id, content)
    commentContent.value = ''
    props.feed.comment_count = (props.feed.comment_count || 0) + 1
    await loadComments(true)
    ElMessage.success('评论成功')
  } catch (e) {
    // handled by interceptor
  } finally {
    commenting.value = false
  }
}

async function deleteComment(commentId) {
  try {
    await feedApi.deleteComment(props.feed.id, commentId)
    comments.value = comments.value.filter((item) => item.id !== commentId)
    props.feed.comment_count = Math.max((props.feed.comment_count || 0) - 1, 0)
    ElMessage.success('评论已删除')
  } catch (e) {
    // handled by interceptor
  }
}

async function handleLikeAction() {
  try {
    if (props.feed.is_liked) {
      await emit('unlike', props.feed.id)
    } else {
      await emit('like', props.feed.id)
    }
  } catch (e) {
    // handled by interceptor
  }
}

function confirmDelete() {
  ElMessageBox.confirm('确定删除这条朋友圈吗？', '提示', { type: 'warning' })
    .then(() => emit('delete', props.feed.id))
    .catch(() => {})
}

async function handleRepost() {
  try {
    await feedApi.repost({ original_id: props.feed.id, content: '' })
    ElMessage.success('转发成功')
    emit('repost-success')
  } catch (e) {
    // handled by interceptor
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
.moments-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 14px 0;
  border-bottom: 1px solid #eef0f3;
}

.avatar-wrap {
  cursor: pointer;
}

.avatar {
  border-radius: 6px;
  background: linear-gradient(135deg, #6f94ff 0%, #4a70df 100%);
  color: #fff;
  font-weight: 600;
}

.moments-main {
  flex: 1;
  min-width: 0;
}

.name-row {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
}

.nickname {
  font-size: 16px;
  font-weight: 600;
  color: #576b95;
}

.v-badge {
  width: 16px;
  height: 16px;
  line-height: 16px;
  text-align: center;
  border-radius: 50%;
  font-size: 11px;
  color: #fff;
  background: #f5a623;
}

.moments-text {
  margin-top: 6px;
  color: #1f2329;
  line-height: 1.7;
  font-size: 15px;
  white-space: pre-wrap;
  word-break: break-word;
}

.clickable-feed {
  cursor: pointer;
}

.clickable-feed:hover {
  opacity: 0.9;
}

.moments-grid,
.repost-grid {
  margin-top: 8px;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 4px;
  width: min(260px, 100%);
}

.moments-grid.grid-1,
.repost-grid.grid-1 {
  grid-template-columns: minmax(0, 1fr);
  width: min(220px, 60vw);
}

.grid-img {
  width: 100%;
  aspect-ratio: 1;
  border-radius: 2px;
  background: #eef1f6;
}

.video-wrap {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: min(320px, 100%);
}

.feed-video {
  width: 100%;
  max-height: 360px;
  border-radius: 6px;
  background: #000;
}

.repost-box {
  margin-top: 8px;
  background: #f2f4f7;
  border-radius: 4px;
  padding: 8px 10px;
  color: #4b5563;
}

.repost-author {
  color: #576b95;
  margin-bottom: 2px;
}

.meta-row {
  margin-top: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.meta-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.time {
  font-size: 12px;
  color: #a3a9b4;
}

.inline-delete-btn {
  border: 0;
  background: transparent;
  color: #f56c6c;
  font-size: 12px;
  cursor: pointer;
  padding: 0;
}

.inline-delete-btn:hover {
  text-decoration: underline;
}

.dot-btn {
  width: 24px;
  height: 18px;
  border: 0;
  border-radius: 3px;
  background: #f0f2f5;
  color: #7f8899;
  cursor: pointer;
  font-weight: 700;
  letter-spacing: 1px;
}

.actions-pop {
  display: flex;
  align-items: stretch;
  justify-content: space-between;
  background: #44484f;
  border-radius: 4px;
  padding: 0;
}

/* 去掉 el-popover 默认外层白框，只保留内部黑色操作条 */
:deep(.moments-action-pop.el-popper) {
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
  padding: 0 !important;
}

:deep(.moments-action-pop .el-popper__arrow) {
  display: none !important;
}

.action-btn {
  flex: 1;
  border: 0;
  background: transparent;
  color: #fff;
  cursor: pointer;
  font-size: 12px;
  line-height: 30px;
  padding: 0 10px;
  text-align: center;
  white-space: nowrap;
}

.action-btn.danger {
  color: #ffcbcb;
}

.divider {
  width: 1px;
  align-self: center;
  height: 14px;
  background: rgba(255, 255, 255, 0.26);
}

.interactions-wrap {
  margin-top: 8px;
  background: #f5f6f8;
  border-radius: 4px;
  padding: 8px 10px;
}

.likes-line {
  color: #576b95;
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 4px;
}

.heart {
  font-size: 12px;
}

.comment-block {
  border-top: 1px solid #e6e8ed;
  padding-top: 6px;
}

.comment-line {
  font-size: 13px;
  line-height: 1.7;
  color: #1f2329;
}

.comment-user {
  color: #576b95;
}

.clickable-user {
  color: #576b95;
  cursor: pointer;
}

.clickable-user:hover {
  text-decoration: underline;
}

.comment-delete {
  margin-left: 8px;
  border: 0;
  background: transparent;
  color: #9fa8ba;
  cursor: pointer;
  font-size: 12px;
}

.more-comments {
  margin-top: 4px;
  border: 0;
  background: transparent;
  color: #7e8aa5;
  font-size: 12px;
  cursor: pointer;
  padding: 0;
}

.comment-editor {
  margin-top: 8px;
}
</style>
