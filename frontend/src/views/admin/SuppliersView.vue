<template>
  <AppLayout>
    <div class="mx-auto max-w-[1600px] space-y-6 p-4 md:p-6">
      <header class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">供应商管理</h1>
          <p class="mt-1 text-sm text-gray-500">管理入驻审核、资源创建和提现申请。</p>
        </div>
        <button class="rounded-lg border bg-white px-3 py-2 text-sm shadow-sm transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900" @click="reloadActive">刷新当前列表</button>
      </header>

      <div v-if="error" class="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{{ error }}</div>

      <section class="rounded-lg border bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
        <div class="mb-4">
          <h2 class="font-medium">供应与结算设置</h2>
          <p class="mt-1 text-xs text-gray-500">设置变更只影响后续供应展示和提现校验。</p>
        </div>
        <div class="flex flex-wrap items-end gap-4">
          <label class="text-sm">自营供应商名称<input v-model="settings.platform_supplier_name" class="mt-1 block w-48 rounded-lg border px-3 py-2 dark:border-gray-700 dark:bg-gray-950" /></label>
          <label class="flex h-10 items-center gap-2 text-sm"><input v-model="settings.platform_supply_enabled" type="checkbox" class="h-4 w-4" />启用平台自营供应</label>
          <label class="text-sm">全局倍率调整<input v-model.number="settings.global_rate_adjustment" type="number" step="0.01" class="mt-1 block w-40 rounded-lg border px-3 py-2 dark:border-gray-700 dark:bg-gray-950" /></label>
          <label class="text-sm">最低提现（USD）<input v-model.number="settings.minimum_withdrawal_usd" type="number" min="0.01" step="1" class="mt-1 block w-40 rounded-lg border px-3 py-2 dark:border-gray-700 dark:bg-gray-950" /></label>
          <button class="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-teal-600 dark:hover:bg-teal-700" @click="saveSettings">保存设置</button>
        </div>
      </section>

      <section class="overflow-hidden rounded-lg border bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
        <div class="flex overflow-x-auto border-b px-2 dark:border-gray-800">
          <button v-for="tab in tabs" :key="tab.value" class="relative whitespace-nowrap px-5 py-4 text-sm font-medium" :class="activeTab === tab.value ? 'text-teal-700 dark:text-teal-400' : 'text-gray-500 hover:text-gray-800 dark:hover:text-gray-200'" @click="selectTab(tab.value)">
            {{ tab.label }}
            <span v-if="tab.count" class="ml-2 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-gray-800 dark:text-gray-300">{{ tab.count }}</span>
            <span v-if="activeTab === tab.value" class="absolute inset-x-3 bottom-0 h-0.5 bg-teal-600"></span>
          </button>
        </div>

        <div v-if="activeLoading" class="py-24 text-center text-sm text-gray-400">正在加载...</div>

        <div v-else-if="activeTab === 'suppliers'" class="overflow-x-auto">
          <table class="w-full min-w-[900px] text-sm">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-gray-950"><tr><th class="p-4">供应商</th><th>中转站</th><th>状态</th><th>待结算</th><th>可提现</th><th class="pr-4 text-right">操作</th></tr></thead>
            <tbody>
              <tr v-for="s in supplierPage" :key="s.id" class="border-t transition hover:bg-gray-50 dark:border-gray-800 dark:hover:bg-gray-800/40">
                <td class="p-4"><div class="font-medium">{{ s.name }}</div><div class="mt-1 text-xs text-gray-400">ID {{ s.id }}</div></td>
                <td><a :href="s.relay_url" target="_blank" rel="noopener" class="text-sky-700 hover:underline">{{ s.relay_url }}</a></td>
                <td><span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(s.status)">{{ statusLabel(s.status) }}</span></td>
                <td>¥{{ money(s.pending_balance_cny) }}</td><td>¥{{ money(s.available_balance_cny) }}</td>
                <td class="space-x-3 pr-4 text-right"><button v-if="s.status === 'pending'" class="text-emerald-700" @click="review(s, 'approved')">通过</button><button v-if="s.status === 'pending'" class="text-rose-700" @click="review(s, 'rejected')">驳回</button><button v-if="s.status === 'approved'" class="text-amber-700" @click="freeze(s)">冻结</button><button v-if="s.status === 'frozen'" class="text-emerald-700" @click="unfreeze(s)">解除冻结</button></td>
              </tr>
              <tr v-if="!supplierPage.length"><td colspan="6" class="p-16 text-center text-gray-400">暂无供应商记录</td></tr>
            </tbody>
          </table>
        </div>

        <div v-else-if="activeTab === 'resources'" class="overflow-x-auto">
          <table class="w-full min-w-[980px] text-sm">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-gray-950"><tr><th class="p-4">分组</th><th>中转站</th><th>地址</th><th>模型</th><th>状态</th><th class="pr-4 text-right">操作</th></tr></thead>
            <tbody>
              <tr v-for="r in resourcePage" :key="r.id" class="border-t transition hover:bg-gray-50 dark:border-gray-800 dark:hover:bg-gray-800/40"><td class="p-4"><div class="font-medium">{{ r.group_name }}</div><div class="mt-1 text-xs text-gray-400">基础倍率 {{ Number(r.rate_multiplier ?? 1).toFixed(4) }}</div></td><td>{{ r.relay_name }}</td><td><a :href="r.relay_url" target="_blank" rel="noopener" class="text-sky-700 hover:underline">{{ r.relay_url }}</a></td><td><div class="flex max-w-64 flex-wrap gap-1"><span v-for="model in resourceModels(r)" :key="model" class="rounded bg-gray-100 px-1.5 py-0.5 text-xs dark:bg-gray-800">{{ model }}</span></div><div class="mt-1 text-xs text-gray-400">监听 {{ r.monitor_model || r.probe_model || r.model }}</div></td><td><span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(r.status)">{{ statusLabel(r.status) }}</span></td><td class="space-x-3 pr-4 text-right"><button v-if="r.status === 'pending'" class="text-sky-700 hover:text-sky-800" @click="openResourceTest(r)">模型测试</button><button v-if="r.status === 'pending'" class="text-emerald-700" @click="reviewResource(r.id, true)">通过并创建</button><button v-if="r.status === 'pending'" class="text-rose-700" @click="reviewResource(r.id, false)">驳回</button><span v-else class="text-xs text-gray-400">已处理</span></td></tr>
              <tr v-if="!resourcePage.length"><td colspan="6" class="p-16 text-center text-gray-400">暂无资源审核记录</td></tr>
            </tbody>
          </table>
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[900px] text-sm">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-gray-950"><tr><th class="p-4">申请号</th><th>供应商 ID</th><th>金额</th><th>方式</th><th>状态</th><th class="pr-4 text-right">操作</th></tr></thead>
            <tbody>
              <tr v-for="w in withdrawalPage" :key="w.id" class="border-t transition hover:bg-gray-50 dark:border-gray-800 dark:hover:bg-gray-800/40"><td class="p-4 font-mono text-xs">{{ w.request_no }}</td><td>{{ w.supplier_id }}</td><td class="font-medium">¥{{ money(w.amount_cny) }}</td><td>{{ methodLabel(w.method) }}</td><td><span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(w.status)">{{ statusLabel(w.status) }}</span></td><td class="space-x-3 pr-4 text-right"><button v-if="w.status === 'pending'" class="text-emerald-700" @click="updateWithdrawal(w.id, 'approved')">通过</button><button v-if="w.status === 'pending'" class="text-rose-700" @click="updateWithdrawal(w.id, 'rejected')">驳回</button><button v-if="w.status === 'approved'" class="text-sky-700" @click="pay(w.id)">标记已打款</button></td></tr>
              <tr v-if="!withdrawalPage.length"><td colspan="6" class="p-16 text-center text-gray-400">暂无提现审核记录</td></tr>
            </tbody>
          </table>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3 border-t px-4 py-3 text-sm dark:border-gray-800">
          <div class="text-gray-500">共 {{ activeTotal }} 条，第 {{ activePage }} / {{ activePages }} 页</div>
          <div class="flex items-center gap-2"><select v-model.number="pageSize" class="rounded-lg border px-2 py-1.5 dark:border-gray-700 dark:bg-gray-950" @change="resetPages"><option :value="10">10 条/页</option><option :value="20">20 条/页</option><option :value="50">50 条/页</option></select><button class="rounded-lg border px-3 py-1.5 disabled:opacity-40 dark:border-gray-700" :disabled="activePage <= 1" @click="changePage(-1)">上一页</button><button class="rounded-lg border px-3 py-1.5 disabled:opacity-40 dark:border-gray-700" :disabled="activePage >= activePages" @click="changePage(1)">下一页</button></div>
        </div>
      </section>
    </div>

    <AccountTestModal
      :show="testingResource !== null"
      :account="testingResourceAccount"
      :model-options="testingResourceModels"
      :test-endpoint="testingResourceEndpoint"
      @close="closeResourceTest"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AccountTestModal from '@/components/admin/account/AccountTestModal.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adminSupplierAPI, type Supplier, type SupplierResourceRequest, type SupplierWithdrawal } from '@/api/suppliers'
