<template>
  <div class="messages-page card">
    <div class="messages-sidebar">
      <div class="messages-title">消息</div>
      <div v-if="conversationLoading" class="placeholder">加载中...</div>
      <div v-else-if="conversations.length === 0" class="placeholder">暂无会话</div>
      <div v-else class="conversation-list">
        <div
          v-for="item in conversations"
          :key="item.target_id"
          class="conversation-item"
          :class="{ active: selectedTargetId === item.target_id }"
          @click="selectConversation(item)"
        >
          <el-avatar :size="42" :src="item.user?.avatar || ''" class="clickable-avatar" @click.stop="goToUserProfile(item.user?.id)">
            {{ item.user?.nickname?.charAt(0) || 'U' }}
          </el-avatar>
          <div class="conversation-meta">
            <div class="top-row">
              <span class="name">{{ item.user?.nickname || '用户' }}</span>
              <div class="right-meta">
                <span class="time">{{ formatTime(item.last_time) }}</span>
                <el-badge v-if="item.unread > 0" :value="item.unread" class="unread-badge" />
              </div>
            </div>
            <div class="last-msg">{{ item.last_msg || '暂无消息' }}</div>
          </div>
        </div>
      </div>
    </div>

    <div class="messages-main">
      <template v-if="selectedTargetId">
        <div class="chat-header">
          <span>{{ selectedUserName }}</span>
          <span class="ws-state" :class="{ online: wsConnected }">{{ wsConnected ? '实时已连接' : '实时重连中...' }}</span>
        </div>
        <div class="chat-list" ref="chatListRef">
          <div
            v-for="msg in messages"
            :key="msg.id"
            class="chat-item"
            :class="{ mine: msg.from_user_id === currentUserId }"
          >
            <template v-if="msg.from_user_id === currentUserId">
              <div class="bubble">{{ msg.content }}</div>
              <el-avatar :size="30" :src="currentUser?.avatar || ''" class="clickable-avatar" @click="goToUserProfile(currentUserId)">
                {{ currentUser?.nickname?.charAt(0) || '我' }}
              </el-avatar>
            </template>
            <template v-else>
              <el-avatar :size="30" :src="msg.from_user?.avatar || ''" class="clickable-avatar" @click="goToUserProfile(msg.from_user?.id)">
                {{ msg.from_user?.nickname?.charAt(0) || 'U' }}
              </el-avatar>
              <div class="bubble">{{ msg.content }}</div>
            </template>
          </div>
        </div>

        <div class="composer">
          <el-input
            v-model="inputContent"
            type="textarea"
            :rows="2"
            maxlength="1000"
            placeholder="请输入消息..."
            @keyup.enter.exact.prevent="send"
          />
          <el-button type="primary" :loading="sending" :disabled="!inputContent.trim()" @click="send">发送</el-button>
        </div>
      </template>

      <div v-else class="no-chat">选择一个联系人开始聊天</div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { messageApi } from '../api'
import { ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()

const conversations = ref([])
const conversationLoading = ref(false)
const selectedTargetId = ref(0)
const selectedUserName = ref('')

const messages = ref([])
const inputContent = ref('')
const sending = ref(false)
const chatListRef = ref(null)
const wsConnected = ref(false)
let ws = null
let reconnectTimer = null
let reconnectAttempt = 0

const pendingReqMap = new Map()
const pendingMessageMap = new Map()
const MAX_RECONNECT_DELAY_MS = 30000

const currentUser = computed(() => JSON.parse(sessionStorage.getItem('user') || 'null'))
const currentUserId = computed(() => currentUser.value?.id || 0)

function initSelectedTargetFromRoute() {
  const target = Number(route.query.target || route.query.user_id || route.query.to || 0)
  if (!target) return
  selectedTargetId.value = target
  selectedUserName.value = route.query.name ? String(route.query.name) : '私信'
  ensureSelectedConversationVisible()
}

onMounted(async () => {
  initSelectedTargetFromRoute()
  await connectWSWithAuth()
})

onUnmounted(() => {
  clearReconnectTimer()
  closeWS()
  pendingReqMap.clear()
})

function makeReqId() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

function clearReconnectTimer() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
}

function closeWS() {
  if (ws) {
    ws.onopen = null
    ws.onmessage = null
    ws.onclose = null
    ws.onerror = null
    ws.close()
    ws = null
  }
  wsConnected.value = false
}

function sendWSRequest(type, data, timeoutMs = 6000) {
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    return Promise.reject(new Error('ws not connected'))
  }

  const reqId = makeReqId()
  const payload = { type, data: { ...(data || {}), req_id: reqId } }

  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      pendingReqMap.delete(reqId)
      reject(new Error('request timeout'))
    }, timeoutMs)

    pendingReqMap.set(reqId, {
      resolve: (res) => {
        clearTimeout(timer)
        resolve(res)
      },
      reject: (err) => {
        clearTimeout(timer)
        reject(err)
      },
    })

    ws.send(JSON.stringify(payload))
  })
}

