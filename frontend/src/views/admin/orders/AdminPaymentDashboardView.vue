<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Header with Day Switcher -->
      <div class="flex flex-col items-end gap-2">
        <div class="flex flex-wrap items-center justify-end gap-2">
          <div class="flex rounded-lg border border-gray-200 dark:border-dark-600">
            <button
              v-for="d in DAYS_OPTIONS"
              :key="d"
              type="button"
              class="px-3 py-1.5 text-xs font-medium transition-colors first:rounded-l-lg last:rounded-r-lg"
              :class="activeRange === d
                ? 'bg-primary-600 text-white'
                : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
              @click="selectDays(d)"
            >
              {{ d }}{{ t('payment.admin.daySuffix') }}
            </button>
          </div>
          <button
            type="button"
            class="btn btn-secondary px-2.5"
            :class="activeRange === 'custom' ? 'border-primary-500 text-primary-600 dark:text-primary-400' : ''"
            :title="t('payment.admin.customRange')"
            @click="showCustomRange = !showCustomRange"
          >
            <Icon name="calendar" size="md" />
          </button>
          <button @click="loadDashboard" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
        <div
          v-if="showCustomRange"
          class="flex w-full flex-wrap items-end justify-end gap-2 rounded-lg border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-600 dark:bg-dark-800"
        >
          <label class="flex min-w-52 flex-col gap-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('payment.admin.startTime') }}
            <input v-model="customStart" type="datetime-local" :max="customEnd || maxDateTime" class="input py-1.5 text-sm" />
          </label>
          <label class="flex min-w-52 flex-col gap-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('payment.admin.endTime') }}
            <input v-model="customEnd" type="datetime-local" :min="customStart" :max="maxDateTime" class="input py-1.5 text-sm" />
          </label>
          <button type="button" class="btn btn-primary" :disabled="!customRangeValid || loading" @click="applyCustomRange">
            {{ t('payment.admin.applyRange') }}
          </button>
        </div>
      </div>

      <!-- Dashboard Content -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
      <template v-else-if="stats">
        <OrderStatsCards :stats="stats" />
        <DailyRevenueChart :data="stats.daily_series || []" :loading="loading" />
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <div class="card p-4">
            <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.paymentDistribution') }}</h3>
            <div v-if="!stats.payment_methods?.length" class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('payment.admin.noData') }}</div>
            <div v-else class="space-y-3">
              <div v-for="method in stats.payment_methods" :key="method.type" class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span :class="['inline-block h-3 w-3 rounded-full', methodColor(method.type)]"></span>
                  <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('payment.methods.' + method.type, method.type) }}</span>
                </div>
                <div class="space-y-1 text-right">
                  <span v-for="[currency, amount] in sortedAmounts(method.amount)" :key="currency" class="block text-sm font-medium text-gray-900 dark:text-white">{{ formatMoney(currency, amount) }}</span>
                  <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">({{ method.count }})</span>
                </div>
              </div>
            </div>
          </div>
          <div class="card p-4">
            <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.topUsers') }}</h3>
            <div v-if="!hasTopUsers(stats.top_users)" class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('payment.admin.noData') }}</div>
            <div v-else class="space-y-2">
              <div v-for="[currency, users] in sortedTopUsers(stats.top_users)" :key="currency" class="space-y-2">
                <p class="text-xs font-semibold text-gray-500 dark:text-gray-400">{{ currency }}</p>
                <div v-for="(user, idx) in users" :key="user.user_id" class="flex items-center justify-between rounded-lg px-3 py-2 hover:bg-gray-50 dark:hover:bg-dark-700">
                  <div class="flex items-center gap-3">
                    <span :class="['flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold', rankClass(idx)]">{{ idx + 1 }}</span>
                    <span class="text-sm text-gray-700 dark:text-gray-300">{{ user.email }}</span>
                  </div>
                  <span class="text-sm font-medium text-gray-900 dark:text-white">{{ formatMoney(currency, user.amount) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { CurrencyAmounts, DashboardStats, TopUserPaymentStats } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderStatsCards from '@/components/admin/payment/OrderStatsCards.vue'
import DailyRevenueChart from '@/components/admin/payment/DailyRevenueChart.vue'

const { t } = useI18n()
const appStore = useAppStore()

const DAYS_OPTIONS = [7, 30, 90, 120] as const
const days = ref<number>(30)
const activeRange = ref<number | 'custom'>(30)
const showCustomRange = ref(false)

function toLocalDateTimeInput(date: Date): string {
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

const now = new Date()
const customEnd = ref(toLocalDateTimeInput(now))
const customStart = ref(toLocalDateTimeInput(new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)))
const appliedCustomStart = ref(customStart.value)
const appliedCustomEnd = ref(customEnd.value)
const maxDateTime = computed(() => toLocalDateTimeInput(new Date()))
const customRangeValid = computed(() => {
  const start = new Date(customStart.value)
  const end = new Date(customEnd.value)
  return customStart.value !== '' && customEnd.value !== '' && !Number.isNaN(start.getTime()) && start < end
})
const loading = ref(false)
const stats = ref<DashboardStats | null>(null)

function methodColor(type: string): string {
  const c: Record<string, string> = {
    alipay: 'bg-blue-500', wxpay: 'bg-green-500',
    alipay_direct: 'bg-blue-400', wxpay_direct: 'bg-green-400',
    stripe: 'bg-purple-500',
  }
  return c[type] || 'bg-gray-400'
}

function rankClass(idx: number): string {
  if (idx === 0) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  if (idx === 1) return 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
  if (idx === 2) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
}

function sortedAmounts(amounts: CurrencyAmounts): [string, number][] {
  return Object.entries(amounts).sort(([left], [right]) => left.localeCompare(right))
}

function sortedTopUsers(usersByCurrency: Record<string, TopUserPaymentStats[]>): [string, TopUserPaymentStats[]][] {
  return Object.entries(usersByCurrency).sort(([left], [right]) => left.localeCompare(right))
}

function hasTopUsers(usersByCurrency: Record<string, TopUserPaymentStats[]>): boolean {
  return Object.values(usersByCurrency).some(users => users.length > 0)
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}

async function loadDashboard() {
  loading.value = true
  try {
    const params = activeRange.value === 'custom'
      ? { start_time: new Date(appliedCustomStart.value).toISOString(), end_time: new Date(appliedCustomEnd.value).toISOString() }
      : { days: days.value }
    const res = await adminPaymentAPI.getDashboard(params)
    stats.value = res.data
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function selectDays(value: number) {
  days.value = value
  activeRange.value = value
  void loadDashboard()
}

function applyCustomRange() {
  if (!customRangeValid.value) return
  appliedCustomStart.value = customStart.value
  appliedCustomEnd.value = customEnd.value
  activeRange.value = 'custom'
  showCustomRange.value = false
  void loadDashboard()
}

onMounted(() => loadDashboard())
</script>
