<template>
  <div class="p-4 md:p-6 space-y-8">
    <header><h1 class="text-xl font-semibold">供应商管理</h1><p class="mt-1 text-sm text-gray-500">审核入驻、调整倍率、冻结供应商与处理提现。</p></header>
    <section class="flex flex-wrap items-end gap-4 border-y py-4">
      <label class="text-sm">全局倍率调整<input v-model.number="settings.global_rate_adjustment" type="number" step="0.01" class="mt-1 block w-40 rounded border px-3 py-2" /></label>
      <label class="text-sm">最低提现（USD）<input v-model.number="settings.minimum_withdrawal_usd" type="number" min="0.01" step="1" class="mt-1 block w-40 rounded border px-3 py-2" /></label>
      <button class="rounded bg-gray-900 px-4 py-2 text-sm text-white" @click="saveSettings">保存设置</button>
    </section>
    <section><h2 class="mb-3 font-medium">入驻与余额</h2><div class="overflow-x-auto border-y"><table class="w-full text-sm"><thead><tr class="text-left text-gray-500"><th class="p-3">供应商</th><th>中转站</th><th>状态</th><th>待结算</th><th>可提现</th><th>操作</th></tr></thead><tbody><tr v-for="s in items" :key="s.id" class="border-t"><td class="p-3 font-medium">{{s.name}}</td><td><a :href="s.relay_url" target="_blank" rel="noopener" class="text-sky-700">{{s.relay_url}}</a></td><td>{{s.status}}</td><td>¥{{s.pending_balance_cny.toFixed(2)}}</td><td>¥{{s.available_balance_cny.toFixed(2)}}</td><td class="space-x-2"><button v-if="s.status==='pending'" class="text-emerald-700" @click="review(s,'approved')">通过</button><button v-if="s.status==='pending'" class="text-rose-700" @click="review(s,'rejected')">驳回</button><button v-if="s.status==='approved'" class="text-amber-700" @click="freeze(s)">冻结</button></td></tr></tbody></table></div></section>
    <section><h2 class="mb-3 font-medium">提现审核</h2><div class="overflow-x-auto border-y"><table class="w-full text-sm"><thead><tr class="text-left text-gray-500"><th class="p-3">申请号</th><th>供应商</th><th>金额</th><th>方式</th><th>状态</th><th>操作</th></tr></thead><tbody><tr v-for="w in withdrawals" :key="w.id" class="border-t"><td class="p-3 font-mono text-xs">{{w.request_no}}</td><td>{{w.supplier_id}}</td><td>¥{{w.amount_cny.toFixed(2)}}</td><td>{{w.method}}</td><td>{{w.status}}</td><td class="space-x-2"><button v-if="w.status==='pending'" class="text-emerald-700" @click="updateWithdrawal(w.id,'approved')">通过</button><button v-if="w.status==='pending'" class="text-rose-700" @click="updateWithdrawal(w.id,'rejected')">驳回</button><button v-if="w.status==='approved'" class="text-sky-700" @click="pay(w.id)">标记已打款</button></td></tr></tbody></table></div></section>
  </div>
</template>
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { adminSupplierAPI, type Supplier, type SupplierWithdrawal } from '@/api/suppliers'
const items = ref<Supplier[]>([])
const withdrawals = ref<SupplierWithdrawal[]>([])
const settings = reactive({ global_rate_adjustment: 0, minimum_withdrawal_usd: 100 })
async function load() { const [s,w,c] = await Promise.all([adminSupplierAPI.list(),adminSupplierAPI.withdrawals(),adminSupplierAPI.settings()]); items.value=s.items; withdrawals.value=w.items; Object.assign(settings,c) }
async function saveSettings(){ Object.assign(settings,await adminSupplierAPI.updateSettings(settings)) }
async function review(item:Supplier,status:'approved'|'rejected'){const note=status==='rejected'?(prompt('驳回原因')||''):'';await adminSupplierAPI.review(item.id,status,note);await load()}
async function freeze(item:Supplier){const reason=prompt('冻结原因');if(reason===null)return;await adminSupplierAPI.freeze(item.id,reason);await load()}
async function updateWithdrawal(id:number,status:'approved'|'rejected'){const note=status==='rejected'?(prompt('驳回原因')||''):'';await adminSupplierAPI.reviewWithdrawal(id,status,note);await load()}
async function pay(id:number){const proof=prompt('打款凭证存储键');if(!proof)return;await adminSupplierAPI.reviewWithdrawal(id,'paid','',proof);await load()}
onMounted(load)
</script>