async function loadConversations() {
  ensureSelectedConversationVisible()
  conversationLoading.value = true
  try {
    let res
    if (wsConnected.value && ws?.readyState === WebSocket.OPEN) {
      res = await sendWSRequest('message:conversations', { page: 1, page_size: 50 })
    } else {
      const httpRes = await messageApi.getConversations(1, 50)
      res = httpRes?.data || httpRes
    }
    conversations.value = mergeSelectedConversation(res?.list || [])

    if (selectedTargetId.value) {
      ensureSelectedConversationVisible()
    }
  } catch (e) {
    ElMessage.error(e?.message || '获取会话失败')
  } finally {
    conversationLoading.value = false
  }
}

function selectedConversationPlaceholder() {
  const targetId = Number(selectedTargetId.value)
  if (!targetId) return null
  return {
    target_id: targetId,
    target_name: selectedUserName.value || '私信',
    user: { id: targetId, nickname: selectedUserName.value || '私信' },
    last_msg: '',
    last_time: '',
    unread: 0,
  }
}

function mergeSelectedConversation(list) {
  const merged = Array.isArray(list) ? [...list] : []
  const placeholder = selectedConversationPlaceholder()
  if (!placeholder) return merged
  const exists = merged.some((item) => Number(item.target_id) === Number(placeholder.target_id))
  if (!exists) {
    merged.unshift(placeholder)
  }
  return merged
}

function ensureSelectedConversationVisible() {
  const placeholder = selectedConversationPlaceholder()
  if (!placeholder) return

  const exists = conversations.value.some((item) => Number(item.target_id) === Number(placeholder.target_id))
  if (!exists) {
    conversations.value.unshift(placeholder)
  }
}

async function loadMessages(targetId) {
  if (!targetId) return
  try {
    let res
    if (wsConnected.value && ws?.readyState === WebSocket.OPEN) {
      res = await sendWSRequest('message:history', { target_user_id: targetId, page: 1, page_size: 100 })
    } else {
      const httpRes = await messageApi.getHistory(targetId, 1, 100)
      res = httpRes?.data || httpRes
    }
    messages.value = (res?.list || []).slice().reverse()
    const idx = conversations.value.findIndex((item) => Number(item.target_id) === Number(targetId))
    if (idx >= 0) {
      conversations.value[idx] = { ...conversations.value[idx], unread: 0 }
    }
    await nextTick()
    scrollToBottom()
  } catch (e) {
    ElMessage.error(e?.message || '获取历史消息失败')
  }
}

function selectConversation(item) {
  selectedTargetId.value = Number(item.target_id)
  selectedUserName.value = item.user?.nickname || '私信'
  router.replace({ path: '/messages', query: { target: selectedTargetId.value, name: selectedUserName.value } })
  loadMessages(selectedTargetId.value)
}

function enrichIncomingMessage(msg) {
  if (!msg || Number(msg.from_user_id) === Number(currentUserId.value)) return msg

  if (!msg.from_user || !msg.from_user.avatar) {
    const conv = conversations.value.find((x) => Number(x.target_id) === Number(msg.from_user_id))
    if (conv?.user) {
      msg.from_user = { ...msg.from_user, ...conv.user }
    }
  }
  return msg
}

function upsertConversationPreview(msg, isIncoming) {
  const currentId = Number(currentUserId.value)
  const fromUserId = Number(msg.from_user_id)
  const toUserId = Number(msg.to_user_id)
  const targetId = fromUserId === currentId ? toUserId : fromUserId
  if (!targetId) return

  const unreadInc = isIncoming && Number(selectedTargetId.value) !== targetId ? 1 : 0

  const idx = conversations.value.findIndex((x) => Number(x.target_id) === targetId)
  if (idx >= 0) {
    const old = conversations.value[idx]
    const mergedUser = old.user && Object.keys(old.user).length > 0
      ? old.user
      : (fromUserId === currentId ? (msg.to_user || {}) : (msg.from_user || {}))

    conversations.value[idx] = {
      ...old,
      user: mergedUser,
      last_msg: msg.content,
      last_time: msg.created_at,
      unread: Math.max(0, Number(old.unread) || 0) + unreadInc,
    }
  } else {
    const user = fromUserId === currentId ? (msg.to_user || {}) : (msg.from_user || {})
    conversations.value.unshift({
      target_id: targetId,
      target_name: user.nickname || selectedUserName.value || '用户',
      user,
      last_msg: msg.content,
      last_time: msg.created_at,
      unread: unreadInc,
    })
  }

  conversations.value.sort((a, b) => new Date(b.last_time).getTime() - new Date(a.last_time).getTime())
}

