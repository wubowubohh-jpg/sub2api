<template>
  <div class="min-h-full bg-gray-50 p-4 dark:bg-gray-950 md:p-6">
    <div class="mx-auto max-w-[1600px]">
      <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div><h1 class="text-xl font-semibold text-gray-900 dark:text-white">供应商大厅</h1><p class="mt-1 text-sm text-gray-500">查看启用分组的实时质量与实际倍率</p></div>
        <div class="flex items-center gap-2"><span class="rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs text-emerald-700">监测中</span><button class="rounded border border-gray-300 bg-white px-3 py-1.5 text-sm dark:border-gray-700 dark:bg-gray-900" @click="load">刷新</button></div>
      </div>
      <div class="mb-4 flex gap-1 rounded border border-gray-200 bg-white p-1 dark:border-gray-800 dark:bg-gray-900" style="width:max-content">
        <button v-for="w in windows" :key="w" class="rounded px-3 py-1.5 text-sm" :class="window===w?'bg-teal-600 text-white':'text-gray-600 dark:text-gray-300'" @click="window=w">{{w}}</button>
      </div>
      <div class="overflow-x-auto border-y border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
        <table class="w-full min-w-[1180px] text-left text-sm"><thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-gray-800"><tr><th class="p-4">分组</th><th>供应商</th><th>倍率</th><th>状态</th><th>首Token</th><th>缓存命中率</th><th>可用率</th><th class="w-[300px]">趋势</th><th>操作</th></tr></thead>
        <tbody><tr v-for="g in groups" :key="g.id" class="border-b border-gray-100 dark:border-gray-800"><td class="p-4"><div class="font-semibold text-gray-900 dark:text-white">{{g.name}}</div><div class="mt-1 text-xs text-gray-500">{{g.platform}}</div></td><td>{{g.supplier_name||'平台自营'}}</td><td><span class="font-medium">{{g.effective_rate.toFixed(4)}}</span><div v-if="g.supplier_id" class="text-xs text-gray-400">{{g.base_rate.toFixed(4)}} {{g.admin_adjustment>=0?'+':''}}{{g.admin_adjustment.toFixed(4)}}</div></td><td><span class="rounded-full bg-emerald-50 px-2 py-1 text-xs text-emerald-700">可用</span></td><td :class="g.metrics.avg_first_token_ms==null?'text-gray-400':''">{{g.metrics.avg_first_token_ms==null?'暂无数据':Math.round(g.metrics.avg_first_token_ms)+' ms'}}</td><td :class="g.metrics.cache_hit_rate==null?'text-gray-400':''">{{g.metrics.cache_hit_rate==null?'暂无数据':g.metrics.cache_hit_rate.toFixed(1)+'%'}}</td><td :class="g.metrics.availability==null?'text-gray-400':''">{{g.metrics.availability==null?'暂无数据':g.metrics.availability.toFixed(1)+'%'}}</td><td><div class="h-10 w-[270px] border-b border-dashed border-teal-300"></div><div class="text-xs text-gray-400">{{g.metrics.tps==null?'暂无数据':g.metrics.tps.toFixed(1)+' tokens/s'}}</div></td><td><RouterLink :to="{path:'/keys',query:{group:String(g.id)}}" class="rounded border border-sky-300 px-3 py-1.5 text-sky-700">使用此分组</RouterLink></td></tr><tr v-if="!loading&&!groups.length"><td colspan="9" class="p-16 text-center text-gray-400">暂无启用分组</td></tr></tbody>
        </table>
      </div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { onMounted,ref } from 'vue'; import { supplierAPI,type HallGroup } from '@/api/suppliers'
const windows=['6h','24h','7d','30d'];const window=ref('6h');const groups=ref<HallGroup[]>([]);const loading=ref(false)
async function load(){loading.value=true;try{groups.value=(await supplierAPI.hall(window.value)).groups}finally{loading.value=false}}onMounted(load)
</script>