import type { ClaudeModel } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

type Tab = 'suppliers' | 'resources' | 'withdrawals'
const activeTab = ref<Tab>('suppliers')
const pageSize = ref(10)
const pages = reactive<Record<Tab, number>>({ suppliers: 1, resources: 1, withdrawals: 1 })
const loaded = reactive<Record<Tab, boolean>>({ suppliers: false, resources: false, withdrawals: false })
const loading = reactive<Record<Tab, boolean>>({ suppliers: false, resources: false, withdrawals: false })
const items = ref<Supplier[]>([])
const resourceRequests = ref<SupplierResourceRequest[]>([])
const withdrawals = ref<SupplierWithdrawal[]>([])
const testingResource = ref<SupplierResourceRequest | null>(null)
const error = ref('')
const settings = reactive({ global_rate_adjustment: 0, minimum_withdrawal_usd: 100, platform_supply_enabled: true, platform_supplier_name: '平台自营' })

const tabs = computed(() => [
  { value: 'suppliers' as Tab, label: '入驻与余额', count: items.value.length },
  { value: 'resources' as Tab, label: '资源审核', count: resourceRequests.value.filter(item => item.status === 'pending').length },
  { value: 'withdrawals' as Tab, label: '提现审核', count: withdrawals.value.filter(item => item.status === 'pending').length },
])
const activeItems = computed(() => activeTab.value === 'suppliers' ? items.value : activeTab.value === 'resources' ? resourceRequests.value : withdrawals.value)
const activeTotal = computed(() => activeItems.value.length)
const activePages = computed(() => Math.max(1, Math.ceil(activeTotal.value / pageSize.value)))
const activePage = computed(() => pages[activeTab.value])
const activeLoading = computed(() => loading[activeTab.value])
function pageSlice<T>(list: T[], tab: Tab) { return list.slice((pages[tab] - 1) * pageSize.value, pages[tab] * pageSize.value) }
const supplierPage = computed(() => pageSlice(items.value, 'suppliers'))
const resourcePage = computed(() => pageSlice(resourceRequests.value, 'resources'))
const withdrawalPage = computed(() => pageSlice(withdrawals.value, 'withdrawals'))
const testingResourceAccount = computed(() => testingResource.value ? {
  id: testingResource.value.id,
  name: `${testingResource.value.group_name} / ${testingResource.value.relay_name}`,
  platform: 'openai' as const,
  type: 'apikey' as const,
  status: 'active' as const,
} : null)
const testingResourceModels = computed<ClaudeModel[]>(() => {
  if (!testingResource.value) return []
  const primary = testingResource.value.probe_model || testingResource.value.monitor_model || testingResource.value.model
  const ids = testingResource.value.supported_models?.length
    ? [primary, ...testingResource.value.supported_models]
    : [primary]
  return [...new Set(ids.filter(Boolean))].map(id => ({
    id,
    type: 'model',
    display_name: id,
    created_at: '',
  }))
})
const testingResourceEndpoint = computed(() => testingResource.value
  ? `/admin/suppliers/resource-requests/${testingResource.value.id}/test`
  : '')

