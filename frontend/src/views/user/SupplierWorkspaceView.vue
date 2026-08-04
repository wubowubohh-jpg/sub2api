<template><div class="mx-auto max-w-6xl p-5 md:p-7"><div class="mb-6 flex items-center justify-between"><div><h1 class="text-xl font-semibold">供应商工作台</h1><p class="mt-1 text-sm text-gray-500">入驻资料、分组、账号和结算</p></div><RouterLink to="/supplier-hall" class="text-sm text-teal-700">查看大厅</RouterLink></div>
  <div v-if="loading" class="py-20 text-center text-gray-400">加载中</div>
  <form v-else-if="!profile||profile.status==='rejected'" class="max-w-2xl space-y-4 border-y border-gray-200 py-6" @submit.prevent="submit"><label class="block text-sm">供应商名称<input v-model="form.name" required class="mt-1 w-full rounded border p-2.5 dark:bg-gray-900" /></label><label class="block text-sm">中转站地址<input v-model="form.relay_url" type="url" required placeholder="https://" class="mt-1 w-full rounded border p-2.5 dark:bg-gray-900" /></label><label class="block text-sm">申请说明<textarea v-model="form.application_note" rows="4" class="mt-1 w-full rounded border p-2.5 dark:bg-gray-900"></textarea></label><button class="rounded bg-teal-600 px-4 py-2 text-white">提交申请</button></form>
  <div v-else-if="profile.status!=='approved'" class="border-y py-12 text-center"><div class="font-medium">{{profile.status==='pending'?'申请审核中':'供应商已冻结'}}</div><p class="mt-2 text-sm text-gray-500">{{profile.review_note||profile.freeze_reason}}</p></div>
  <template v-else><div class="mb-6 grid grid-cols-3 gap-3"><div v-for="b in balances" :key="b.label" class="border-y border-gray-200 py-4"><div class="text-xs text-gray-500">{{b.label}}</div><div class="mt-1 text-xl font-semibold">¥{{b.value.toFixed(2)}}</div></div></div>
    <div class="mb-4 flex items-center justify-between"><h2 class="font-semibold">我的分组</h2><button class="rounded bg-teal-600 px-3 py-2 text-sm text-white" @click="newGroup">新增分组</button></div><div class="overflow-x-auto border-y"><table class="w-full text-sm"><thead><tr class="text-left text-gray-500"><th class="p-3">名称</th><th>平台</th><th>基础倍率</th><th>状态</th><th></th></tr></thead><tbody><tr v-for="g in groups" :key="g.id" class="border-t"><td class="p-3 font-medium">{{g.name}}</td><td>{{g.platform}}</td><td>{{g.rate_multiplier}}</td><td>{{g.status}}</td><td><button class="text-teal-700" @click="editGroup(g)">编辑</button></td></tr></tbody></table></div>
  </template>
  <div v-if="editing" class="fixed inset-0 z-50 grid place-items-center bg-black/40 p-4"><form class="w-full max-w-lg rounded bg-white p-5 shadow-xl dark:bg-gray-900" @submit.prevent="saveGroup"><h3 class="mb-4 font-semibold">{{editing.id?'编辑分组':'新增分组'}}</h3><div class="grid gap-3"><input v-model="editing.name" required placeholder="分组名称" class="rounded border p-2"/><select v-model="editing.platform" class="rounded border p-2"><option value="openai">OpenAI</option><option value="anthropic">Anthropic</option><option value="gemini">Gemini</option><option value="grok">Grok</option></select><input v-model.number="editing.rate_multiplier" type="number" min="0" step="0.0001" class="rounded border p-2"/><select v-model="editing.status" class="rounded border p-2"><option value="active">启用</option><option value="disabled">停用</option></select></div><div class="mt-5 flex justify-end gap-2"><button type="button" class="px-3 py-2" @click="editing=null">取消</button><button class="rounded bg-teal-600 px-4 py-2 text-white">保存</button></div></form></div>
</div></template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { supplierAPI, type Supplier, type SupplierGroup } from '@/api/suppliers'

const loading = ref(true)
const profile = ref<Supplier | null>(null)
const groups = ref<SupplierGroup[]>([])
const editing = ref<any>(null)
const form = reactive({ name: '', relay_url: '', application_note: '' })
const balances = computed(() => [
  { label: '待结算', value: profile.value?.pending_balance_cny || 0 },
  { label: '可提现', value: profile.value?.available_balance_cny || 0 },
  { label: '提现中', value: profile.value?.frozen_balance_cny || 0 },
])
async function load() {
  try {
    profile.value = await supplierAPI.me()
    if (profile.value.status === 'approved') groups.value = await supplierAPI.groups()
  } catch {
    profile.value = null
  } finally {
    loading.value = false
  }
}
async function submit() { profile.value = await supplierAPI.apply(form) }
function newGroup() { editing.value = { name: '', platform: 'openai', subscription_type: 'standard', rate_multiplier: 1, status: 'disabled', is_exclusive: false, sort_order: 0 } }
function editGroup(item: SupplierGroup) { editing.value = { ...item } }
async function saveGroup() {
  const saved = editing.value.id
    ? await supplierAPI.updateGroup(editing.value.id, editing.value)
    : await supplierAPI.createGroup(editing.value)
  const index = groups.value.findIndex((item) => item.id === saved.id)
  if (index >= 0) groups.value[index] = saved
  else groups.value.push(saved)
  editing.value = null
}
onMounted(load)
</script>
