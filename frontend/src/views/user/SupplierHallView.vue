<template>
  <AppLayout>
  <div class="min-h-full bg-gray-50 p-4 dark:bg-gray-950 md:p-6">
    <div class="mx-auto max-w-[1600px]">
      <div class="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div><h1 class="text-xl font-semibold text-gray-900 dark:text-white">供应商大厅</h1><p class="mt-1 text-sm text-gray-500">查看启用分组的实时质量与实际倍率</p></div>
        <div class="flex items-center gap-2"><span class="inline-flex items-center gap-1.5 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-xs font-medium text-emerald-700"><span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>监测中</span><button class="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm shadow-sm transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900" @click="load">刷新</button></div>
      </div>
      <div class="mb-5 flex w-full max-w-md gap-1 rounded-xl border border-gray-200 bg-white p-1 shadow-sm dark:border-gray-800 dark:bg-gray-900">
        <button v-for="w in windows" :key="w" class="flex-1 rounded-lg px-3 py-2 text-sm transition" :class="window===w?'bg-teal-600 font-medium text-white shadow-sm':'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800'" @click="window=w">{{w}}</button>
      </div>
      <div v-if="loading" class="border border-gray-200 bg-white px-6 py-20 text-center text-sm text-gray-400 dark:border-gray-800 dark:bg-gray-900">正在刷新分组监测数据...</div>
      <div v-else class="overflow-x-auto rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
        <table class="w-full min-w-[1180px] text-left text-sm"><thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-gray-800"><tr><th class="p-4">分组</th><th>供应商</th><th>倍率</th><th>状态</th><th>首Token</th><th>缓存命中率</th><th>可用率</th><th class="w-[300px]">趋势</th><th>操作</th></tr></thead>
        <tbody><tr v-for="g in groups" :key="g.id" class="border-b border-gray-100 transition hover:bg-gray-50/70 dark:border-gray-800 dark:hover:bg-gray-800/40"><td class="p-4"><div class="font-semibold text-gray-900 dark:text-white">{{g.name}}</div><div class="mt-1 text-xs text-gray-500">{{g.platform}} · {{g.supplier_name||'平台自营'}}</div></td><td><div class="font-medium">{{g.effective_rate.toFixed(4)}}</div><div v-if="g.supplier_id" class="mt-1 text-xs text-gray-400">基础 {{g.base_rate.toFixed(4)}} · 调整 {{g.admin_adjustment>=0?'+':''}}{{g.admin_adjustment.toFixed(4)}}</div></td><td><span class="inline-flex rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700">可用</span></td><td :class="g.metrics.avg_first_token_ms==null?'text-gray-400':'font-medium'">{{g.metrics.avg_first_token_ms==null?'暂无数据':Math.round(g.metrics.avg_first_token_ms)+' ms'}}</td><td :class="g.metrics.cache_hit_rate==null?'text-gray-400':''">{{g.metrics.cache_hit_rate==null?'暂无数据':g.metrics.cache_hit_rate.toFixed(1)+'%'}}</td><td :class="g.metrics.availability==null?'text-gray-400':''">{{g.metrics.availability==null?'暂无数据':g.metrics.availability.toFixed(1)+'%'}}</td><td><div class="h-10 w-[270px] border-b border-dashed border-teal-300"></div><div class="text-xs text-gray-400">{{g.metrics.tps==null?'暂无数据':g.metrics.tps.toFixed(1)+' tokens/s'}}</div></td><td><RouterLink :to="{path:'/keys',query:{group:String(g.id)}}" class="inline-flex rounded-lg border border-sky-300 px-3 py-1.5 text-sky-700 transition hover:bg-sky-50">使用此分组</RouterLink></td></tr><tr v-if="!groups.length"><td colspan="8" class="p-16 text-center text-gray-400">暂无启用分组</td></tr></tbody>
        </table>
      </div>
    </div>
  </div>
  </AppLayout>
</template>
<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import { onMounted,ref } from 'vue'; import { supplierAPI,type HallGroup } from '@/api/suppliers'
const windows=['6h','24h','7d','30d'];const window=ref('6h');const groups=ref<HallGroup[]>([]);const loading=ref(false)
async function load(){loading.value=true;try{groups.value=(await supplierAPI.hall(window.value)).groups}finally{loading.value=false}}onMounted(load)
</script>
