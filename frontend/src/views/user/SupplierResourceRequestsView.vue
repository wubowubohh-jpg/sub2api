<template>
  <section class="space-y-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">资源申请记录</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">查看审核结果、监听模型和上游倍率探测状态。</p>
      </div>
      <button type="button" class="btn btn-secondary self-start" :disabled="loading" @click="loadRequests">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        刷新
      </button>
    </div>

    <div class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
      <div v-if="loading" class="flex min-h-[260px] items-center justify-center">
        <LoadingSpinner />
      </div>

      <template v-else-if="requests.length">
        <div class="hidden overflow-x-auto md:block">
          <table class="w-full min-w-[1040px] text-sm">
            <thead class="border-b border-gray-200 bg-gray-50 text-left text-xs font-medium text-gray-500 dark:border-dark-700 dark:bg-dark-800/60 dark:text-dark-400">
              <tr>
                <th class="px-4 py-3">分组与中转站</th>
                <th class="px-4 py-3">模型配置</th>
                <th class="px-4 py-3">审核状态</th>
                <th class="px-4 py-3">上游倍率探测</th>
                <th class="px-4 py-3">提交时间</th>
                <th class="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="request in pagedRequests" :key="request.id" class="hover:bg-gray-50/70 dark:hover:bg-dark-800/40">
                <td class="px-4 py-4 align-top">
                  <div class="font-medium text-gray-900 dark:text-white">{{ request.group_name }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ request.relay_name }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    供应商提交倍率 <span class="font-mono font-medium text-gray-700 dark:text-dark-200">{{ formatRate(request.rate_multiplier) }}</span>
                  </div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    当前有效倍率 <span class="font-mono font-semibold text-emerald-600 dark:text-emerald-400">{{ formatRate(effectiveRate(request)) }}</span>
                    <span class="ml-1 text-gray-400">（{{ rateFormula(request) }}）</span>
                  </div>
                  <a
                    :href="request.relay_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="mt-1 inline-flex max-w-[220px] items-center gap-1 truncate text-xs text-primary-600 hover:underline dark:text-primary-400"
                  >
                    <span class="truncate">{{ request.relay_url }}</span>
                    <Icon name="externalLink" size="xs" class="flex-shrink-0" />
                  </a>
                </td>
                <td class="px-4 py-4 align-top">
                  <div class="flex max-w-[300px] flex-wrap gap-1.5">
                    <span
                      v-for="model in supportedModels(request)"
                      :key="model"
                      class="rounded-md bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-dark-700 dark:text-dark-200"
                    >
                      {{ model }}
                    </span>
                  </div>
                  <div class="mt-2 flex items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400">
                    <Icon name="sync" size="xs" />
                    监听 {{ monitorModel(request) }}
                  </div>
                </td>
                <td class="px-4 py-4 align-top">
                  <StatusBadge :status="request.status" />
                  <p v-if="request.review_note" class="mt-2 max-w-[220px] text-xs text-gray-500 dark:text-dark-400">
                    {{ request.review_note }}
                  </p>
                </td>
                <td class="px-4 py-4 align-top">
                  <div class="flex items-center gap-2">
                    <Toggle
                      :model-value="probeEnabled(request)"
                      :disabled="request.status !== 'approved' || probeSavingId === request.id"
                      @update:model-value="setProbeEnabled(request, $event)"
                    />
                    <span class="text-xs text-gray-500 dark:text-dark-400">
                      {{ probeEnabled(request) ? '已开启' : '已关闭' }}
                    </span>
                  </div>
                  <div class="mt-2 flex items-center gap-2">
                    <span class="inline-flex items-center gap-1.5 text-xs" :class="probeStatusClass(request)">
                      <span class="h-1.5 w-1.5 rounded-full bg-current" />
                      {{ probeStatusLabel(request) }}
                    </span>
                    <span v-if="upstreamRate(request) !== null" class="text-sm font-semibold text-gray-900 dark:text-white">
                      {{ formatRate(upstreamRate(request)) }}
                    </span>
                  </div>
                  <div v-if="probeUpdatedAt(request)" class="mt-1 text-xs text-gray-400">
                    {{ formatDate(probeUpdatedAt(request) || '') }}
                  </div>
                  <div
                    v-if="request.upstream_probe_error"
                    class="mt-1 max-w-[240px] truncate text-xs text-rose-500"
                    :title="request.upstream_probe_error"
                  >
                    {{ request.upstream_probe_error }}
                  </div>
                </td>
                <td class="px-4 py-4 align-top text-xs text-gray-500 dark:text-dark-400">
                  {{ formatDate(request.created_at) }}
                </td>
                <td class="px-4 py-4 text-right align-top">
                  <button
                    v-if="canUpdateAPIKey(request)"
                    type="button"
                    class="btn btn-secondary btn-sm"
                    @click="openKeyDialog(request)"
                  >
                    <Icon name="key" size="xs" />
                    更新 API Key
                  </button>
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm ml-2"
                    @click="openRateDialog(request)"
                  >
                    <Icon name="edit" size="xs" />
                    修改倍率
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="divide-y divide-gray-100 md:hidden dark:divide-dark-700">
          <article v-for="request in pagedRequests" :key="request.id" class="space-y-4 p-4">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="truncate font-medium text-gray-900 dark:text-white">{{ request.group_name }}</h3>
                <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ request.relay_name }}</p>
              </div>
              <StatusBadge :status="request.status" />
            </div>
            <div class="flex flex-wrap gap-1.5">
              <span
                v-for="model in supportedModels(request)"
                :key="model"
                class="rounded-md bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-dark-700 dark:text-dark-200"
              >{{ model }}</span>
            </div>
            <dl class="grid grid-cols-2 gap-3 text-xs">
              <div>
                <dt class="text-gray-400">监听模型</dt>
                <dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">{{ monitorModel(request) }}</dd>
              </div>
              <div>
                <dt class="text-gray-400">供应商提交倍率</dt>
                <dd class="mt-1 font-mono font-medium text-gray-700 dark:text-dark-200">{{ formatRate(request.rate_multiplier) }}</dd>
              </div>
              <div>
                <dt class="text-gray-400">探测倍率</dt>
                <dd class="mt-1 font-medium text-gray-700 dark:text-dark-200">
                  {{ upstreamRate(request) === null ? probeStatusLabel(request) : formatRate(upstreamRate(request)) }}
                </dd>
              </div>
              <div>
                <dt class="text-gray-400">当前有效倍率</dt>
                <dd class="mt-1 font-mono font-semibold text-emerald-600 dark:text-emerald-400">{{ formatRate(effectiveRate(request)) }}</dd>
              </div>
            </dl>
            <div class="flex items-center justify-between border-t border-gray-100 pt-3 dark:border-dark-700">
              <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                <Toggle
                  :model-value="probeEnabled(request)"
                  :disabled="request.status !== 'approved' || probeSavingId === request.id"
                  @update:model-value="setProbeEnabled(request, $event)"
                />
                倍率探测
              </div>
              <button
                v-if="canUpdateAPIKey(request)"
                type="button"
                class="btn btn-secondary btn-sm"
                @click="openKeyDialog(request)"
              >
                <Icon name="key" size="xs" />
                更新 Key
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="openRateDialog(request)">
                <Icon name="edit" size="xs" />
                修改倍率
              </button>
            </div>
          </article>
        </div>

        <div class="flex flex-col gap-3 border-t border-gray-200 px-4 py-3 text-xs text-gray-500 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700 dark:text-dark-400">
          <span>共 {{ requests.length }} 条申请</span>
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
          <Icon name="inbox" size="lg" />
        </div>
        <h3 class="mt-4 font-medium text-gray-900 dark:text-white">暂无资源申请</h3>
        <RouterLink :to="{ name: 'SupplierResourceSubmit' }" class="btn btn-primary mt-5">
          <Icon name="plus" size="sm" />
          提交中转资源
        </RouterLink>
      </div>
    </div>

    <BaseDialog :show="Boolean(editingRequest)" title="更新 API Key" width="narrow" @close="closeKeyDialog">
      <form @submit.prevent="updateAPIKey">
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ editingRequest?.group_name }} · {{ editingRequest?.relay_name }}
        </p>
        <label class="mt-5 block">
          <span class="input-label">新的 API Key</span>
          <input
            ref="apiKeyInput"
            v-model="newAPIKey"
            type="password"
            required
            autocomplete="new-password"
            placeholder="sk-..."
            class="input w-full"
          />
        </label>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="keySaving" @click="closeKeyDialog">取消</button>
        <button type="button" class="btn btn-primary" :disabled="keySaving || !newAPIKey.trim()" @click="updateAPIKey">
          <LoadingSpinner v-if="keySaving" size="sm" color="white" />
          <Icon v-else name="check" size="sm" />
          保存
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="Boolean(editingRateRequest)" title="修改供应商提交倍率" width="narrow" @close="closeRateDialog">
      <form @submit.prevent="updateRate">
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ editingRateRequest?.group_name }} · {{ editingRateRequest?.relay_name }}
        </p>
        <label class="mt-5 block">
          <span class="input-label">供应商提交倍率</span>
          <input
            v-model.number="newRateMultiplier"
            type="number"
            required
            min="0"
            step="0.0001"
            class="input w-full"
          />
        </label>
        <div v-if="editingRateRequest" class="mt-4 rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-300">
          实际倍率：本次设置 {{ formatRate(Number(newRateMultiplier)) }} + 管理员增加
          {{ signedRate(adminRateAdjustment(editingRateRequest)) }}
        </div>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="rateSaving" @click="closeRateDialog">取消</button>
        <button type="button" class="btn btn-primary" :disabled="rateSaving || !validNewRate" @click="updateRate">
          <LoadingSpinner v-if="rateSaving" size="sm" color="white" />
          <Icon v-else name="check" size="sm" />
          保存倍率
        </button>
      </template>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, nextTick, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Toggle from '@/components/common/Toggle.vue'
