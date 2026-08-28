<template>
  <div class="layout-xhs">
    <!-- 左侧导航 -->
    <aside class="side-nav card">
      <div class="brand" @click="router.push('/')"><span class="brand-inter">Inter</span><span class="brand-linked">Linked</span></div>

      <button class="nav-item" :class="{ active: isActive('/timeline') }" @click="router.push('/timeline')">
        <el-icon><Connection /></el-icon>
        <span>动态</span>
      </button>

      <button class="nav-item" :class="{ active: isActive('/publish') }" @click="router.push('/publish')">
        <el-icon><Plus /></el-icon>
        <span>发布</span>
      </button>

      <button class="nav-item" :class="{ active: isActive('/messages') }" @click="router.push('/messages')">
        <el-icon><ChatLineRound /></el-icon>
        <span>消息</span>
      </button>

      <button class="nav-item" :class="{ active: isActive('/notifications') }" @click="router.push('/notifications')">
        <el-icon><Bell /></el-icon>
        <span>通知</span>
        <span v-if="notificationUnread > 0" class="nav-badge">{{ notificationUnread > 99 ? '99+' : notificationUnread }}</span>
      </button>

      <button class="nav-item" :class="{ active: route.path.startsWith('/profile') }" @click="router.push(`/profile/${userStore.userInfo?.id}`)">
        <el-icon><User /></el-icon>
        <span>我</span>
      </button>

      <div class="side-bottom">
        <el-dropdown trigger="click" @command="handleSettingCommand" popper-class="setting-dropdown settings-popper" @visible-change="syncSettingsMenuWidth">
          <button ref="settingsTriggerRef" class="nav-item setting-item">
            <el-icon><Setting /></el-icon>
            <span>设置</span>
          </button>
          <template #dropdown>
            <el-dropdown-menu class="settings-menu" :style="{ width: `${settingsMenuWidth}px` }">
              <el-dropdown-item command="ops" class="settings-row">
                <span class="settings-left">
                  <el-icon><Monitor /></el-icon>
                  <span>运维面板</span>
                </span>
              </el-dropdown-item>
              <el-dropdown-item command="logout" class="settings-row">
                <span class="settings-left">
                  <el-icon><SwitchButton /></el-icon>
                  <span>退出登录</span>
                </span>
              </el-dropdown-item>
              <el-dropdown-item class="settings-row dark-mode-item" @click.stop>
                <span class="settings-left">
                  <el-icon><Setting /></el-icon>
                  <span>深色模式</span>
                </span>
                <el-switch v-model="isDarkMode" @change="toggleDarkMode" />
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </aside>

    <main class="content-wrap">
      <!-- 顶部全局搜索栏（小红书风格） -->
      <header class="top-bar">
        <div class="global-search">
          <el-icon class="search-icon"><Search /></el-icon>
          <input
            v-model="searchKeyword"
            class="search-input"
            type="text"
            placeholder="搜索用户、动态"
            @keyup.enter="goDiscoverSearch"
          />
          <button class="search-action" type="button" @click="goDiscoverSearch">搜索</button>
        </div>
      </header>

      <section class="router-section">
        <router-view />
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { notificationApi } from '../api'
import { ElMessageBox } from 'element-plus'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const searchKeyword = ref('')
const isDarkMode = ref(false)
const notificationUnread = ref(0)
const settingsTriggerRef = ref(null)
const settingsMenuWidth = ref(220)

const isActive = (path) => computed(() => route.path === path).value

function applyDarkMode(enabled) {
  document.body.classList.toggle('dark', enabled)
  localStorage.setItem('theme', enabled ? 'dark' : 'light')
}

function toggleDarkMode(value) {
  applyDarkMode(value)
}

function goDiscoverSearch() {
  const keyword = searchKeyword.value.trim()
  if (!keyword) {
    router.push('/search_result')
    return
  }
  router.push({ path: '/search_result', query: { keyword, type: 'users' } })
}

function handleSettingCommand(command) {
  if (command === 'ops') {
    router.push('/ops')
    return
  }

  if (command === 'logout') {
    ElMessageBox.confirm('确定退出登录吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }).then(() => {
      userStore.logout()
      router.push('/login')
    }).catch(() => {})
  }
}

function syncSettingsMenuWidth() {
  const triggerEl = settingsTriggerRef.value
  if (!triggerEl) return
  settingsMenuWidth.value = Math.max(180, Math.round(triggerEl.getBoundingClientRect().width))
}

onMounted(async () => {
  const savedTheme = localStorage.getItem('theme')
  const shouldUseDark = savedTheme ? savedTheme === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches
  isDarkMode.value = shouldUseDark
  applyDarkMode(shouldUseDark)
  syncSettingsMenuWidth()
  await refreshNotificationUnread()
})

