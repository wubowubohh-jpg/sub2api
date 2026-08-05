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

      <section class="rounded-lg border bg-white px-5 py-4 shadow-sm dark:border-gray-800 dark:bg-gray-900">
        <div class="flex flex-wrap items-start justify-between gap-4 border-b pb-4 dark:border-gray-800">
          <div>
            <h2 class="font-medium text-gray-900 dark:text-white">供应商规则</h2>
            <p class="mt-1 text-xs text-gray-500">统一管理供应商分组加价和收益转为可提现余额的时间。</p>
          </div>
          <button
            class="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-teal-600 dark:hover:bg-teal-700"
            :disabled="settingsSaving || !settingsValid"
            @click="saveSettings"
          >
            {{ settingsSaving ? '保存中...' : '保存设置' }}
          </button>
        </div>
        <div class="grid gap-5 pt-4 md:grid-cols-2">
          <label class="block text-sm">
            <span class="font-medium text-gray-700 dark:text-gray-200">供应商全局倍率调整</span>
            <input v-model.number="settings.global_rate_adjustment" type="number" step="0.01" class="mt-2 block w-full max-w-xs rounded-lg border px-3 py-2 dark:border-gray-700 dark:bg-gray-950" />
            <span class="mt-1.5 block text-xs leading-5 text-gray-500">分组未设置单独调整时，按“供应商基础倍率 + 此调整值”计算有效倍率。</span>
          </label>
          <label class="block text-sm">
            <span class="font-medium text-gray-700 dark:text-gray-200">收益可提现等待时间</span>
            <div class="mt-2 flex max-w-xs items-center gap-2">
              <input v-model.number="settings.settlement_delay_days" type="number" min="0" max="365" step="1" class="block w-full rounded-lg border px-3 py-2 dark:border-gray-700 dark:bg-gray-950" />
              <span class="shrink-0 text-gray-500">天</span>
            </div>
            <span class="mt-1.5 block text-xs leading-5 text-gray-500">只影响保存后产生的收益；设置为 0 天时，收益当天进入可提现周期。</span>
          </label>
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
                <td class="space-x-3 pr-4 text-right"><button class="text-sky-700 hover:text-sky-800" @click="openSupplierBills(s)">查看账单</button><button v-if="s.status === 'pending'" class="text-emerald-700" @click="review(s, 'approved')">通过</button><button v-if="s.status === 'pending'" class="text-rose-700" @click="review(s, 'rejected')">驳回</button><button v-if="s.status === 'approved'" class="text-amber-700" @click="freeze(s)">冻结</button><button v-if="s.status === 'frozen'" class="text-emerald-700" @click="unfreeze(s)">解除冻结</button></td>
              </tr>
              <tr v-if="!supplierPage.length"><td colspan="6" class="p-16 text-center text-gray-400">暂无供应商记录</td></tr>
            </tbody>
          </table>
        </div>

        <div v-else-if="activeTab === 'resources'" class="overflow-x-auto">
          <table class="w-full min-w-[980px] text-sm">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-gray-950"><tr><th class="p-4">分组</th><th>中转站</th><th>地址</th><th>模型</th><th>状态</th><th class="pr-4 text-right">操作</th></tr></thead>
            <tbody>
              <tr v-for="r in resourcePage" :key="r.id" class="border-t transition hover:bg-gray-50 dark:border-gray-800 dark:hover:bg-gray-800/40"><td class="p-4"><div class="font-medium">{{ r.group_name }}</div><div class="mt-1 text-xs text-gray-500">供应商提交 {{ formatRate(r.rate_multiplier) }}</div><div class="mt-1 text-xs font-medium text-emerald-700">有效倍率 {{ formatRate(resourceEffectiveRate(r)) }}</div><div class="mt-1 max-w-64 text-xs text-gray-400">{{ resourceRateFormula(r) }}</div></td><td>{{ r.relay_name }}</td><td><a :href="r.relay_url" target="_blank" rel="noopener" class="text-sky-700 hover:underline">{{ r.relay_url }}</a></td><td><div class="flex max-w-64 flex-wrap gap-1"><span v-for="model in resourceModels(r)" :key="model" class="rounded bg-gray-100 px-1.5 py-0.5 text-xs dark:bg-gray-800">{{ model }}</span></div><div class="mt-1 text-xs text-gray-400">监听 {{ r.monitor_model || r.probe_model || r.model }}</div></td><td><span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(r.status)">{{ statusLabel(r.status) }}</span></td><td class="space-x-3 pr-4 text-right"><button class="text-sky-700 hover:text-sky-800" @click="openResourceEdit(r)">编辑资源</button><button v-if="r.status === 'pending'" class="text-sky-700 hover:text-sky-800" @click="openResourceTest(r)">模型测试</button><button v-if="r.status === 'pending'" class="text-emerald-700" @click="reviewResource(r.id, true)">通过并创建</button><button v-if="r.status === 'pending'" class="text-rose-700" @click="reviewResource(r.id, false)">驳回</button></td></tr>
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
    <BaseDialog
      :show="billSupplier !== null"
      :title="billSupplier ? `${billSupplier.name} · 收益账单` : '收益账单'"
      width="extra-wide"
      @close="closeSupplierBills"
    >
      <div class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-1 rounded-lg bg-gray-100 p-1 dark:bg-gray-800">
            <button
              v-for="filter in [{ label: '全部', value: '' }, { label: '待结算', value: 'pending' }, { label: '可提现', value: 'available' }, { label: '已冻结', value: 'frozen' }]"
              :key="filter.value || 'all'"
              type="button"
              class="rounded-md px-3 py-1.5 text-xs font-medium"
              :class="supplierBillsStatus === filter.value ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-700 dark:text-white' : 'text-gray-500 dark:text-gray-300'"
              @click="setSupplierBillsStatus(filter.value)"
            >
              {{ filter.label }}
            </button>
          </div>
          <span class="text-xs text-gray-500">共 {{ supplierBillsTotal }} 条记录</span>
        </div>
        <div v-if="supplierBillsLoading" class="py-16 text-center text-sm text-gray-400">正在加载账单...</div>
        <div v-else-if="supplierBills.length" class="overflow-x-auto rounded-lg border dark:border-gray-700">
          <table class="w-full min-w-[1250px] text-xs">
            <thead class="bg-gray-50 text-left text-gray-500 dark:bg-gray-800/70 dark:text-gray-300">
              <tr>
                <th class="px-3 py-3">时间</th>
                <th class="px-3 py-3">分组 / 模型</th>
                <th class="px-3 py-3">用户</th>
                <th class="px-3 py-3">账号 / API Key</th>
                <th class="px-3 py-3">请求</th>
                <th class="px-3 py-3">Token</th>
                <th class="px-3 py-3">倍率快照</th>
                <th class="px-3 py-3">模型原价</th>
                <th class="px-3 py-3">供应商收益</th>
                <th class="px-3 py-3">状态</th>
              </tr>
            </thead>
            <tbody class="divide-y dark:divide-gray-800">
              <tr v-for="bill in supplierBills" :key="bill.id" class="hover:bg-gray-50/70 dark:hover:bg-gray-800/40">
                <td class="whitespace-nowrap px-3 py-3 text-gray-500">{{ formatDate(bill.created_at) }}</td>
                <td class="px-3 py-3"><div class="font-medium text-gray-900 dark:text-white">{{ bill.group_name || `#${bill.group_id}` }}</div><div class="mt-1 text-gray-500">{{ bill.model || '--' }}</div></td>
                <td class="px-3 py-3"><div class="font-medium text-gray-900 dark:text-white">{{ bill.username || '--' }}</div><div class="mt-1 text-gray-500">{{ bill.user_email || (bill.user_id ? `ID ${bill.user_id}` : '--') }}</div></td>
                <td class="px-3 py-3 text-gray-500"><div>账号 {{ bill.account_id ?? '--' }}</div><div class="mt-1">Key {{ bill.api_key_id ?? '--' }}</div></td>
                <td class="max-w-48 truncate px-3 py-3 font-mono text-gray-500" :title="bill.request_id">{{ bill.request_id || (bill.usage_log_id ? `log #${bill.usage_log_id}` : '--') }}</td>
                <td class="whitespace-nowrap px-3 py-3 text-gray-600 dark:text-gray-300">{{ formatInteger(bill.input_tokens + bill.output_tokens) }}<span class="mt-1 block text-gray-400">缓存 {{ formatInteger(bill.cache_read_tokens) }}</span></td>
                <td class="whitespace-nowrap px-3 py-3 font-mono text-gray-600 dark:text-gray-300">{{ formatRate(bill.base_rate) }} + {{ formatRate(bill.admin_adjustment) }} = {{ formatRate(bill.effective_rate) }}</td>
                <td class="whitespace-nowrap px-3 py-3 font-mono text-gray-600 dark:text-gray-300">${{ bill.model_cost_usd.toFixed(6) }}<span class="mt-1 block text-gray-400">比例 {{ bill.recharge_ratio }}</span></td>
                <td class="whitespace-nowrap px-3 py-3 font-semibold text-emerald-700 dark:text-emerald-400">¥{{ money(bill.amount_cny) }}<span class="mt-1 block font-normal text-gray-400">{{ bill.entry_type }}</span></td>
                <td class="whitespace-nowrap px-3 py-3"><span class="rounded-full bg-gray-100 px-2 py-1 text-gray-600 dark:bg-gray-700 dark:text-gray-200">{{ statusLabel(bill.status) }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="py-16 text-center text-sm text-gray-400">暂无收益账单</div>
        <div class="flex items-center justify-end gap-3 border-t pt-3 text-xs text-gray-500 dark:border-gray-800">
          <button class="rounded-lg border px-3 py-1.5 disabled:opacity-40 dark:border-gray-700" :disabled="supplierBillsPage <= 1 || supplierBillsLoading" @click="changeSupplierBillsPage(-1)">上一页</button>
          <span>{{ supplierBillsPage }} / {{ supplierBillsPageCount }}</span>
          <button class="rounded-lg border px-3 py-1.5 disabled:opacity-40 dark:border-gray-700" :disabled="supplierBillsPage >= supplierBillsPageCount || supplierBillsLoading" @click="changeSupplierBillsPage(1)">下一页</button>
        </div>
      </div>
    </BaseDialog>
    <BaseDialog :show="editingResource !== null" title="编辑供应商资源" width="wide" @close="closeResourceEdit">
      <div v-if="editingResource" class="space-y-6">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b pb-4 dark:border-gray-800">
          <div>
            <div class="font-medium text-gray-900 dark:text-white">申请 #{{ editingResource.id }}</div>
            <div class="mt-1 text-xs text-gray-500">供应商 ID {{ editingResource.supplier_id }} · {{ statusLabel(editingResource.status) }}</div>
          </div>
          <div class="text-right text-xs text-gray-500">
            <div v-if="editingResource.group_id">分组 {{ editingResource.group_id }}</div>
            <div v-if="editingResource.account_id">账号 {{ editingResource.account_id }} · 监听 {{ editingResource.monitor_id }}</div>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <label class="block text-sm">
            <span class="input-label">大厅分组后缀</span>
            <input v-model.trim="resourceEditForm.group_name" maxlength="90" class="input mt-1 w-full" />
          </label>
          <label class="block text-sm">
            <span class="input-label">中转站名称</span>
            <input v-model.trim="resourceEditForm.relay_name" maxlength="100" class="input mt-1 w-full" />
          </label>
          <label class="block text-sm md:col-span-2">
            <span class="input-label">API 基础地址</span>
            <input v-model.trim="resourceEditForm.relay_url" type="url" class="input mt-1 w-full" />
          </label>
          <label class="block text-sm md:col-span-2">
            <span class="input-label">替换 API Key</span>
            <input v-model="resourceEditForm.api_key" type="password" autocomplete="new-password" placeholder="留空保持当前凭据" class="input mt-1 w-full" />
          </label>
          <label class="block text-sm">
            <span class="input-label">供应商基础倍率</span>
            <input v-model.number="resourceEditForm.rate_multiplier" type="number" min="0" step="0.0001" class="input mt-1 w-full" />
          </label>
          <label class="block text-sm">
            <span class="input-label">管理员增加倍率</span>
            <input v-model.number="resourceEditForm.admin_rate_adjustment" type="number" min="0" step="0.0001" class="input mt-1 w-full" :disabled="!editingResource.group_id" />
            <span v-if="!editingResource.group_id" class="mt-1 block text-xs text-amber-600">审核通过并创建分组后可设置。</span>
          </label>
        </div>

        <fieldset>
          <legend class="text-sm font-medium text-gray-700 dark:text-gray-200">支持模型</legend>
          <div class="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            <label
              v-for="model in resourceModelCatalog"
              :key="model"
              class="flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm"
              :class="resourceEditForm.supported_models.includes(model) ? 'border-teal-300 bg-teal-50 text-teal-800 dark:border-teal-700 dark:bg-teal-900/20 dark:text-teal-200' : 'border-gray-200 text-gray-600 dark:border-gray-700 dark:text-gray-300'"
            >
              <input :checked="resourceEditForm.supported_models.includes(model)" type="checkbox" class="h-4 w-4 rounded" @change="toggleResourceModel(model)" />
              <span class="truncate">{{ model }}</span>
            </label>
          </div>
          <div class="mt-3 flex gap-2">
            <input v-model.trim="customResourceModel" class="input min-w-0 flex-1" placeholder="添加其他模型 ID" @keydown.enter.prevent="addResourceModel" />
            <button type="button" class="btn btn-secondary" @click="addResourceModel">添加模型</button>
          </div>
        </fieldset>

        <div class="grid gap-4 md:grid-cols-2">
          <label class="block text-sm">
            <span class="input-label">默认监听模型</span>
            <select v-model="resourceEditForm.monitor_model" class="input mt-1 w-full">
              <option v-for="model in resourceEditForm.supported_models" :key="model" :value="model">{{ model }}</option>
            </select>
          </label>
          <div class="flex items-center justify-between rounded-lg border px-4 py-3 dark:border-gray-700">
            <div class="text-sm font-medium text-gray-700 dark:text-gray-200">探测上游倍率</div>
            <Toggle v-model="resourceEditForm.upstream_billing_probe_enabled" />
          </div>
          <label class="block text-sm md:col-span-2">
            <span class="input-label">审核备注</span>
            <textarea v-model="resourceEditForm.review_note" rows="3" class="input mt-1 w-full resize-y" />
          </label>
        </div>

        <div class="rounded-lg bg-gray-50 px-4 py-3 text-sm text-gray-600 dark:bg-gray-800 dark:text-gray-300">
          {{ resourceEditPreview }}
        </div>
      </div>
      <template #footer>
        <button class="btn btn-secondary" :disabled="resourceSaving" @click="closeResourceEdit">取消</button>
        <button class="btn btn-primary" :disabled="resourceSaving || !resourceEditValid" @click="saveResourceEdit">
          {{ resourceSaving ? '保存中' : '保存全部修改' }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AccountTestModal from '@/components/admin/account/AccountTestModal.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { adminSupplierAPI, type Supplier, type SupplierAdminBill, type SupplierResourceRequest, type SupplierWithdrawal } from '@/api/suppliers'
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
const billSupplier = ref<Supplier | null>(null)
const supplierBills = ref<SupplierAdminBill[]>([])
const supplierBillsTotal = ref(0)
const supplierBillsPage = ref(1)
const supplierBillsPageSize = 20
const supplierBillsLoading = ref(false)
const supplierBillsStatus = ref('')
const testingResource = ref<SupplierResourceRequest | null>(null)
const editingResource = ref<SupplierResourceRequest | null>(null)
const resourceSaving = ref(false)
const customResourceModel = ref('')
const settingsSaving = ref(false)
const builtInResourceModels = [
  'gpt-5.4',
  'gpt-5.4-mini',
  'gpt-5.5',
  'gpt-5.6',
  'gpt-5.6-sol',
  'gpt-5.6-terra',
  'gpt-5.6-luna',
  'gpt-5.3-codex-spark',
]
const resourceEditForm = reactive({
  group_name: '',
  relay_name: '',
  relay_url: '',
  api_key: '',
  rate_multiplier: 0,
  admin_rate_adjustment: 0,
  monitor_model: '',
  supported_models: [] as string[],
  upstream_billing_probe_enabled: true,
  review_note: '',
})
const error = ref('')
const settings = reactive({ global_rate_adjustment: 0, settlement_delay_days: 7 })
const settingsValid = computed(() => {
  const adjustment = Number(settings.global_rate_adjustment)
  const delayDays = Number(settings.settlement_delay_days)
  return Number.isFinite(adjustment)
    && Number.isInteger(delayDays)
    && delayDays >= 0
    && delayDays <= 365
})

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
const supplierBillsPageCount = computed(() => Math.max(1, Math.ceil(supplierBillsTotal.value / supplierBillsPageSize)))
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
const resourceModelCatalog = computed(() => [...new Set([...builtInResourceModels, ...resourceEditForm.supported_models])])
const resourceEditValid = computed(() => {
  const rate = Number(resourceEditForm.rate_multiplier)
  const adjustment = Number(resourceEditForm.admin_rate_adjustment)
  return Boolean(
    resourceEditForm.group_name.trim()
    && resourceEditForm.relay_name.trim()
    && resourceEditForm.relay_url.trim()
    && Number.isFinite(rate)
    && rate >= 0
    && Number.isFinite(adjustment)
    && adjustment >= 0
    && resourceEditForm.supported_models.length
    && resourceEditForm.supported_models.includes(resourceEditForm.monitor_model),
  )
})
const resourceEditPreview = computed(() => {
  if (!editingResource.value) return ''
  const rate = Number(resourceEditForm.rate_multiplier)
  const adjustment = editingResource.value.group_id
    ? Number(resourceEditForm.admin_rate_adjustment)
    : resourceAdjustment(editingResource.value)
  return `有效倍率 ${formatRate(rate + adjustment)} = 基础倍率 ${formatRate(rate)} + 管理员增加 ${formatRate(adjustment)}`
})

function resourceModels(resource: SupplierResourceRequest) {
  return resource.supported_models?.length ? resource.supported_models : [resource.model].filter(Boolean)
}

function formatRate(value: unknown) {
  const rate = Number(value)
  return Number.isFinite(rate) ? rate.toFixed(4) : '--'
}

function formatDate(value: string | undefined) {
  if (!value) return '--'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function formatInteger(value: unknown) {
  const number = Number(value || 0)
  return Number.isFinite(number) ? number.toLocaleString() : '0'
}

function resourceAppliedRate(resource: SupplierResourceRequest) {
  const serverRate = Number(resource.applied_rate_multiplier)
  if (Number.isFinite(serverRate)) return serverRate
  return Number(resource.rate_multiplier || 0)
}

function resourceAdjustment(resource: SupplierResourceRequest) {
  const adjustment = Number(resource.admin_rate_adjustment ?? 0)
  return Number.isFinite(adjustment) ? adjustment : 0
}

function resourceEffectiveRate(resource: SupplierResourceRequest) {
  const serverRate = Number(resource.effective_rate_multiplier)
  return Number.isFinite(serverRate) ? serverRate : resourceAppliedRate(resource) + resourceAdjustment(resource)
}

function resourceRateFormula(resource: SupplierResourceRequest) {
  return `设置倍率 ${formatRate(resourceAppliedRate(resource))} + 管理员增加 ${formatRate(resourceAdjustment(resource))}`
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

async function loadSupplierBills() {
  if (!billSupplier.value) return
  supplierBillsLoading.value = true
  try {
    const response = await adminSupplierAPI.bills(
      billSupplier.value.id,
      supplierBillsStatus.value,
      supplierBillsPageSize,
      (supplierBillsPage.value - 1) * supplierBillsPageSize,
    )
    supplierBills.value = response.items || []
    supplierBillsTotal.value = Number(response.total || 0)
    supplierBillsPage.value = Math.min(supplierBillsPage.value, supplierBillsPageCount.value)
  } catch (e) {
    error.value = extractApiErrorMessage(e, '收益账单加载失败')
  } finally {
    supplierBillsLoading.value = false
  }
}

async function openSupplierBills(item: Supplier) {
  billSupplier.value = item
  supplierBillsStatus.value = ''
  supplierBillsPage.value = 1
  supplierBills.value = []
  supplierBillsTotal.value = 0
  await loadSupplierBills()
}

function closeSupplierBills() {
  if (!supplierBillsLoading.value) billSupplier.value = null
}

function setSupplierBillsStatus(value: string) {
  supplierBillsStatus.value = value
  supplierBillsPage.value = 1
  void loadSupplierBills()
}

function changeSupplierBillsPage(delta: number) {
  const next = Math.min(supplierBillsPageCount.value, Math.max(1, supplierBillsPage.value + delta))
  if (next === supplierBillsPage.value) return
  supplierBillsPage.value = next
  void loadSupplierBills()
}

async function saveSettings() {
  if (!settingsValid.value || settingsSaving.value) return
  settingsSaving.value = true
  try {
    Object.assign(settings, await adminSupplierAPI.updateSettings(settings))
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '设置保存失败'
  } finally {
    settingsSaving.value = false
  }
}
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
function resourceGroupSuffix(resource: SupplierResourceRequest) {
  return resource.group_name_suffix || resource.group_name.replace(/^A\d+-/, '')
}

function openResourceEdit(resource: SupplierResourceRequest) {
  editingResource.value = resource
  resourceEditForm.group_name = resourceGroupSuffix(resource)
  resourceEditForm.relay_name = resource.relay_name
  resourceEditForm.relay_url = resource.relay_url
  resourceEditForm.api_key = ''
  resourceEditForm.rate_multiplier = Number(resource.rate_multiplier || 0)
  resourceEditForm.admin_rate_adjustment = resourceAdjustment(resource)
  resourceEditForm.supported_models = [...resourceModels(resource)]
  resourceEditForm.monitor_model = resource.monitor_model || resource.probe_model || resource.model || resourceEditForm.supported_models[0] || ''
  resourceEditForm.upstream_billing_probe_enabled = resource.upstream_billing_probe_enabled ?? resource.probe_enabled ?? true
  resourceEditForm.review_note = resource.review_note || ''
  customResourceModel.value = ''
}

function closeResourceEdit() {
  if (resourceSaving.value) return
  editingResource.value = null
}

function toggleResourceModel(model: string) {
  const index = resourceEditForm.supported_models.indexOf(model)
  if (index >= 0) {
    resourceEditForm.supported_models.splice(index, 1)
    if (resourceEditForm.monitor_model === model) {
      resourceEditForm.monitor_model = resourceEditForm.supported_models[0] || ''
    }
  } else {
    resourceEditForm.supported_models.push(model)
    if (!resourceEditForm.monitor_model) resourceEditForm.monitor_model = model
  }
}

function addResourceModel() {
  const model = customResourceModel.value.trim()
  if (!model) return
  if (!resourceEditForm.supported_models.includes(model)) resourceEditForm.supported_models.push(model)
  if (!resourceEditForm.monitor_model) resourceEditForm.monitor_model = model
  customResourceModel.value = ''
}

async function saveResourceEdit() {
  if (!editingResource.value || !resourceEditValid.value) return
  resourceSaving.value = true
  error.value = ''
  try {
    const resource = editingResource.value
    const updated = await adminSupplierAPI.updateResourceRequest(resource.id, {
      group_name: resourceEditForm.group_name.trim(),
      relay_name: resourceEditForm.relay_name.trim(),
      relay_url: resourceEditForm.relay_url.trim(),
      api_key: resourceEditForm.api_key.trim() || undefined,
      monitor_model: resourceEditForm.monitor_model,
      supported_models: [...resourceEditForm.supported_models],
      upstream_billing_probe_enabled: resourceEditForm.upstream_billing_probe_enabled,
      rate_multiplier: Number(resourceEditForm.rate_multiplier),
      admin_rate_adjustment: resource.group_id ? Number(resourceEditForm.admin_rate_adjustment) : undefined,
      review_note: resourceEditForm.review_note,
    })
    const index = resourceRequests.value.findIndex(item => item.id === updated.id)
    if (index >= 0) resourceRequests.value[index] = updated
    editingResource.value = null
  } catch (e) {
    error.value = extractApiErrorMessage(e, '资源更新失败')
  } finally {
    resourceSaving.value = false
  }
}
async function updateWithdrawal(id: number, status: 'approved' | 'rejected') { const note = status === 'rejected' ? (prompt('驳回原因') || '') : ''; await adminSupplierAPI.reviewWithdrawal(id, status, note); await loadTab('withdrawals', true) }
async function pay(id: number) { const proof = prompt('打款凭证存储键'); if (!proof) return; await adminSupplierAPI.reviewWithdrawal(id, 'paid', '', proof); await loadTab('withdrawals', true) }

onMounted(async () => {
  try { Object.assign(settings, await adminSupplierAPI.settings()) } catch (e) { error.value = e instanceof Error ? e.message : '设置加载失败' }
  await loadTab('suppliers')
})
</script>
