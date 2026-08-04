<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-none space-y-4">
      <header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-center gap-3">
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('channelStatus.title') }}
          </h1>
          <span
            class="inline-flex h-7 items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-2.5 text-xs font-medium text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-900/20 dark:text-emerald-300"
          >
            <span
              class="h-1.5 w-1.5 rounded-full bg-emerald-500"
              :class="{ 'animate-pulse': loading }"
            />
            {{ loading ? t('channelStatus.refreshing') : t('channelStatus.monitoring') }}
          </span>
        </div>

        <div class="flex items-center gap-2 self-start sm:self-auto">
          <span v-if="lastUpdatedAt" class="hidden text-xs text-gray-400 sm:inline">
            {{ t('channelStatus.updatedAt', { time: formatTime(lastUpdatedAt) }) }}
          </span>
          <button
            type="button"
            class="btn btn-secondary inline-flex h-9 items-center gap-2"
            :disabled="loading"
            :title="t('channelStatus.refresh')"
            @click="manualReload"
          >
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            {{ t('channelStatus.refresh') }}
          </button>
        </div>
      </header>

      <div class="flex flex-col gap-3 border-b border-gray-200 pb-4 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700">
        <div
          role="tablist"
          :aria-label="t('channelStatus.timeWindow')"
          class="grid w-full grid-cols-4 rounded-lg border border-gray-200 bg-white p-1 shadow-sm sm:w-[420px] dark:border-dark-700 dark:bg-dark-900"
        >
          <button
            v-for="window in windows"
            :key="window.value"
            type="button"
            role="tab"
            :aria-selected="selectedWindow === window.value"
            class="h-9 rounded-md px-3 text-sm font-medium transition-colors"
            :class="selectedWindow === window.value
              ? 'bg-primary-600 text-white shadow-sm'
              : 'text-gray-500 hover:bg-gray-50 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white'"
            @click="selectWindow(window.value)"
          >
            {{ window.label }}
          </button>
        </div>

        <div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-gray-500 dark:text-dark-400">
          <span class="inline-flex items-center gap-1.5"><span class="h-2 w-2 rounded-sm bg-emerald-500" />{{ t('monitorCommon.status.operational') }}</span>
          <span class="inline-flex items-center gap-1.5"><span class="h-2 w-2 rounded-sm bg-amber-500" />{{ t('monitorCommon.status.degraded') }}</span>
          <span class="inline-flex items-center gap-1.5"><span class="h-2 w-2 rounded-sm bg-red-500" />{{ t('monitorCommon.status.failed') }}</span>
          <span class="inline-flex items-center gap-1.5"><span class="h-2 w-2 rounded-sm bg-gray-300 dark:bg-dark-600" />{{ t('channelStatus.noRecord') }}</span>
        </div>
      </div>

      <section class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div v-if="loading && rows.length === 0" class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="index in 5" :key="index" class="flex h-28 animate-pulse items-center gap-6 px-5">
            <div class="h-10 w-40 rounded bg-gray-100 dark:bg-dark-800" />
            <div class="h-8 w-20 rounded bg-gray-100 dark:bg-dark-800" />
            <div class="h-14 flex-1 rounded bg-gray-100 dark:bg-dark-800" />
            <div class="h-8 w-56 rounded bg-gray-100 dark:bg-dark-800" />
          </div>
        </div>

        <template v-else>
          <div class="hidden w-full lg:block">
            <table class="w-full table-fixed text-left text-sm">
              <thead class="border-b border-gray-200 bg-gray-50 text-xs font-medium text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-dark-400">
                <tr>
                  <th class="w-[200px] px-3 py-3">{{ t('channelStatus.columns.group') }}</th>
                  <th class="w-[90px] px-3 py-3" :aria-sort="sortAria('rate')">
                    <button type="button" class="inline-flex items-center gap-1.5 font-medium hover:text-gray-900 dark:hover:text-white" data-sort="rate" @click="toggleSort('rate')">
                      {{ t('channelStatus.columns.rate') }}
                      <Icon :name="sortIcon('rate')" size="xs" />
                    </button>
                  </th>
                  <th class="w-[250px] px-3 py-3">{{ t('channelStatus.columns.models') }}</th>
                  <th class="w-[115px] px-3 py-3" :aria-sort="sortAria('status')">
                    <button type="button" class="inline-flex items-center gap-1.5 font-medium hover:text-gray-900 dark:hover:text-white" data-sort="status" @click="toggleSort('status')">
                      {{ t('channelStatus.columns.status') }}
                      <Icon :name="sortIcon('status')" size="xs" />
                    </button>
                  </th>
                  <th class="w-[95px] px-3 py-3" :aria-sort="sortAria('dialog_latency')">
                    <button type="button" class="inline-flex items-center gap-1.5 font-medium hover:text-gray-900 dark:hover:text-white" data-sort="dialog_latency" @click="toggleSort('dialog_latency')">
                      {{ t('channelStatus.dialogLatency') }}
                      <Icon :name="sortIcon('dialog_latency')" size="xs" />
                    </button>
                  </th>
                  <th class="w-[95px] px-3 py-3" :aria-sort="sortAria('endpoint_ping')">
                    <button type="button" class="inline-flex items-center gap-1.5 font-medium hover:text-gray-900 dark:hover:text-white" data-sort="endpoint_ping" @click="toggleSort('endpoint_ping')">
                      {{ t('channelStatus.endpointPing') }}
                      <Icon :name="sortIcon('endpoint_ping')" size="xs" />
                    </button>
                  </th>
                  <th class="w-[100px] px-3 py-3" :aria-sort="sortAria('availability')">
                    <button type="button" class="inline-flex items-center gap-1.5 font-medium hover:text-gray-900 dark:hover:text-white" data-sort="availability" @click="toggleSort('availability')">
                      {{ t('channelStatus.availability') }}
                      <Icon :name="sortIcon('availability')" size="xs" />
                    </button>
                  </th>
                  <th class="w-[110px] px-3 py-3" :aria-sort="sortAria('cache_hit_rate')">
                    <button type="button" class="inline-flex items-center gap-1.5 font-medium hover:text-gray-900 dark:hover:text-white" data-sort="cache_hit_rate" @click="toggleSort('cache_hit_rate')">
                      {{ t('channelStatus.cacheHit') }}
                      <Icon :name="sortIcon('cache_hit_rate')" size="xs" />
                    </button>
                  </th>
                  <th class="w-[300px] px-3 py-3">
                    <span class="inline-flex items-center gap-2">
                      <span>{{ t('channelStatus.columns.history') }}</span>
                      <span class="text-[10px] font-normal text-gray-400 dark:text-dark-500">
                        {{ t('monitorCommon.history60pts', { n: 60 }) }}
                      </span>
                    </span>
                  </th>
                  <th class="w-[145px] px-3 py-3 text-right">{{ t('channelStatus.columns.actions') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr
                  v-for="row in rows"
                  :key="row.group.id"
                  :data-group-id="row.group.id"
                  class="align-top transition-colors hover:bg-gray-50/70 dark:hover:bg-dark-800/40"
                >
                  <td class="px-4 py-4">
                    <div class="font-semibold text-gray-900 dark:text-white">{{ row.group.name }}</div>
                    <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400">
                      <span>{{ providerLabel(row.group.platform) }}</span>
                      <span aria-hidden="true">·</span>
                      <span>{{ row.group.is_exclusive ? t('channelStatus.exclusive') : t('channelStatus.public') }}</span>
                    </div>
                  </td>

                  <td class="px-4 py-4">
                    <div class="font-mono text-base font-semibold text-gray-900 dark:text-white">
                      {{ formatRate(row.group.effective_rate) }}x
                    </div>
                  </td>

                  <td class="px-4 py-4">
                    <template v-if="row.monitor">
                      <div class="flex flex-wrap gap-1.5">
                        <span
                          v-for="model in monitorModels(row.monitor)"
                          :key="model.model"
                          :data-model="model.model"
                          :data-model-status="model.status"
                          class="inline-flex items-center gap-1 rounded border border-gray-200 px-1.5 py-1 text-[10px] text-gray-600 dark:border-dark-600 dark:text-dark-300"
                        >
                          <span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass(model.status)" />
                          {{ model.model }}
                        </span>
                      </div>
                    </template>
                    <span v-else class="text-xs text-gray-400">{{ t('channelStatus.noMonitor') }}</span>
                  </td>

                  <td class="px-4 py-4">
                    <div class="flex flex-col items-start gap-2">
                      <span
                        class="inline-flex rounded-full px-2.5 py-1 text-xs font-medium"
                        :class="statusBadgeClass(groupMonitorStatus(row.monitor))"
                        :data-group-status="groupMonitorStatus(row.monitor) || 'unknown'"
                      >
                        {{ row.monitor ? statusLabel(groupMonitorStatus(row.monitor)) : t('channelStatus.noRecord') }}
                      </span>
                      <span class="text-xs text-gray-400">{{ latestProbeText(row) }}</span>
                    </div>
                  </td>

                  <td class="px-4 py-4">
                    <span class="font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(row.monitor?.primary_latency_ms, 'ms') }}</span>
                  </td>

                  <td class="px-4 py-4">
                    <span class="font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(row.monitor?.primary_ping_latency_ms, 'ms') }}</span>
                  </td>

                  <td class="px-4 py-4">
                    <span class="font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(row.group.metrics.availability, '%', 1) }}</span>
                  </td>

                  <td class="px-3 py-4">
                    <span class="font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(row.group.metrics.cache_hit_rate, '%', 1) }}</span>
                  </td>

                  <td class="px-3 py-2.5">
                    <MonitorTimeline
                      v-if="row.monitor?.timeline.length"
                      :buckets="row.monitor.timeline"
                      :countdown-seconds="countdown"
                      compact
                    />
                    <div v-else class="flex h-14 items-center justify-center border-b border-dashed border-gray-200 text-xs text-gray-400 dark:border-dark-600">
                      {{ t('channelStatus.noRecord') }}
                    </div>
                    <div v-if="latestRecord(row.monitor)" class="mt-0.5 truncate text-[11px] text-gray-400">
                      {{ latestRecordText(row.monitor) }}
                    </div>
                  </td>

                  <td class="px-3 py-4 text-right">
                    <div class="flex flex-col items-end gap-2">
                      <button
                        v-if="row.monitor"
                        type="button"
                        class="inline-flex items-center gap-1.5 text-xs font-medium text-gray-600 hover:text-primary-700 dark:text-dark-300 dark:hover:text-primary-300"
                        @click="openDetail(row.monitor)"
                      >
                        <Icon name="eye" size="xs" />
                        {{ t('channelStatus.monitorDetail') }}
                      </button>
                      <RouterLink
                        :to="{ path: '/keys', query: { group: String(row.group.id) } }"
                        class="btn btn-secondary btn-sm inline-flex items-center gap-1.5 whitespace-nowrap text-primary-700 dark:text-primary-300"
                      >
                        <Icon name="swap" size="xs" />
                        {{ t('channelStatus.useGroup') }}
                      </RouterLink>
                    </div>
                  </td>
                </tr>

                <tr v-if="rows.length === 0">
                  <td colspan="10" class="px-5 py-20 text-center text-sm text-gray-400">
                    {{ t('channelStatus.empty.title') }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="divide-y divide-gray-100 lg:hidden dark:divide-dark-700">
            <article v-for="row in rows" :key="row.group.id" class="space-y-4 p-4 sm:p-5">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <h2 class="truncate font-semibold text-gray-900 dark:text-white">{{ row.group.name }}</h2>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ providerLabel(row.group.platform) }} · {{ row.group.is_exclusive ? t('channelStatus.exclusive') : t('channelStatus.public') }}
                  </p>
                </div>
                <div class="text-right">
                  <div class="font-mono text-base font-semibold text-gray-900 dark:text-white">{{ formatRate(row.group.effective_rate) }}x</div>
                  <span
                    class="mt-1 inline-flex rounded-full px-2 py-0.5 text-[10px] font-medium"
                    :class="statusBadgeClass(groupMonitorStatus(row.monitor))"
                    :data-group-status="groupMonitorStatus(row.monitor) || 'unknown'"
                  >
                    {{ row.monitor ? statusLabel(groupMonitorStatus(row.monitor)) : t('channelStatus.noRecord') }}
                  </span>
                </div>
              </div>

              <div v-if="row.monitor" class="flex flex-wrap gap-1.5">
                <span
                  v-for="model in monitorModels(row.monitor)"
                  :key="model.model"
                  :data-model="model.model"
                  :data-model-status="model.status"
                  class="inline-flex items-center gap-1 rounded border border-gray-200 px-1.5 py-1 text-[10px] text-gray-600 dark:border-dark-600 dark:text-dark-300"
                >
                  <span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass(model.status)" />
                  {{ model.model }}
                </span>
              </div>

              <dl class="grid grid-cols-2 gap-x-3 gap-y-4 border-y border-gray-100 py-4 text-xs dark:border-dark-700 sm:grid-cols-4">
                <div><dt class="text-gray-400">{{ t('channelStatus.dialogLatency') }}</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(row.monitor?.primary_latency_ms, 'ms') }}</dd></div>
                <div><dt class="text-gray-400">{{ t('channelStatus.endpointPing') }}</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(row.monitor?.primary_ping_latency_ms, 'ms') }}</dd></div>
                <div><dt class="text-gray-400">{{ t('channelStatus.availability') }}</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(row.group.metrics.availability, '%', 1) }}</dd></div>
                <div><dt class="text-gray-400">{{ t('channelStatus.cacheHit') }}</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(row.group.metrics.cache_hit_rate, '%', 1) }}</dd></div>
              </dl>

              <MonitorTimeline
                v-if="row.monitor?.timeline.length"
                :buckets="row.monitor.timeline"
                :countdown-seconds="countdown"
              />
              <div v-else class="border-b border-dashed border-gray-200 py-4 text-center text-xs text-gray-400 dark:border-dark-600">
                {{ t('channelStatus.noRecord') }}
              </div>

              <div class="flex items-center justify-between gap-3">
                <button
                  v-if="row.monitor"
                  type="button"
                  class="inline-flex items-center gap-1.5 text-xs font-medium text-gray-600 dark:text-dark-300"
                  @click="openDetail(row.monitor)"
                >
                  <Icon name="eye" size="xs" />
                  {{ t('channelStatus.monitorDetail') }}
                </button>
                <span v-else />
                <RouterLink
                  :to="{ path: '/keys', query: { group: String(row.group.id) } }"
                  class="btn btn-secondary btn-sm inline-flex items-center gap-1.5 whitespace-nowrap text-primary-700 dark:text-primary-300"
                >
                  <Icon name="swap" size="xs" />
                  {{ t('channelStatus.useGroup') }}
                </RouterLink>
              </div>
            </article>

            <div v-if="rows.length === 0" class="px-5 py-20 text-center text-sm text-gray-400">
              {{ t('channelStatus.empty.title') }}
            </div>
          </div>
        </template>
      </section>
    </div>

    <MonitorDetailDialog
      :show="showDetail"
      :monitor-id="detailTarget?.id ?? null"
      :title="detailTarget?.group_name || t('channelStatus.detailTitle')"
      @close="closeDetail"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import MonitorDetailDialog from '@/components/user/MonitorDetailDialog.vue'
