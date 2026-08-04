<template>
  <section class="space-y-5">
    <div>
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">收益账单</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">账单仅展示资源、用量与结算信息。</p>
    </div>

    <div class="grid gap-3 sm:grid-cols-3">
      <div class="rounded-lg border border-amber-200 bg-amber-50/70 p-4 dark:border-amber-900/50 dark:bg-amber-900/10">
        <div class="flex items-center justify-between text-amber-700 dark:text-amber-300">
          <span class="text-xs font-medium">待结算</span>
          <Icon name="clock" size="sm" />
        </div>
        <div class="mt-3 text-xl font-semibold text-gray-900 dark:text-white">¥{{ money(currentSupplier.pending_balance_cny) }}</div>
      </div>
      <div class="rounded-lg border border-emerald-200 bg-emerald-50/70 p-4 dark:border-emerald-900/50 dark:bg-emerald-900/10">
        <div class="flex items-center justify-between text-emerald-700 dark:text-emerald-300">
          <span class="text-xs font-medium">可提现</span>
          <Icon name="dollar" size="sm" />
        </div>
        <div class="mt-3 text-xl font-semibold text-gray-900 dark:text-white">¥{{ money(currentSupplier.available_balance_cny) }}</div>
      </div>
      <div class="rounded-lg border border-gray-200 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-800/50">
        <div class="flex items-center justify-between text-gray-500 dark:text-dark-400">
          <span class="text-xs font-medium">提现冻结</span>
          <Icon name="lock" size="sm" />
        </div>
        <div class="mt-3 text-xl font-semibold text-gray-900 dark:text-white">¥{{ money(currentSupplier.frozen_balance_cny) }}</div>
      </div>
    </div>

    <div class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
      <div class="flex flex-col gap-3 border-b border-gray-200 px-4 py-3 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700">
        <div class="flex overflow-x-auto rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
          <button
            v-for="filter in filters"
            :key="filter.value"
            type="button"
            class="min-w-max rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
            :class="status === filter.value
              ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
              : 'text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-dark-200'"
            @click="setStatus(filter.value)"
          >
            {{ filter.label }}
          </button>
        </div>
        <button type="button" class="btn btn-secondary btn-sm self-start sm:self-auto" :disabled="loading" @click="loadBills">
          <Icon name="refresh" size="xs" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
      </div>

      <div v-if="loading" class="flex min-h-[260px] items-center justify-center">
        <LoadingSpinner />
      </div>

      <template v-else-if="bills.length">
        <div class="hidden overflow-x-auto md:block">
          <table class="w-full min-w-[980px] text-sm">
            <thead class="border-b border-gray-200 bg-gray-50 text-left text-xs font-medium text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-dark-400">
              <tr>
                <th class="px-4 py-3">调用时间</th>
                <th class="px-4 py-3">资源与模型</th>
                <th class="px-4 py-3">Token 用量</th>
                <th class="px-4 py-3">计费倍率</th>
                <th class="px-4 py-3">供应商收益</th>
                <th class="px-4 py-3">结算状态</th>
                <th class="px-4 py-3">可提现时间</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="bill in pagedBills" :key="bill.id" class="hover:bg-gray-50/70 dark:hover:bg-dark-800/40">
                <td class="px-4 py-4 text-xs text-gray-500 dark:text-dark-400">{{ formatDate(bill.created_at) }}</td>
                <td class="px-4 py-4">
                  <div class="font-medium text-gray-900 dark:text-white">{{ bill.group_name || `#${bill.group_id}` }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ bill.model || '--' }}</div>
                </td>
                <td class="px-4 py-4 text-xs text-gray-600 dark:text-dark-300">
                  <div>输入 {{ formatInteger(bill.input_tokens) }} / 输出 {{ formatInteger(bill.output_tokens) }}</div>
                  <div class="mt-1 text-gray-400">缓存 {{ formatInteger(bill.cache_read_tokens) }}</div>
                </td>
                <td class="px-4 py-4 font-mono text-xs text-gray-600 dark:text-dark-300">
                  {{ rate(bill.base_rate) }} → {{ rate(bill.effective_rate) }}
                </td>
                <td class="px-4 py-4 font-semibold text-emerald-700 dark:text-emerald-400">¥{{ money(bill.amount_cny) }}</td>
                <td class="px-4 py-4"><BillStatus :status="bill.status" /></td>
                <td class="px-4 py-4 text-xs text-gray-500 dark:text-dark-400">{{ bill.available_at ? formatDate(bill.available_at) : '--' }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="divide-y divide-gray-100 md:hidden dark:divide-dark-700">
          <article v-for="bill in pagedBills" :key="bill.id" class="space-y-4 p-4">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="truncate font-medium text-gray-900 dark:text-white">{{ bill.group_name || `#${bill.group_id}` }}</h3>
                <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ bill.model || '--' }}</p>
              </div>
              <BillStatus :status="bill.status" />
            </div>
            <dl class="grid grid-cols-2 gap-3 text-xs">
              <div>
                <dt class="text-gray-400">Token</dt>
                <dd class="mt-1 text-gray-700 dark:text-dark-200">{{ formatInteger(bill.input_tokens + bill.output_tokens) }}</dd>
              </div>
              <div>
                <dt class="text-gray-400">计费倍率</dt>
                <dd class="mt-1 font-mono text-gray-700 dark:text-dark-200">{{ rate(bill.base_rate) }} → {{ rate(bill.effective_rate) }}</dd>
              </div>
              <div>
                <dt class="text-gray-400">收益</dt>
                <dd class="mt-1 font-semibold text-emerald-700 dark:text-emerald-400">¥{{ money(bill.amount_cny) }}</dd>
              </div>
              <div>
                <dt class="text-gray-400">调用时间</dt>
                <dd class="mt-1 text-gray-700 dark:text-dark-200">{{ formatDate(bill.created_at) }}</dd>
              </div>
            </dl>
          </article>
        </div>

        <div class="flex flex-col gap-3 border-t border-gray-200 px-4 py-3 text-xs text-gray-500 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700 dark:text-dark-400">
          <span>共 {{ bills.length }} 条账单</span>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="page <= 1" @click="page--">
              <Icon name="chevronLeft" size="xs" />
              上一页
            </button>
            <span class="min-w-16 text-center">{{ page }} / {{ pageCount }}</span>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="page >= pageCount" @click="page++">
              下一页
              <Icon name="chevronRight" size="xs" />
            </button>
          </div>
        </div>
      </template>

      <div v-else class="flex min-h-[280px] flex-col items-center justify-center px-5 text-center">
        <div class="rounded-full bg-gray-100 p-3 text-gray-400 dark:bg-dark-800 dark:text-dark-500">
          <Icon name="chart" size="lg" />
        </div>
        <h3 class="mt-4 font-medium text-gray-900 dark:text-white">暂无收益账单</h3>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Icon } from '@/components/icons'
