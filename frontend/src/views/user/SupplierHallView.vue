<template>
  <AppLayout>
    <div class="mx-auto max-w-[1800px] space-y-5">
      <header class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">供应商大厅</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">查看启用分组的主动探测、真实流量与最终倍率。</p>
        </div>
        <div class="flex items-center gap-2 self-start sm:self-auto">
          <span class="inline-flex h-9 items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 text-xs font-medium text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-900/20 dark:text-emerald-300">
            <span class="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            监测中
          </span>
          <button type="button" class="btn btn-secondary h-9" :disabled="loading" @click="load">
            {{ loading ? '刷新中' : '刷新' }}
          </button>
        </div>
      </header>

      <div class="grid w-full max-w-md grid-cols-4 rounded-lg border border-gray-200 bg-white p-1 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <button
          v-for="item in windows"
          :key="item"
          type="button"
          class="h-9 rounded-md px-3 text-sm font-medium transition-colors"
          :class="selectedWindow === item
            ? 'bg-primary-600 text-white shadow-sm'
            : 'text-gray-500 hover:bg-gray-50 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white'"
          @click="selectWindow(item)"
        >
          {{ item }}
        </button>
      </div>

      <div class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div v-if="loading" class="flex min-h-[320px] items-center justify-center text-sm text-gray-400">
          正在加载监测数据...
        </div>

        <div v-else>
          <div class="hidden overflow-x-auto md:block">
          <table class="w-full min-w-[1480px] text-left text-sm">
            <thead class="border-b border-gray-200 bg-gray-50 text-xs font-medium text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-dark-400">
              <tr>
                <th class="px-4 py-3">分组</th>
                <th class="px-4 py-3">最终倍率</th>
                <th class="px-4 py-3">状态</th>
                <th class="px-4 py-3">首 Token</th>
                <th class="px-4 py-3">探测延迟</th>
                <th class="px-4 py-3">用户平均</th>
                <th class="px-4 py-3">缓存命中</th>
                <th class="px-4 py-3">可用率</th>
                <th class="w-[250px] px-4 py-3">TPS 与趋势</th>
                <th class="px-4 py-3">最近探测</th>
                <th class="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="group in groups" :key="group.id" class="hover:bg-gray-50/70 dark:hover:bg-dark-800/40">
                <td class="px-4 py-4">
                  <div class="font-semibold text-gray-900 dark:text-white">{{ group.name }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ group.platform }}</div>
                </td>
                <td class="px-4 py-4">
                  <div class="font-mono font-semibold text-gray-900 dark:text-white">{{ formatRate(group.effective_rate) }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="inline-flex rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300">可用</span>
                </td>
                <td class="px-4 py-4" :class="metricClass(group.metrics.avg_first_token_ms)">
                  {{ formatMetric(group.metrics.avg_first_token_ms, ' ms') }}
                </td>
                <td class="px-4 py-4" :class="metricClass(group.metrics.probe_latency_ms)">
                  {{ formatMetric(group.metrics.probe_latency_ms, ' ms') }}
                </td>
                <td class="px-4 py-4" :class="metricClass(group.metrics.avg_latency_ms)">
                  {{ formatMetric(group.metrics.avg_latency_ms, ' ms') }}
                </td>
                <td class="px-4 py-4" :class="metricClass(group.metrics.cache_hit_rate)">
                  {{ formatMetric(group.metrics.cache_hit_rate, '%', 1) }}
                </td>
                <td class="px-4 py-4" :class="metricClass(group.metrics.availability)">
                  {{ formatMetric(group.metrics.availability, '%', 1) }}
                </td>
                <td class="px-4 py-4">
                  <div v-if="hasTimeline(group)" class="h-8 w-[210px] text-primary-500" :title="`${group.metrics.request_count} 次真实调用`">
                    <svg viewBox="0 0 210 32" class="h-8 w-[210px]" preserveAspectRatio="none" aria-hidden="true">
                      <polyline :points="sparklinePoints(group)" fill="none" stroke="currentColor" stroke-width="2" vector-effect="non-scaling-stroke" />
                    </svg>
                  </div>
                  <div v-else class="h-8 w-[210px] border-b border-dashed border-gray-200 text-xs text-gray-400 dark:border-dark-600">暂无数据</div>
                  <div class="mt-1 text-xs" :class="metricClass(group.metrics.tps)">
                    {{ formatMetric(group.metrics.tps, ' tokens/s', 1) }}
                  </div>
                </td>
                <td class="px-4 py-4 text-xs" :class="group.metrics.latest_probe_at ? 'text-gray-600 dark:text-dark-300' : 'text-gray-400'">
                  {{ group.metrics.latest_probe_at ? formatDate(group.metrics.latest_probe_at) : '暂无数据' }}
                </td>
                <td class="px-4 py-4 text-right">
                  <RouterLink :to="{ path: '/keys', query: { group: String(group.id) } }" class="btn btn-secondary btn-sm whitespace-nowrap text-primary-700 dark:text-primary-300">
                    使用此分组
                  </RouterLink>
                </td>
              </tr>
              <tr v-if="!groups.length">
                <td colspan="11" class="px-5 py-20 text-center text-gray-400">暂无启用分组</td>
              </tr>
            </tbody>
          </table>
          </div>

          <div class="divide-y divide-gray-100 md:hidden dark:divide-dark-700">
            <article v-for="group in groups" :key="group.id" class="space-y-4 p-4">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <h2 class="truncate font-semibold text-gray-900 dark:text-white">{{ group.name }}</h2>
                  <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ group.platform }}</p>
                </div>
                <span class="inline-flex flex-shrink-0 rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300">可用</span>
              </div>
              <div class="flex items-end justify-between border-b border-gray-100 pb-3 dark:border-dark-700">
                <div>
                  <div class="text-xs text-gray-400">最终倍率</div>
                  <div class="mt-1 font-mono text-lg font-semibold text-gray-900 dark:text-white">{{ formatRate(group.effective_rate) }}</div>
                </div>
              </div>
              <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-xs">
                <div><dt class="text-gray-400">首 Token</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(group.metrics.avg_first_token_ms, ' ms') }}</dd></div>
                <div><dt class="text-gray-400">探测延迟</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(group.metrics.probe_latency_ms, ' ms') }}</dd></div>
                <div><dt class="text-gray-400">用户平均</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(group.metrics.avg_latency_ms, ' ms') }}</dd></div>
                <div><dt class="text-gray-400">缓存命中</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(group.metrics.cache_hit_rate, '%', 1) }}</dd></div>
                <div><dt class="text-gray-400">可用率</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(group.metrics.availability, '%', 1) }}</dd></div>
                <div><dt class="text-gray-400">TPS</dt><dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ formatMetric(group.metrics.tps, ' tokens/s', 1) }}</dd></div>
              </dl>
              <div class="flex items-center justify-between gap-3 border-t border-gray-100 pt-3 text-xs dark:border-dark-700">
                <span class="text-gray-400">最近探测：{{ group.metrics.latest_probe_at ? formatDate(group.metrics.latest_probe_at) : '暂无数据' }}</span>
                <RouterLink :to="{ path: '/keys', query: { group: String(group.id) } }" class="btn btn-secondary btn-sm whitespace-nowrap text-primary-700 dark:text-primary-300">使用此分组</RouterLink>
              </div>
            </article>
            <div v-if="!groups.length" class="px-5 py-20 text-center text-gray-400">暂无启用分组</div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { supplierAPI, type HallGroup } from '@/api/suppliers'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const windows = ['6h', '24h', '7d', '30d'] as const