function resourceModels(resource: SupplierResourceRequest) {
  return resource.supported_models?.length ? resource.supported_models : [resource.model].filter(Boolean)
}

function statusClass(status: string) { return ['approved', 'available', 'paid', 'active'].includes(status) ? 'bg-emerald-50 text-emerald-700' : ['rejected', 'frozen'].includes(status) ? 'bg-rose-50 text-rose-700' : 'bg-amber-50 text-amber-700' }
function statusLabel(status: string) { return ({ pending: '待审核', approved: '已通过', rejected: '已驳回', frozen: '已冻结', paid: '已打款' } as Record<string, string>)[status] || status }
function methodLabel(value: string) { return ({ alipay: '支付宝', wechat: '微信', bank: '银行卡' } as Record<string, string>)[value] || value }
function money(value: unknown) { const amount = Number(value); return Number.isFinite(amount) ? amount.toFixed(2) : '0.00' }

async function loadTab(tab: Tab, force = false) {
  if (loaded[tab] && !force) return
  loading[tab] = true; error.value = ''
  try {
    if (tab === 'suppliers') items.value = (await adminSupplierAPI.list()).items || []
    else if (tab === 'resources') resourceRequests.value = (await adminSupplierAPI.resourceRequests()).items || []
    else withdrawals.value = (await adminSupplierAPI.withdrawals()).items || []
    loaded[tab] = true
  } catch (e) { error.value = e instanceof Error ? e.message : '数据加载失败' }
  finally { loading[tab] = false }
}
async function selectTab(tab: Tab) { activeTab.value = tab; await loadTab(tab) }
function reloadActive() { return loadTab(activeTab.value, true) }
function resetPages() { pages.suppliers = 1; pages.resources = 1; pages.withdrawals = 1 }
function changePage(delta: number) { pages[activeTab.value] = Math.min(activePages.value, Math.max(1, pages[activeTab.value] + delta)) }
async function saveSettings() { try { Object.assign(settings, await adminSupplierAPI.updateSettings(settings)); error.value = '' } catch (e) { error.value = e instanceof Error ? e.message : '设置保存失败' } }
async function review(item: Supplier, status: 'approved' | 'rejected') { const note = status === 'rejected' ? (prompt('驳回原因') || '') : ''; await adminSupplierAPI.review(item.id, status, note); await loadTab('suppliers', true) }
async function freeze(item: Supplier) { const reason = prompt('冻结原因'); if (reason === null) return; await adminSupplierAPI.freeze(item.id, reason); await loadTab('suppliers', true) }
async function unfreeze(item: Supplier) { if (!confirm(`确认解除 ${item.name} 的冻结状态？`)) return; await adminSupplierAPI.unfreeze(item.id); await loadTab('suppliers', true) }
async function reviewResource(id: number, approved: boolean) {
  const note = approved ? '' : (prompt('驳回原因') || '')
  error.value = ''
  try {
    await adminSupplierAPI.reviewResourceRequest(id, approved, note)
    await loadTab('resources', true)
  } catch (e) {
    error.value = extractApiErrorMessage(e, '资源审核失败')
  }
}
function openResourceTest(resource: SupplierResourceRequest) { testingResource.value = resource }
function closeResourceTest() { testingResource.value = null }
async function updateWithdrawal(id: number, status: 'approved' | 'rejected') { const note = status === 'rejected' ? (prompt('驳回原因') || '') : ''; await adminSupplierAPI.reviewWithdrawal(id, status, note); await loadTab('withdrawals', true) }
async function pay(id: number) { const proof = prompt('打款凭证存储键'); if (!proof) return; await adminSupplierAPI.reviewWithdrawal(id, 'paid', '', proof); await loadTab('withdrawals', true) }

onMounted(async () => {
  try { Object.assign(settings, await adminSupplierAPI.settings()) } catch (e) { error.value = e instanceof Error ? e.message : '设置加载失败' }
  await loadTab('suppliers')
})
</script>
