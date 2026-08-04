<template><div class="mx-auto max-w-6xl p-5 md:p-7"><header class="mb-6"><h1 class="text-xl font-semibold">供应商工作台</h1><p class="mt-1 text-sm text-gray-500">入驻审核通过后，可提交中转资源等待管理员审核。</p></header>
<div v-if="loading" class="py-20 text-center text-gray-400">加载中</div><form v-else-if="!profile||profile.status==='rejected'" class="max-w-2xl space-y-4 border-y py-6" @submit.prevent="submitSupplier"><label class="block text-sm">供应商名称<input v-model="supplierForm.name" required class="input mt-1 w-full" /></label><label class="block text-sm">中转站地址<input v-model="supplierForm.relay_url" type="url" required placeholder="https://" class="input mt-1 w-full" /></label><label class="block text-sm">申请说明<textarea v-model="supplierForm.application_note" rows="4" class="input mt-1 w-full" /></label><button class="rounded bg-teal-600 px-4 py-2 text-white">提交入驻审核</button></form>
<div v-else-if="profile.status!=='approved'" class="border-y py-12 text-center"><div class="font-medium">{{profile.status==='pending'?'入驻申请审核中':'供应商已冻结'}}</div><p class="mt-2 text-sm text-gray-500">{{profile.review_note||profile.freeze_reason}}</p></div>
<template v-else><form class="grid gap-4 border-y py-5 md:grid-cols-2" @submit.prevent="submitResource"><label class="text-sm">分组名称<input v-model="resourceForm.group_name" required class="input mt-1 w-full" /></label><label class="text-sm">中转站名称<input v-model="resourceForm.relay_name" required class="input mt-1 w-full" /></label><label class="text-sm">中转站地址<input v-model="resourceForm.relay_url" type="url" required placeholder="https://" class="input mt-1 w-full" /></label><label class="text-sm">API Key<input v-model="resourceForm.api_key" type="password" required autocomplete="new-password" class="input mt-1 w-full" /></label><label class="text-sm">监听模型<input v-model="resourceForm.model" readonly class="input mt-1 w-full bg-gray-50" /></label><div class="flex items-end"><button class="rounded bg-teal-600 px-4 py-2 text-white">提交资源审核</button></div></form>
<h2 class="mb-3 mt-8 font-semibold">资源申请</h2><div class="overflow-x-auto border-y"><table class="w-full text-sm"><thead><tr class="text-left text-gray-500"><th class="p-3">分组</th><th>中转站</th><th>模型</th><th>状态</th><th>审核备注</th></tr></thead><tbody><tr v-for="r in requests" :key="r.id" class="border-t"><td class="p-3">{{r.group_name}}</td><td>{{r.relay_name}}</td><td>{{r.model}}</td><td>{{r.status}}</td><td>{{r.review_note}}</td></tr></tbody></table></div></template>
</div></template>
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { supplierAPI, type Supplier, type SupplierResourceRequest } from '@/api/suppliers'
const loading = ref(true)
const profile = ref<Supplier | null>(null)
const requests = ref<SupplierResourceRequest[]>([])
const supplierForm = reactive({ name: '', relay_url: '', application_note: '' })
const resourceForm = reactive({ group_name: '', relay_name: '', relay_url: '', api_key: '', model: 'gpt-5.5' })
async function load() { try { const p = await supplierAPI.me(); profile.value = p; if (p.status === 'approved') requests.value = (await supplierAPI.resourceRequests()).items || [] } catch { profile.value = null } finally { loading.value = false } }
async function submitSupplier() { profile.value = await supplierAPI.apply(supplierForm) }
async function submitResource() { await supplierAPI.createResourceRequest(resourceForm); Object.assign(resourceForm, { group_name: '', relay_name: '', relay_url: '', api_key: '', model: 'gpt-5.5' }); await load() }
onMounted(load)
</script>