async function send() {
  const content = inputContent.value.trim()
  if (!content || !selectedTargetId.value) return

  if (!ws || ws.readyState !== WebSocket.OPEN || !wsConnected.value) {
    ElMessage.error('实时消息通道未连接，请等待重连后再发送')
    connectWSWithAuth()
    return
  }

  sending.value = true
  try {
    const clientMsgId = makeReqId()
    const pending = {
      id: `pending-${clientMsgId}`,
      from_user_id: Number(currentUserId.value),
      to_user_id: Number(selectedTargetId.value),
      content,
      created_at: new Date().toISOString(),
      from_user: currentUser.value || {},
      _pending: true,
      _client_msg_id: clientMsgId,
    }

    messages.value.push(pending)
    pendingMessageMap.set(clientMsgId, pending)
    upsertConversationPreview(pending, false)
    await nextTick()
    scrollToBottom()

    ws.send(JSON.stringify({
      type: 'message:send',
      data: {
        to_user_id: selectedTargetId.value,
        content,
        client_msg_id: clientMsgId,
      },
    }))
    inputContent.value = ''
  } catch (e) {
    ElMessage.error('发送失败')
  } finally {
    sending.value = false
  }
}

async function connectWSWithAuth() {
  const token = sessionStorage.getItem('token')
  if (!token) {
    wsConnected.value = false
    return
  }
  connectWS(token)
}

function connectWS(token) {
  closeWS()
  clearReconnectTimer()
  initSelectedTargetFromRoute()

  ws = messageApi.connectMessageWS(token)

  ws.onopen = async () => {
    wsConnected.value = true
    reconnectAttempt = 0
    await loadConversations()
    if (selectedTargetId.value) {
      await loadMessages(selectedTargetId.value)
    } else {
      messages.value = []
    }
  }

  ws.onclose = () => {
    wsConnected.value = false
    scheduleReconnect()
  }

  ws.onerror = () => {
    wsConnected.value = false
    loadConversations()
    if (selectedTargetId.value) {
      loadMessages(selectedTargetId.value)
    }
  }

  ws.onmessage = async (event) => {
    try {
      const payload = JSON.parse(event.data || '{}')
      const reqId = payload?.data?.req_id

      if (reqId && pendingReqMap.has(reqId)) {
        const req = pendingReqMap.get(reqId)
        pendingReqMap.delete(reqId)
        req.resolve(payload.data)
        return
      }

      if (payload?.type === 'auth:expired') {
        wsConnected.value = false
        closeWS()
        await connectWSWithAuth()
        return
      }

      if (payload?.type === 'message:ack') {
        const msg = payload?.data?.message
        const clientMsgId = payload?.data?.client_msg_id
        if (msg) {
          upsertConversationPreview(msg, false)

          if (clientMsgId && pendingMessageMap.has(clientMsgId)) {
            const pending = pendingMessageMap.get(clientMsgId)
            const idx = messages.value.findIndex((x) => x === pending || x._client_msg_id === clientMsgId)
            if (idx >= 0) {
              messages.value[idx] = { ...msg, from_user: msg.from_user || currentUser.value || {} }
            } else {
              messages.value.push({ ...msg, from_user: msg.from_user || currentUser.value || {} })
            }
            pendingMessageMap.delete(clientMsgId)
          } else if (Number(selectedTargetId.value) === Number(msg.to_user_id) || Number(selectedTargetId.value) === Number(msg.from_user_id)) {
            messages.value.push({ ...msg, from_user: msg.from_user || currentUser.value || {} })
          }

          await nextTick()
          scrollToBottom()
        }
        return
      }

      if (payload?.type === 'message:new') {
        const rawMsg = payload?.data
        if (rawMsg) {
          const msg = enrichIncomingMessage(rawMsg)
          upsertConversationPreview(msg, true)
          const currentId = Number(currentUserId.value)
          const fromUserId = Number(msg.from_user_id)
          const toUserId = Number(msg.to_user_id)
          const targetId = fromUserId === currentId ? toUserId : fromUserId
          if (targetId && Number(targetId) === Number(selectedTargetId.value)) {
            messages.value.push(msg)
            await nextTick()
            scrollToBottom()
          }
        }
        return
      }

      if (payload?.type === 'message:error') {
        const reqIdFromErr = payload?.data?.req_id
        if (reqIdFromErr && pendingReqMap.has(reqIdFromErr)) {
          const req = pendingReqMap.get(reqIdFromErr)
          pendingReqMap.delete(reqIdFromErr)
          req.reject(new Error(payload?.data?.message || 'request failed'))
          return
        }

        const errClientMsgId = payload?.data?.client_msg_id
        if (errClientMsgId && pendingMessageMap.has(errClientMsgId)) {
          const pending = pendingMessageMap.get(errClientMsgId)
          const idx = messages.value.findIndex((x) => x === pending || x._client_msg_id === errClientMsgId)
          if (idx >= 0) {
            messages.value.splice(idx, 1)
          }
          pendingMessageMap.delete(errClientMsgId)
        }

        ElMessage.error(payload?.data?.message || '消息发送失败')
      }
    } catch (e) {
      // ignore parse error
    }
  }
}