import MonitorTimeline from '@/components/user/monitor/MonitorTimeline.vue'
import { supplierAPI, type HallGroup } from '@/api/suppliers'
import {
  list as listChannelMonitorViews,
  type MonitorStatus,
  type UserMonitorView,
} from '@/api/channelMonitor'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { DEFAULT_INTERVAL_SECONDS } from '@/constants/channelMonitor'

type HallWindow = '6h' | '24h' | '7d' | '30d'

interface HallRow {
  group: HallGroup
  monitor: UserMonitorView | null
}

interface MonitorModelStatus {
  model: string
  status: MonitorStatus
  latency_ms: number | null
}

const { t } = useI18n()
const appStore = useAppStore()
const { statusLabel, statusBadgeClass, providerLabel, formatRelativeTime } = useChannelMonitorFormat()

const windows: Array<{ value: HallWindow; label: string }> = [
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
]

const selectedWindow = ref<HallWindow>('6h')
type SortKey = 'rate' | 'status' | 'dialog_latency' | 'endpoint_ping' | 'cache_hit_rate' | 'availability'
const sortKey = ref<SortKey | null>(null)
const sortDirection = ref<'asc' | 'desc'>('desc')
const groups = ref<HallGroup[]>([])
const monitors = ref<UserMonitorView[]>([])
const loading = ref(true)
const lastUpdatedAt = ref<Date | null>(null)
const showDetail = ref(false)
const detailTarget = ref<UserMonitorView | null>(null)
let loadSequence = 0

