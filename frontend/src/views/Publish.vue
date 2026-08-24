<template>
  <div class="publish-page card">
    <section class="composer">
      <div class="composer-avatar">
        <el-avatar :size="44" :src="userStore.userInfo?.avatar || ''">{{ userStore.userInfo?.nickname?.charAt(0) || '我' }}</el-avatar>
      </div>

      <div class="composer-main">
        <el-input
          v-model="publishContent"
          type="textarea"
          :rows="4"
          maxlength="5000"
          resize="none"
          placeholder="分享这一刻..."
        />

        <div v-if="mediaPreviews.length" class="composer-medias">
          <div v-for="(item, idx) in mediaPreviews" :key="item.url + idx" class="preview-item">
            <template v-if="item.kind === 'image'">
              <el-image :src="item.url" fit="cover" class="preview-image" />
            </template>
            <template v-else>
              <video class="preview-video" :src="item.url" controls preload="metadata" />
            </template>
            <button class="remove-btn" type="button" @click="removeMedia(idx)">×</button>
          </div>
        </div>

        <div class="composer-footer">
          <div class="composer-tools">
            <label class="upload-btn" for="publish-upload-input">
              <el-icon><Plus /></el-icon>
              <span>添加图片/视频</span>
            </label>
            <input
              id="publish-upload-input"
              class="hidden-input"
              type="file"
              accept="image/*,video/*"
              multiple
              @change="handleSelectMedia"
            />
            <span class="count">{{ publishContent.length }}/5000</span>
          </div>

          <el-button
            type="primary"
            class="publish-btn"
            :loading="publishing"
            :disabled="!canPublish"
            @click="handlePublish"
          >
            发布
          </el-button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { useUserStore } from '../stores/user'
import { feedApi, uploadApi } from '../api'

const userStore = useUserStore()

const publishing = ref(false)
const publishContent = ref('')
const mediaPreviews = ref([])
const uploadedImageUrls = ref([])
const uploadedVideoUrls = ref([])

const canPublish = computed(() => {
  return !!publishContent.value.trim() || uploadedImageUrls.value.length > 0 || uploadedVideoUrls.value.length > 0
})

async function handleSelectMedia(event) {
  const files = Array.from(event.target.files || [])
  event.target.value = ''
  if (!files.length) return

  const mediaFiles = files.filter((file) => file.type?.startsWith('image/') || file.type?.startsWith('video/'))
  if (mediaFiles.length !== files.length) {
    ElMessage.warning('仅支持图片和视频文件')
  }
  if (!mediaFiles.length) return

  const allowedCount = Math.max(0, 9 - mediaPreviews.value.length)
  const picked = mediaFiles.slice(0, allowedCount)
  if (!picked.length) {
    ElMessage.warning('最多上传9个媒体文件')
    return
  }

  const previewItems = picked.map((file) => ({
    kind: file.type.startsWith('video/') ? 'video' : 'image',
    file,
    url: URL.createObjectURL(file),
    uploadedUrl: '',
  }))

  mediaPreviews.value.push(...previewItems)

  try {
    for (const item of previewItems) {
      if (item.kind === 'video') {
        const res = await uploadApi.uploadVideo(item.file)
        item.uploadedUrl = res.data.url
        uploadedVideoUrls.value.push(res.data.url)
      } else {
        const res = await uploadApi.uploadImage(item.file)
        item.uploadedUrl = res.data.url
        uploadedImageUrls.value.push(res.data.url)
      }
    }
  } catch (e) {
    // handled by interceptor
  }
}

function removeMedia(index) {
  const [removed] = mediaPreviews.value.splice(index, 1)
  if (!removed) return

  if (removed.kind === 'video') {
    const idx = uploadedVideoUrls.value.findIndex((u) => u === removed.uploadedUrl)
    if (idx >= 0) uploadedVideoUrls.value.splice(idx, 1)
  } else {
    const idx = uploadedImageUrls.value.findIndex((u) => u === removed.uploadedUrl)
    if (idx >= 0) uploadedImageUrls.value.splice(idx, 1)
  }

  if (removed.url?.startsWith('blob:')) URL.revokeObjectURL(removed.url)
}

async function handlePublish() {
  if (!canPublish.value) return
  publishing.value = true
  try {
    const payload = {
      content: publishContent.value.trim(),
      images: uploadedImageUrls.value.length ? JSON.stringify(uploadedImageUrls.value) : '',
      videos: uploadedVideoUrls.value.length ? JSON.stringify(uploadedVideoUrls.value) : '',
    }
    await feedApi.publish(payload)

    publishContent.value = ''
    mediaPreviews.value.forEach((item) => {
      if (item.url?.startsWith('blob:')) URL.revokeObjectURL(item.url)
    })
    mediaPreviews.value = []
    uploadedImageUrls.value = []
    uploadedVideoUrls.value = []

    ElMessage.success('发布成功')
  } catch (e) {
    // handled by interceptor
  } finally {
    publishing.value = false
  }
}
</script>

<style scoped>
.publish-page {
  max-width: 640px;
  margin: 0 auto;
}

.composer {
  display: flex;
  gap: 12px;
}

.composer-avatar :deep(.el-avatar) {
  border-radius: 12px;
  background: linear-gradient(135deg, #3a3a40 0%, #111113 100%);
  font-weight: 700;
}

.composer-main {
  flex: 1;
  min-width: 0;
}

/* 无边框内凹输入区 */
.composer-main :deep(.el-textarea__inner) {
  background: var(--surface-sunken);
  border: 1px solid transparent;
  border-radius: var(--r-md);
  box-shadow: none;
  padding: 12px 14px;
  font-size: 15px;
  line-height: 1.7;
  color: var(--text-primary);
  transition: border-color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease);
}

.composer-main :deep(.el-textarea__inner:focus) {
  background: var(--surface-raised);
  border-color: var(--border-strong);
  box-shadow: none;
}

.composer-main :deep(.el-textarea__inner::placeholder) {
  color: var(--text-tertiary);
}

.composer-medias {
  margin-top: 12px;
  display: grid;
  grid-template-columns: repeat(3, 96px);
  gap: 10px;
}

.preview-item {
  position: relative;
  width: 96px;
  height: 96px;
}

.preview-image,
.preview-video {
  width: 100%;
  height: 100%;
  border-radius: var(--r-sm);
  background: var(--surface-sunken);
  object-fit: cover;
}

.remove-btn {
  position: absolute;
  top: -7px;
  right: -7px;
  width: 20px;
  height: 20px;
  border: 0;
  border-radius: var(--r-pill);
  background: var(--ink);
  color: var(--on-ink);
  cursor: pointer;
  font-size: 13px;
  line-height: 20px;
  box-shadow: var(--shadow-1);
  transition: transform var(--dur-fast) var(--ease);
}

.remove-btn:hover {
  transform: scale(1.1);
}

.composer-footer {
  margin-top: 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.composer-tools {
  display: flex;
  align-items: center;
  gap: 12px;
}

.upload-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  padding: 7px 14px;
  border-radius: var(--r-pill);
  border: 1px solid var(--border-subtle);
  background: var(--surface-raised);
  transition: border-color var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease);
}

.upload-btn:hover {
  border-color: var(--border-strong);
  color: var(--text-primary);
}

.hidden-input {
  display: none;
}

.count {
  color: var(--text-tertiary);
  font-size: 12px;
}

.publish-btn {
  min-width: 88px;
  border-radius: var(--r-pill);
  font-weight: 600;
}
</style>