function scheduleReconnect() {
  clearReconnectTimer()
  reconnectAttempt += 1
  const delay = Math.min(1000 * 2 ** (reconnectAttempt - 1), MAX_RECONNECT_DELAY_MS)
  reconnectTimer = setTimeout(() => {
    connectWSWithAuth()
  }, delay)
}

function scrollToBottom() {
  const el = chatListRef.value
  if (!el) return
  el.scrollTop = el.scrollHeight
}

function goToUserProfile(userId) {
  if (!userId) return
  router.push(`/profile/${userId}`)
}

function formatTime(timeStr) {
  if (!timeStr) return ''
  const date = new Date(timeStr)
  if (Number.isNaN(date.getTime())) return ''
  const hh = String(date.getHours()).padStart(2, '0')
  const mm = String(date.getMinutes()).padStart(2, '0')
  return `${hh}:${mm}`
}
</script>

<style scoped>
.messages-page {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  gap: 0;
  padding: 0;
  min-height: calc(100dvh - 190px);
  height: calc(100dvh - 190px);
  overflow: hidden;
}

.messages-sidebar {
  border-right: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.messages-title {
  flex: 0 0 auto;
  padding: 18px 18px 14px;
  font-size: 19px;
  font-weight: 800;
  letter-spacing: -0.01em;
  border-bottom: 1px solid var(--border-subtle);
}

.placeholder {
  padding: 20px 18px;
  color: var(--text-tertiary);
  font-size: 13px;
}

.conversation-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px;
}

.conversation-item {
  display: flex;
  gap: 10px;
  padding: 10px 12px;
  cursor: pointer;
  border-radius: var(--r-md);
  transition: background var(--dur-fast) var(--ease);
}

.conversation-item:hover {
  background: var(--nav-hover-bg);
}

.clickable-avatar { cursor: pointer; }
.conversation-item :deep(.el-avatar) { border-radius: 12px; }
.conversation-item.active { background: var(--surface-sunken); }
.conversation-meta { min-width: 0; flex: 1; }
.top-row { display: flex; justify-content: space-between; align-items: flex-start; gap: 8px; }
.name { font-size: 14px; font-weight: 600; color: var(--text-primary); min-width: 0; flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.time { font-size: 12px; color: var(--text-tertiary); }
.last-msg { margin-top: 4px; font-size: 12px; color: var(--text-secondary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.right-meta { display: inline-flex; flex-direction: column; align-items: flex-end; gap: 4px; }
.unread-badge { margin-top: 0; }
.unread-badge :deep(.el-badge__content) { background: var(--accent); border: 0; }

.messages-main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  background: var(--surface-base);
}

.chat-header {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-subtle);
  flex: 0 0 auto;
}

.ws-state { font-size: 12px; color: var(--el-color-danger); font-weight: 500; }
.ws-state.online { color: #3d9a50; }

.chat-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 18px 20px;
}

.chat-item { display: flex; align-items: flex-end; gap: 8px; margin-bottom: 12px; }
.chat-item :deep(.el-avatar) { border-radius: 9px; }
.chat-item.mine { justify-content: flex-end; }

/* 气泡：对方白底发丝边，己方墨黑底白字 */
.bubble {
  max-width: 62%;
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-1);
  padding: 9px 13px;
  border-radius: 16px 16px 16px 5px;
  line-height: 1.65;
  font-size: 14px;
  color: var(--text-primary);
  word-break: break-word;
}

.chat-item.mine .bubble {
  background: var(--ink);
  border-color: transparent;
  color: var(--on-ink);
  border-radius: 16px 16px 5px 16px;
}

.composer {
  padding: 14px 16px;
  border-top: 1px solid var(--border-subtle);
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
  align-items: end;
  flex: 0 0 auto;
  background: var(--surface-raised);
}

.composer :deep(.el-textarea__inner) {
  background: var(--surface-sunken);
  border: 1px solid transparent;
  border-radius: var(--r-md);
  box-shadow: none;
  padding: 10px 14px;
  transition: border-color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease);
}

.composer :deep(.el-textarea__inner:focus) {
  background: var(--surface-raised);
  border-color: var(--border-strong);
  box-shadow: none;
}

.composer .el-button {
  border-radius: var(--r-pill);
  font-weight: 600;
  padding-left: 22px;
  padding-right: 22px;
}

.no-chat {
  margin: auto;
  color: var(--text-tertiary);
  font-size: 14px;
}
</style>