const selectedWindow = ref<(typeof windows)[number]>('6h')
const groups = ref<HallGroup[]>([])
const loading = ref(true)
const appStore = useAppStore()

async function load() {
  loading.value = true
  try {
    groups.value = (await supplierAPI.hall(selectedWindow.value)).groups || []
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载供应商大厅失败'))
  } finally {
    loading.value = false
  }
}

function selectWindow(value: (typeof windows)[number]) {
  if (selectedWindow.value === value) return
  selectedWindow.value = value
  void load()
}

function formatRate(value: number) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '--'
  return number.toFixed(4).replace(/\.?0+$/, '')
}

function formatMetric(value: number | undefined, suffix: string, digits = 0) {
  return typeof value === 'number' && Number.isFinite(value) ? `${value.toFixed(digits)}${suffix}` : '暂无数据'
}

function metricClass(value: number | undefined) {
  return typeof value === 'number' && Number.isFinite(value)
    ? 'font-medium text-gray-700 dark:text-dark-200'
    : 'text-gray-400'
}

function hasTimeline(group: HallGroup) {
  return Boolean(group.metrics.timeline?.length)
}

function sparklinePoints(group: HallGroup) {
  const points = group.metrics.timeline || []
  if (points.length === 1) return '0,16 210,16'
  const max = Math.max(...points.map(point => point.requests), 1)
  return points.map((point, index) => {
    const x = index * 210 / Math.max(points.length - 1, 1)
    const y = 29 - point.requests / max * 26
    return `${x.toFixed(1)},${y.toFixed(1)}`
  }).join(' ')
}

function formatDate(value: string) {
  return new Date(value).toLocaleString()
}

onMounted(load)
</script>
