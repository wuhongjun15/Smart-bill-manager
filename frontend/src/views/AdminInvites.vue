<template>
  <div class="page">
    <Card class="sbm-surface">
      <template #title>
        <div class="header">
          <span>邀请码管理</span>
          <div class="toolbar">
            <Dropdown
              v-model="expiresInDays"
              :options="expiresOptions"
              optionLabel="label"
              optionValue="value"
              class="expires-dropdown"
            />
            <Button icon="pi pi-plus" label="生成邀请码" @click="createInvite" />
          </div>
        </div>
      </template>
      <template #content>
        <Message v-if="!isAdmin" severity="warn" :closable="false">仅管理员可访问</Message>

        <div v-else class="content">
          <div v-if="lastCode" class="last-code">
            <div class="last-code-title">最新邀请码（只显示一次，请及时保存）</div>
            <div class="last-code-row">
              <span class="last-code-value">{{ lastCode }}</span>
              <Button class="p-button-outlined" icon="pi pi-copy" label="复制" @click="copyLastCode" />
            </div>
            <small v-if="lastExpiresHint" class="muted">{{ lastExpiresHint }}</small>
          </div>

          <DataTable
            class="invites-table"
            :value="invites"
            :loading="loading"
            responsiveLayout="scroll"
            :paginator="true"
            :rows="20"
            :rowsPerPageOptions="[10, 20, 50]"
          >
            <Column field="code_hint" header="标识" :style="{ width: '18%' }" />
            <Column field="createdAt" header="生成时间" :style="{ width: '22%' }">
              <template #body="{ data: row }">{{ formatDateTime(row.createdAt) }}</template>
            </Column>
            <Column field="expiresAt" header="过期时间" :style="{ width: '22%' }">
              <template #body="{ data: row }">
                <span v-if="row.expiresAt">{{ formatDateTime(row.expiresAt) }}</span>
                <span v-else class="muted">永不过期</span>
              </template>
            </Column>
            <Column header="状态" :style="{ width: '18%' }">
              <template #body="{ data: row }">
                <Tag v-if="row.usedAt" severity="secondary" value="已使用" />
                <Tag v-else-if="row.expired" severity="danger" value="已过期" />
                <Tag v-else severity="success" value="可使用" />
              </template>
            </Column>
            <Column field="usedAt" header="使用时间" :style="{ width: '20%' }">
              <template #body="{ data: row }">
                <span v-if="row.usedAt">{{ formatDateTime(row.usedAt) }}</span>
                <span v-else class="muted">-</span>
              </template>
            </Column>
          </DataTable>
        </div>
      </template>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Card from 'primevue/card'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dropdown from 'primevue/dropdown'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { useToast } from 'primevue/usetoast'
import dayjs from 'dayjs'
import { authApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

type InviteRow = {
  id: string
  code_hint: string
  createdBy: string
  createdAt: string
  expiresAt?: string | null
  usedAt?: string | null
  usedBy?: string | null
  expired: boolean
}

const toast = useToast()
const authStore = useAuthStore()

const isAdmin = computed(() => authStore.user?.role === 'admin')

const loading = ref(false)
const invites = ref<InviteRow[]>([])

const expiresInDays = ref<number>(7)
const expiresOptions = [
  { label: '7 天过期', value: 7 },
  { label: '30 天过期', value: 30 },
  { label: '永不过期', value: 0 },
]

const lastCode = ref('')
const lastExpiresHint = ref('')

const formatDateTime = (v?: string | null) => {
  if (!v) return '-'
  const d = dayjs(v)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : v
}

const loadInvites = async () => {
  if (!isAdmin.value) return
  loading.value = true
  try {
    const res = await authApi.adminListInvites(50)
    if (res.data.success && res.data.data) {
      invites.value = res.data.data as any
      return
    }
    toast.add({ severity: 'error', summary: res.data.message || '获取邀请码失败', life: 3000 })
  } catch (e: any) {
    toast.add({ severity: 'error', summary: e.response?.data?.message || '获取邀请码失败', life: 3000 })
  } finally {
    loading.value = false
  }
}

const createInvite = async () => {
  if (!isAdmin.value) return
  loading.value = true
  try {
    const res = await authApi.adminCreateInvite(expiresInDays.value)
    if (res.data.success && res.data.data) {
      lastCode.value = res.data.data.code
      const exp = res.data.data.expiresAt
      lastExpiresHint.value = exp ? `有效期至：${formatDateTime(exp)}` : '有效期：永不过期'
      toast.add({ severity: 'success', summary: '邀请码已生成', life: 2000 })
      await loadInvites()
      return
    }
    toast.add({ severity: 'error', summary: res.data.message || '生成邀请码失败', life: 3000 })
  } catch (e: any) {
    toast.add({ severity: 'error', summary: e.response?.data?.message || '生成邀请码失败', life: 3000 })
  } finally {
    loading.value = false
  }
}

const copyLastCode = async () => {
  if (!lastCode.value) return
  try {
    await navigator.clipboard.writeText(lastCode.value)
    toast.add({ severity: 'success', summary: '已复制到剪贴板', life: 1600 })
  } catch {
    toast.add({ severity: 'warn', summary: '复制失败，请手动复制', life: 2500 })
  }
}

onMounted(() => {
  loadInvites()
})
</script>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.expires-dropdown {
  min-width: 140px;
}

.content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.last-code {
  border: 1px dashed color-mix(in srgb, var(--p-surface-200), transparent 15%);
  border-radius: var(--radius-md);
  padding: 12px;
  background: color-mix(in srgb, var(--p-surface-0), transparent 0%);
}

.last-code-title {
  font-weight: 900;
  color: var(--p-text-color);
  margin-bottom: 8px;
}

.last-code-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.last-code-value {
  font-family: var(--font-mono);
  font-weight: 800;
  letter-spacing: 0.5px;
}

.muted {
  color: var(--p-text-muted-color);
}

.invites-table :deep(.p-datatable-table) {
  width: 100%;
  table-layout: fixed;
}

.invites-table :deep(.p-datatable-thead > tr > th),
.invites-table :deep(.p-datatable-tbody > tr > td) {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 768px) {
  .invites-table :deep(.p-datatable-table) {
    width: max-content;
    min-width: 100%;
  }
}
</style>