import { supplierAPI, type Supplier, type SupplierBill } from '@/api/suppliers'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ supplier: Supplier }>()
const appStore = useAppStore()
const loading = ref(true)
const bills = ref<SupplierBill[]>([])
const currentSupplier = ref(props.supplier)
const status = ref('')
const page = ref(1)
const pageSize = 15
const filters = [
  { label: '全部', value: '' },
  { label: '待结算', value: 'pending' },
  { label: '可提现', value: 'available' },
  { label: '已冻结', value: 'frozen' }
]

const BillStatus = defineComponent({
  props: { status: { type: String, required: true } },
  setup(props) {
    return () => h('span', {
      class: `inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${
        props.status === 'available'
          ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
          : props.status === 'frozen'
            ? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'
            : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
      }`
    }, props.status === 'available' ? '可提现' : props.status === 'frozen' ? '已冻结' : '待结算')
  }
})

const pageCount = computed(() => Math.max(1, Math.ceil(bills.value.length / pageSize)))
const pagedBills = computed(() => bills.value.slice((page.value - 1) * pageSize, page.value * pageSize))

async function loadBills() {
  loading.value = true
  try {
    const [response, latestSupplier] = await Promise.all([
      supplierAPI.bills(status.value),
      supplierAPI.me().catch(() => currentSupplier.value)
    ])
    bills.value = response.items || []
    currentSupplier.value = latestSupplier
    page.value = Math.min(page.value, pageCount.value)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载收益账单失败'))
  } finally {
    loading.value = false
  }
}

function setStatus(value: string) {
  status.value = value
  page.value = 1
  void loadBills()
}

const formatDate = (value: string) => new Date(value).toLocaleString()
const formatInteger = (value: number) => Number(value || 0).toLocaleString()
const rate = (value: number) => Number(value || 0).toFixed(4)
const money = (value: number) => Number(value || 0).toFixed(2)

onMounted(loadBills)
</script>
