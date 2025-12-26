<template>
  <div class="logs-page">
    <el-card>
      <template #header>
        <div class="logs-header">
          <div class="title">
            实时日志
            <span class="status" :class="{ connected: connected }">
              {{ connected ? '已连接' : '未连接' }}
            </span>
          </div>
          <div class="controls">
            <el-select
              v-model="selectedSources"
              multiple
              collapse-tags
              collapse-tags-tooltip
              placeholder="选择日志源"
              style="width: 260px"
              @change="handleSourcesChange"
            >
              <el-option
                v-for="s in sources"
                :key="s.name"
                :label="s.name + (s.available ? '' : ' (不可用)')"
                :value="s.name"
                :disabled="!s.available"
              />
            </el-select>
            <el-input v-model="filter" clearable placeholder="过滤关键词" style="width: 220px" />
            <el-switch v-model="autoScroll" active-text="自动滚动" />
            <el-button @click="clearLogs">清空</el-button>
            <el-button type="primary" :loading="connecting" @click="reconnect">重连</el-button>
          </div>
        </div>
      </template>

      <el-scrollbar ref="scrollbarRef" height="70vh">
        <div class="log-list">
          <div v-for="(l, idx) in filteredLogs" :key="idx" class="log-line">
            <span class="ts">{{ formatTs(l.timestamp) }}</span>
            <span class="src">[{{ l.source }}]</span>
            <span class="msg">{{ l.message }}</span>
          </div>
        </div>
      </el-scrollbar>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/auth'

type LogSource = { name: string; available: boolean }
type LogEvent = { type: 'log' | 'ping'; timestamp: string; source?: string; message?: string }

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api'

const sources = ref<LogSource[]>([])
const selectedSources = ref<string[]>(['backend_out', 'backend_err'])

const logs = ref<LogEvent[]>([])
const connected = ref(false)
const connecting = ref(false)
const filter = ref('')
const autoScroll = ref(true)
const scrollbarRef = ref<any>(null)

let abortController: AbortController | null = null

const filteredLogs = computed(() => {
  const q = filter.value.trim()
  if (!q) return logs.value
  return logs.value.filter((l) => (l.message || '').includes(q) || (l.source || '').includes(q))
})

const formatTs = (ts: string) => {
  if (!ts) return ''
  try {
    return new Date(ts).toLocaleString()
  } catch {
    return ts
  }
}

const clearLogs = () => {
  logs.value = []
}

const scrollToBottom = async () => {
  if (!autoScroll.value) return
  await nextTick()
  scrollbarRef.value?.setScrollTop?.(Number.MAX_SAFE_INTEGER)
}

const appendLog = (evt: LogEvent) => {
  if (evt.type !== 'log') return
  logs.value.push(evt)
  if (logs.value.length > 2000) {
    logs.value.splice(0, logs.value.length - 2000)
  }
  scrollToBottom()
}

const stopStream = () => {
  abortController?.abort()
  abortController = null
  connected.value = false
}

const loadSources = async () => {
  const res = await api.get('/logs/sources')
  if (res.data?.success && res.data.data) {
    sources.value = res.data.data
    // Default to available backend logs if possible
    const available = new Set(sources.value.filter((s) => s.available).map((s) => s.name))
    selectedSources.value = selectedSources.value.filter((s) => available.has(s))
    if (selectedSources.value.length === 0) {
      selectedSources.value = sources.value.filter((s) => s.available).slice(0, 2).map((s) => s.name)
    }
  }
}

const loadTail = async () => {
  if (selectedSources.value.length === 0) return
  const res = await api.get('/logs/tail', {
    params: { sources: selectedSources.value.join(','), lines: 200 },
  })
  if (res.data?.success && Array.isArray(res.data.data)) {
    for (const evt of res.data.data as LogEvent[]) {
      appendLog(evt)
    }
  }
}

const startStream = async () => {
  stopStream()

  if (selectedSources.value.length === 0) {
    ElMessage.warning('请选择至少一个日志源')
    return
  }

  const token = localStorage.getItem('token')
  if (!token) {
    ElMessage.error('未登录，无法查看日志')
    return
  }

  connecting.value = true
  abortController = new AbortController()

  try {
    const url = `${API_BASE_URL}/logs/stream?sources=${encodeURIComponent(selectedSources.value.join(','))}`
    const res = await fetch(url, {
      method: 'GET',
      headers: { Authorization: `Bearer ${token}` },
      signal: abortController.signal,
    })

    if (!res.ok) {
      const text = await res.text()
      throw new Error(text || `HTTP ${res.status}`)
    }
    if (!res.body) {
      throw new Error('浏览器不支持流式读取')
    }

    connected.value = true
    const reader = res.body.getReader()
    const decoder = new TextDecoder('utf-8')
    let buffer = ''

    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      let idx = buffer.indexOf('\n')
      while (idx >= 0) {
        const line = buffer.slice(0, idx).trim()
        buffer = buffer.slice(idx + 1)
        idx = buffer.indexOf('\n')
        if (!line) continue
        try {
          const evt = JSON.parse(line) as LogEvent
          appendLog(evt)
        } catch {
          appendLog({ type: 'log', timestamp: new Date().toISOString(), source: 'parse', message: line })
        }
      }
    }
  } catch (e: unknown) {
    if ((e as any)?.name !== 'AbortError') {
      const msg = e instanceof Error ? e.message : String(e)
      ElMessage.error(`日志连接失败：${msg}`)
    }
  } finally {
    connecting.value = false
    connected.value = false
  }
}

const reconnect = async () => {
  clearLogs()
  await loadTail()
  await startStream()
}

const handleSourcesChange = async () => {
  await reconnect()
}

onMounted(async () => {
  try {
    await loadSources()
    await loadTail()
    await startStream()
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    ElMessage.error(`加载日志失败：${msg}`)
  }
})

onBeforeUnmount(() => {
  stopStream()
})
</script>

<style scoped>
.logs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.title {
  font-size: 16px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 10px;
}

.status {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.06);
  color: rgba(0, 0, 0, 0.6);
}

.status.connected {
  background: rgba(82, 196, 26, 0.14);
  color: #389e0d;
}

.controls {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.log-list {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  line-height: 1.6;
  padding: 8px;
}

.log-line {
  display: flex;
  gap: 10px;
  white-space: pre-wrap;
  word-break: break-word;
}

.ts {
  flex: 0 0 auto;
  color: rgba(0, 0, 0, 0.45);
}

.src {
  flex: 0 0 auto;
  color: rgba(0, 0, 0, 0.65);
}

.msg {
  flex: 1 1 auto;
}
</style>