const monitorByGroup = computed(() => {
  const indexed = new Map<string, UserMonitorView>()
  for (const monitor of monitors.value) {
    const key = normalizeGroupName(monitor.group_name)
    if (key && !indexed.has(key)) indexed.set(key, monitor)
  }
  return indexed
})

const rows = computed<HallRow[]>(() => {
  const mapped = groups.value.map(group => ({
    group,
    monitor: monitorByGroup.value.get(normalizeGroupName(group.name)) || null,
  }))
  if (!sortKey.value) return mapped

  const key = sortKey.value
  return [...mapped].sort((left, right) => {
    const leftValue = sortValue(left, key)
    const rightValue = sortValue(right, key)
    if (leftValue == null && rightValue == null) return left.group.name.localeCompare(right.group.name)
    if (leftValue == null) return 1
    if (rightValue == null) return -1
    const result = sortDirection.value === 'asc' ? leftValue - rightValue : rightValue - leftValue
    return result || left.group.name.localeCompare(right.group.name)
  })
})

const autoRefresh = useAutoRefresh({
  storageKey: 'supplier-hall-auto-refresh',
  intervals: [30, 60, 120] as const,
  defaultInterval: DEFAULT_INTERVAL_SECONDS,
  onRefresh: () => load(true),
  shouldPause: () => document.hidden || loading.value,
})
const countdown = autoRefresh.countdown

