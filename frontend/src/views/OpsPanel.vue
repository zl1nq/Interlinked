<template>
  <div class="ops-page">
    <section class="card ops-head">
      <div>
        <h2>运维面板</h2>
        <p>MQ 与缓存实时指标</p>
      </div>
      <div class="actions">
        <el-switch v-model="autoRefresh" active-text="自动刷新" />
        <el-button :loading="loading" @click="loadMetrics">立即刷新</el-button>
      </div>
    </section>

    <section class="card tabs-wrap">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="MQ 指标" name="mq" />
        <el-tab-pane label="缓存指标" name="cache" />
      </el-tabs>
    </section>

    <section class="grid" v-if="activeTab === 'mq'">
      <div class="card metric" v-for="item in mqMetricCards" :key="item.key">
        <div class="label">{{ item.label }}</div>
        <div class="value">{{ item.value }}</div>
      </div>
    </section>

    <section class="grid" v-else>
      <div class="card metric" v-for="item in cacheMetricCards" :key="item.key">
        <div class="label">{{ item.label }}</div>
        <div class="value">{{ item.value }}</div>
      </div>
    </section>

    <section class="card raw-wrap">
      <div class="raw-title">原始 JSON（{{ activeTab.toUpperCase() }}）</div>
      <pre>{{ JSON.stringify(activeTab === 'mq' ? mqMetrics : cacheMetrics, null, 2) }}</pre>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { opsApi } from '../api'

const loading = ref(false)
const autoRefresh = ref(false)
const activeTab = ref('mq')
let timer = null

const mqMetrics = ref({})
const cacheMetrics = ref({})

const mqMetricCards = computed(() => [
  { key: 'circuit_open', label: '熔断状态(0/1)', value: mqMetrics.value.circuit_open ?? 0 },
  { key: 'circuit_open_count', label: '熔断开启次数', value: mqMetrics.value.circuit_open_count ?? 0 },
  { key: 'circuit_close_count', label: '熔断关闭次数', value: mqMetrics.value.circuit_close_count ?? 0 },
  { key: 'degrade_count', label: '降级次数(outbox only)', value: mqMetrics.value.degrade_count ?? 0 },
  { key: 'publish_fail_count', label: '发布失败次数', value: mqMetrics.value.publish_fail_count ?? 0 },
  { key: 'retry_count', label: '重试次数', value: mqMetrics.value.retry_count ?? 0 },
  { key: 'dlq_count', label: '死信次数', value: mqMetrics.value.dlq_count ?? 0 },
  { key: 'dispatch_error_count', label: '分发错误次数', value: mqMetrics.value.dispatch_error_count ?? 0 },
])

const cacheMetricCards = computed(() => [
  { key: 'feed_cache_hit', label: 'Feed 缓存命中', value: cacheMetrics.value.feed_cache_hit ?? 0 },
  { key: 'feed_cache_miss', label: 'Feed 缓存未命中', value: cacheMetrics.value.feed_cache_miss ?? 0 },
  { key: 'feed_cache_hit_ratio_bp', label: 'Feed 命中率(‰‰)', value: cacheMetrics.value.feed_cache_hit_ratio_bp ?? 0 },
  { key: 'feed_cache_delete', label: 'Feed 缓存删除次数', value: cacheMetrics.value.feed_cache_delete ?? 0 },
  { key: 'user_cache_hit', label: 'User 缓存命中', value: cacheMetrics.value.user_cache_hit ?? 0 },
  { key: 'user_cache_miss', label: 'User 缓存未命中', value: cacheMetrics.value.user_cache_miss ?? 0 },
  { key: 'user_cache_hit_ratio_bp', label: 'User 命中率(‰‰)', value: cacheMetrics.value.user_cache_hit_ratio_bp ?? 0 },
  { key: 'user_cache_delete', label: 'User 缓存删除次数', value: cacheMetrics.value.user_cache_delete ?? 0 },
])

onMounted(async () => {
  await loadMetrics()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})

watch(autoRefresh, (v) => {
  if (v) startAutoRefresh()
  else stopAutoRefresh()
})

function startAutoRefresh() {
  stopAutoRefresh()
  if (!autoRefresh.value) return
  timer = setInterval(loadMetrics, 5000)
}

function stopAutoRefresh() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

async function loadMetrics() {
  loading.value = true
  try {
    const [mqRes, cacheRes] = await Promise.all([
      opsApi.getMQMetrics(),
      opsApi.getCacheMetrics(),
    ])
    mqMetrics.value = mqRes.data || {}
    cacheMetrics.value = cacheRes.data || {}
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.ops-page { min-width: 0; }
.ops-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  border-radius: 14px;
}
.ops-head h2 { margin: 0; }
.ops-head p { margin: 4px 0 0; color: #95a0b5; font-size: 13px; }
.actions { display: inline-flex; align-items: center; gap: 10px; }
.tabs-wrap { margin-bottom: 12px; border-radius: 12px; padding-bottom: 4px; }
.grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}
.metric { border-radius: 12px; }
.label { color: #8f99ad; font-size: 12px; }
.value { margin-top: 6px; font-size: 24px; font-weight: 700; }
.raw-wrap { margin-top: 12px; border-radius: 12px; }
.raw-title { font-weight: 600; margin-bottom: 6px; }
pre {
  margin: 0;
  background: #f6f8fb;
  border-radius: 8px;
  padding: 10px;
  overflow: auto;
}
@media (max-width: 1100px) {
  .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