import { Icon } from '@/components/icons'
import { supplierAPI, type SupplierResourceRequest } from '@/api/suppliers'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

defineProps<{ supplier: unknown }>()
const appStore = useAppStore()
const loading = ref(true)
const requests = ref<SupplierResourceRequest[]>([])
const page = ref(1)
const pageSize = 10
const editingRequest = ref<SupplierResourceRequest | null>(null)
const newAPIKey = ref('')
const apiKeyInput = ref<HTMLInputElement | null>(null)
const keySaving = ref(false)
const probeSavingId = ref<number | null>(null)
const editingRateRequest = ref<SupplierResourceRequest | null>(null)
const newRateMultiplier = ref<number | string>(0)
const rateSaving = ref(false)

const StatusBadge = defineComponent({
  props: { status: { type: String, required: true } },
  setup(props) {
    return () => h('span', {
      class: `inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${
        props.status === 'approved'
          ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
          : props.status === 'rejected'
            ? 'bg-rose-50 text-rose-700 dark:bg-rose-900/20 dark:text-rose-300'
            : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
      }`
    }, props.status === 'approved' ? '已通过' : props.status === 'rejected' ? '已驳回' : '审核中')
  }
})

const pageCount = computed(() => Math.max(1, Math.ceil(requests.value.length / pageSize)))
const pagedRequests = computed(() => requests.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const validNewRate = computed(() => {
  const value = Number(newRateMultiplier.value)
  return Number.isFinite(value) && value >= 0
})

function supportedModels(request: SupplierResourceRequest) {
  return request.supported_models?.length ? request.supported_models : [request.model].filter(Boolean)
}

function monitorModel(request: SupplierResourceRequest) {
  return request.monitor_model || request.probe_model || request.model || '--'
}

function probeEnabled(request: SupplierResourceRequest) {
  return request.upstream_billing_probe_enabled ?? request.upstream_billing_probe?.enabled ?? request.status === 'approved'
}

function probeStatus(request: SupplierResourceRequest) {
  if (!probeEnabled(request)) return 'disabled'
  const snapshotStatus = request.upstream_billing_probe?.snapshot?.status
  if (snapshotStatus === 'ok') return 'available'
  if (snapshotStatus === 'failed') return 'failed'
  if (snapshotStatus === 'unsupported') return 'no_data'
  return request.upstream_probe_status || (request.status === 'approved' ? 'pending' : 'no_data')
}

function probeStatusLabel(request: SupplierResourceRequest) {
  return ({
    pending: '等待探测',
    probing: '探测中',
    available: '已获取',
    failed: '探测失败',
    credential_invalid: '凭据失效',
    disabled: '已关闭',
    no_data: '暂无数据'
  } as const)[probeStatus(request)] || '暂无数据'
}

function probeStatusClass(request: SupplierResourceRequest) {
  return ({
    available: 'text-emerald-600 dark:text-emerald-400',
    probing: 'text-primary-600 dark:text-primary-400',
    pending: 'text-amber-600 dark:text-amber-400',
    credential_invalid: 'text-rose-600 dark:text-rose-400',
    failed: 'text-rose-600 dark:text-rose-400',
    disabled: 'text-gray-400',
    no_data: 'text-gray-400'
  } as const)[probeStatus(request)] || 'text-gray-400'
}

function upstreamRate(request: SupplierResourceRequest): number | null {
  const dataRate = request.upstream_billing_probe?.snapshot?.data?.effective_rate_multiplier
    ?? request.upstream_billing_probe?.snapshot?.data?.resolved_rate_multiplier
  if (typeof dataRate === 'number' && Number.isFinite(dataRate)) return dataRate
  if (typeof request.upstream_rate === 'number') return request.upstream_rate
  return null
}

function adminRateAdjustment(request: SupplierResourceRequest) {
  const value = Number(request.admin_rate_adjustment ?? 0)
  return Number.isFinite(value) ? value : 0
}

function appliedRate(request: SupplierResourceRequest) {
  const serverValue = Number(request.applied_rate_multiplier)
  if (Number.isFinite(serverValue)) return serverValue
  return Number(request.rate_multiplier || 0)
}

function effectiveRate(request: SupplierResourceRequest) {
  const serverValue = Number(request.effective_rate_multiplier)
  return Number.isFinite(serverValue) ? serverValue : appliedRate(request) + adminRateAdjustment(request)
}

function signedRate(value: number) {
  return `${value >= 0 ? '+' : ''}${formatRate(value)}`
}

function rateFormula(request: SupplierResourceRequest) {
  return `设置倍率 ${formatRate(appliedRate(request))} + 管理员增加 ${formatRate(adminRateAdjustment(request))}`
}

function probeUpdatedAt(request: SupplierResourceRequest) {
  return request.upstream_rate_updated_at || request.upstream_billing_probe?.snapshot?.received_at || request.upstream_billing_probe?.snapshot?.last_attempt_at
}

function canUpdateAPIKey(request: SupplierResourceRequest) {
  const snapshot = request.upstream_billing_probe?.snapshot
  const credentialFailure = snapshot?.status === 'failed' &&
    (snapshot.http_status === 401 || snapshot.http_status === 403 || /unauthori[sz]ed|invalid.+key|api.?key/i.test(snapshot.last_error || ''))
  return request.status === 'pending' ||
    request.status === 'rejected' ||
    request.credentials_need_update === true ||
    request.credentials_valid === false ||
    probeStatus(request) === 'credential_invalid' ||
    credentialFailure
}

function formatRate(value: number | null) {
  return value === null ? '暂无数据' : Number(value).toFixed(4)
}

function formatDate(value: string) {
  return new Date(value).toLocaleString()
}

async function loadRequests() {
  loading.value = true
  try {
    const response = await supplierAPI.resourceRequests()
    requests.value = response.items || []
    page.value = Math.min(page.value, pageCount.value)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '加载资源申请失败'))
  } finally {
    loading.value = false
  }
}