function normalizeGroupName(value: string) {
  return value.trim().toLocaleLowerCase()
}

async function load(silent = false) {
  const sequence = ++loadSequence
  if (!silent) loading.value = true
  try {
    const [hallResponse, monitorResponse] = await Promise.all([
      supplierAPI.hall(selectedWindow.value),
      listChannelMonitorViews(),
    ])
    if (sequence !== loadSequence) return
    groups.value = hallResponse.groups || []
    monitors.value = monitorResponse.items || []
    lastUpdatedAt.value = new Date()
    autoRefresh.resetCountdown()
  } catch (error) {
    if (sequence !== loadSequence) return
    appStore.showError(extractApiErrorMessage(error, t('channelStatus.loadError')))
  } finally {
    if (sequence === loadSequence && !silent) loading.value = false
  }
}

async function manualReload() {
  await load(false)
}

function selectWindow(value: HallWindow) {
  if (selectedWindow.value === value) return
  selectedWindow.value = value
  void load(false)
}

function sortValue(row: HallRow, key: SortKey) {
  if (key === 'rate') return finiteNumber(row.group.effective_rate)
  if (key === 'status') {
    const status = groupMonitorStatus(row.monitor)
    if (status === 'operational') return 3
    if (status === 'failed') return 2
    if (status === 'error') return 1
    return null
  }
  if (key === 'dialog_latency') return finiteNumber(row.monitor?.primary_latency_ms)
  if (key === 'endpoint_ping') return finiteNumber(row.monitor?.primary_ping_latency_ms)
  if (key === 'cache_hit_rate') return finiteNumber(row.group.metrics.cache_hit_rate)
  return finiteNumber(row.group.metrics.availability)
}