watch(() => route.path, async (path) => {
  if (path === '/notifications') {
    notificationUnread.value = 0
    return
  }
  await refreshNotificationUnread()
})

async function refreshNotificationUnread() {
  try {
    const res = await notificationApi.getNotifications(1, 1)
    notificationUnread.value = Number(res.data.unread_count || 0)
  } catch {
    notificationUnread.value = 0
  }
}
</script>

<style scoped>
.layout-xhs {
  min-height: 100dvh;
  background: var(--layout-bg);
  display: grid;
  grid-template-columns: 232px minmax(0, 1fr);
  gap: 20px;
  padding: 20px;
}

/* 悬浮岛式侧导航 */
.side-nav {
  position: sticky;
  top: 20px;
  height: calc(100dvh - 40px);
  padding: 20px 12px;
  border-radius: var(--r-xl);
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-2);
  display: flex;
  flex-direction: column;
}

.brand {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.03em;
  padding: 8px 12px 20px;
  cursor: pointer;
}

.brand-inter {
  color: var(--brand-inter);
}

.brand-linked {
  color: var(--brand-linked);
}

.nav-item {
  width: 100%;
  border: 0;
  background: transparent;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 14px;
  border-radius: var(--r-pill);
  margin-bottom: 4px;
  cursor: pointer;
  color: var(--text-secondary);
  font-size: 15px;
  font-weight: 500;
  text-align: left;
  position: relative;
  transition: background var(--dur-fast) var(--ease), color var(--dur-fast) var(--ease);
}

.nav-item:hover {
  background: var(--nav-hover-bg);
  color: var(--text-primary);
}

.nav-item.active {
  background: var(--ink);
  color: var(--on-ink);
}

.nav-item.active:hover {
  color: var(--on-ink);
}

.nav-badge {
  margin-left: auto;
  min-width: 19px;
  height: 19px;
  line-height: 19px;
  border-radius: var(--r-pill);
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  text-align: center;
  padding: 0 5px;
}

.side-bottom {
  margin-top: auto;
}

.side-bottom :deep(.el-dropdown) {
  display: block;
  width: 100%;
}

.side-bottom :deep(.el-dropdown-link) {
  display: block;
  width: 100%;
}

.setting-item {
  margin-bottom: 0;
  width: 100%;
  justify-content: flex-start;
}

.content-wrap {
  min-width: 0;
}

.top-bar {
  margin-bottom: 16px;
  display: flex;
  justify-content: center;
}

/* 胶囊搜索条：白底悬浮，聚焦时描边加深 */
.global-search {
  width: min(640px, 100%);
  height: 48px;
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-pill);
  box-shadow: var(--shadow-1);
  display: grid;
  grid-template-columns: 22px 1fr auto;
  align-items: center;
  gap: 8px;
  padding: 0 8px 0 16px;
  transition: border-color var(--dur-fast) var(--ease), box-shadow var(--dur-fast) var(--ease);
}

.global-search:focus-within {
  border-color: var(--border-strong);
  box-shadow: var(--shadow-2);
}

.search-icon {
  color: var(--text-tertiary);
  font-size: 16px;
}

.search-input {
  border: 0;
  outline: none;
  height: 100%;
  font-size: 14px;
  color: var(--text-primary);
  background: transparent;
}

.search-input::placeholder {
  color: var(--text-tertiary);
}

.search-action {
  border: 0;
  height: 36px;
  padding: 0 18px;
  border-radius: var(--r-pill);
  background: var(--ink);
  color: var(--on-ink);
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  transition: transform var(--dur-fast) var(--ease), opacity var(--dur-fast) var(--ease);
}

.search-action:hover {
  opacity: 0.88;
}

.search-action:active {
  transform: scale(0.97);
}

.router-section {
  min-width: 0;
}

:deep(.settings-popper .el-dropdown-menu) {
  min-width: 180px;
  padding: 6px;
}

:deep(.settings-popper .el-dropdown-menu__item.settings-row) {
  width: 100%;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 40px;
  border-radius: 10px;
  padding: 0 12px;
}

:deep(.settings-popper .settings-left) {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text-color);
  flex: 1;
  min-width: 0;
}

:deep(.settings-popper .settings-left .el-icon) {
  font-size: 15px;
}

:deep(.settings-popper .dark-mode-item .el-switch) {
  margin-left: auto;
}

@media (max-width: 960px) {
  .layout-xhs {
    grid-template-columns: 1fr;
  }

  .side-nav {
    position: static;
    height: auto;
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 8px;
  }

  .side-bottom {
    margin-top: 0;
  }

  .brand {
    grid-column: 1 / -1;
    padding-bottom: 8px;
  }

  .nav-item {
    justify-content: center;
  }

  .setting-item {
    justify-content: center;
  }

  .global-search {
    width: 100%;
  }
}
</style>