async function setProbeEnabled(request: SupplierResourceRequest, enabled: boolean) {
  if (request.status !== 'approved') return
  const previous = request.upstream_billing_probe_enabled
  request.upstream_billing_probe_enabled = enabled
  probeSavingId.value = request.id
  try {
    const updated = await supplierAPI.updateResourceProbe(request.id, enabled)
    Object.assign(request, updated)
    appStore.showSuccess(enabled ? '已开启上游倍率探测' : '已关闭上游倍率探测')
  } catch (error) {
    request.upstream_billing_probe_enabled = previous
    appStore.showError(extractApiErrorMessage(error, '更新探测设置失败'))
  } finally {
    probeSavingId.value = null
  }
}

async function openKeyDialog(request: SupplierResourceRequest) {
  editingRequest.value = request
  newAPIKey.value = ''
  await nextTick()
  apiKeyInput.value?.focus()
}

function closeKeyDialog() {
  if (keySaving.value) return
  editingRequest.value = null
  newAPIKey.value = ''
}

function openRateDialog(request: SupplierResourceRequest) {
  editingRateRequest.value = request
  newRateMultiplier.value = request.rate_multiplier
}

function closeRateDialog() {
  if (rateSaving.value) return
  editingRateRequest.value = null
}

async function updateRate() {
  if (!editingRateRequest.value || !validNewRate.value) return
  rateSaving.value = true
  try {
    const updated = await supplierAPI.updateResourceRate(
      editingRateRequest.value.id,
      Number(newRateMultiplier.value),
    )
    const index = requests.value.findIndex(item => item.id === updated.id)
    if (index >= 0) requests.value[index] = updated
    appStore.showSuccess('供应商倍率已更新并实时生效')
    editingRateRequest.value = null
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '更新倍率失败'))
  } finally {
    rateSaving.value = false
  }
}

async function updateAPIKey() {
  if (!editingRequest.value || !newAPIKey.value.trim()) return
  keySaving.value = true
  try {
    const updated = await supplierAPI.updateResourceRequestAPIKey(editingRequest.value.id, newAPIKey.value.trim())
    const index = requests.value.findIndex(item => item.id === updated.id)
    if (index >= 0) requests.value[index] = updated
    appStore.showSuccess('API Key 已更新')
    editingRequest.value = null
    newAPIKey.value = ''
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '更新 API Key 失败'))
  } finally {
    keySaving.value = false
  }
}

onMounted(loadRequests)
</script>