function finiteNumber(value: number | null | undefined) {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function toggleSort(key: SortKey) {
  if (sortKey.value === key) sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
  else {
    sortKey.value = key
    sortDirection.value = 'desc'
  }
}

function sortAria(key: SortKey) {
  return sortKey.value === key ? sortDirection.value === 'asc' ? 'ascending' : 'descending' : 'none'
}

function sortIcon(key: SortKey) {
  if (sortKey.value !== key) return 'sort'
  return sortDirection.value === 'asc' ? 'arrowUp' : 'arrowDown'
}

function openDetail(monitor: UserMonitorView) {
  detailTarget.value = monitor
  showDetail.value = true
}

function closeDetail() {
  showDetail.value = false
  detailTarget.value = null
}

function formatRate(value: number) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '--'
  return number.toFixed(4).replace(/\.?0+$/, '')
}

function formatMetric(value: number | null | undefined, suffix: string, digits = 0) {
  return typeof value === 'number' && Number.isFinite(value)
    ? `${value.toFixed(digits)}${suffix}`
    : t('channelStatus.noData')
}

function formatTime(value: Date) {
  return value.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function latestRecord(monitor: UserMonitorView | null) {
  return monitor?.timeline?.[0] || null
}

function latestRecordText(monitor: UserMonitorView | null) {
  const record = latestRecord(monitor)
  if (!record) return t('channelStatus.noRecord')
  return `${statusLabel(record.status)} · ${formatRelativeTime(record.checked_at)} · ${formatMetric(record.latency_ms, 'ms')}`
}

function latestProbeText(row: HallRow) {
  const checkedAt = latestRecord(row.monitor)?.checked_at || row.group.metrics.latest_probe_at
  return checkedAt ? formatRelativeTime(checkedAt) : t('channelStatus.noData')
}

function monitorModels(monitor: UserMonitorView): MonitorModelStatus[] {
  return [{
    model: monitor.primary_model,
    status: monitor.primary_status,
    latency_ms: monitor.primary_latency_ms,
  }, ...monitor.extra_models]
}

function groupMonitorStatus(monitor: UserMonitorView | null): MonitorStatus | '' {
  if (!monitor) return ''
  const statuses = monitorModels(monitor).map(model => model.status)
  if (statuses.some(status => status === 'operational' || status === 'degraded')) return 'operational'
  if (statuses.some(status => status === 'failed')) return 'failed'
  if (statuses.some(status => status === 'error')) return 'error'
  return ''
}

function statusDotClass(status: MonitorStatus) {
  switch (status) {
    case 'operational': return 'bg-emerald-500'
    case 'degraded': return 'bg-amber-500'
    case 'failed':
    case 'error': return 'bg-red-500'
    default: return 'bg-gray-300 dark:bg-dark-600'
  }
}

onMounted(() => {
  void load(false)
  autoRefresh.setEnabled(true)
})
</script>
