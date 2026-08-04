<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <header class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div class="flex flex-wrap items-center gap-3">
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">供应商工作台</h1>
            <span
              v-if="profile"
              class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
              :class="profileStatusClass"
            >
              <span class="h-1.5 w-1.5 rounded-full bg-current" />
              {{ profileStatusLabel }}
            </span>
          </div>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ profile?.name || '完成入驻审核后即可管理中转资源与收益。' }}
          </p>
        </div>
        <RouterLink
          v-if="profile?.status === 'approved'"
          to="/supplier-hall"
          class="btn btn-secondary inline-flex items-center gap-2 self-start"
        >
          <Icon name="globe" size="sm" />
          供应商大厅
        </RouterLink>
      </header>

      <div
        v-if="loading"
        class="flex min-h-[320px] items-center justify-center rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
      >
        <LoadingSpinner />
      </div>

      <form
        v-else-if="!profile || profile.status === 'rejected'"
        class="max-w-2xl rounded-lg border border-gray-200 bg-white p-5 shadow-sm sm:p-7 dark:border-dark-700 dark:bg-dark-900"
        @submit.prevent="submitSupplier"
      >
        <div class="flex items-start gap-3 border-b border-gray-100 pb-5 dark:border-dark-700">
          <div class="rounded-lg bg-primary-50 p-2.5 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300">
            <Icon name="badge" size="lg" />
          </div>
          <div>
            <h2 class="font-semibold text-gray-900 dark:text-white">
              {{ profile?.status === 'rejected' ? '重新提交入驻资料' : '申请供应商入驻' }}
            </h2>
            <p v-if="profile?.review_note" class="mt-1 text-sm text-rose-600 dark:text-rose-300">
              审核备注：{{ profile.review_note }}
            </p>
          </div>
        </div>

        <div class="mt-5 space-y-5">
          <label class="block">
            <span class="input-label">供应商名称</span>
            <input
              v-model.trim="supplierForm.name"
              required
              maxlength="100"
              placeholder="公司、团队或个人品牌名称"
              class="input mt-2 w-full"
            />
          </label>
          <label class="block">
            <span class="input-label">官方网站或中转站地址</span>
            <input
              v-model.trim="supplierForm.relay_url"
              type="url"
              required
              placeholder="https://example.com"
              class="input mt-2 w-full"
            />
          </label>
          <label class="block">
            <span class="input-label">申请说明</span>
            <textarea
              v-model.trim="supplierForm.application_note"
              rows="4"
              maxlength="2000"
              placeholder="资源来源、服务能力与联系信息"
              class="input mt-2 w-full resize-y"
            />
          </label>
        </div>

        <div class="mt-6 flex justify-end">
          <button type="submit" class="btn btn-primary" :disabled="submitting">
            <LoadingSpinner v-if="submitting" size="sm" color="white" />
            <Icon v-else name="upload" size="sm" />
            {{ submitting ? '提交中' : '提交入驻审核' }}
          </button>
        </div>
      </form>

      <section
        v-else-if="profile.status !== 'approved'"
        class="rounded-lg border border-gray-200 bg-white px-5 py-14 text-center shadow-sm dark:border-dark-700 dark:bg-dark-900"
      >
        <div
          class="mx-auto flex h-12 w-12 items-center justify-center rounded-full"
          :class="profile.status === 'frozen' ? 'bg-rose-50 text-rose-600 dark:bg-rose-900/20' : 'bg-amber-50 text-amber-600 dark:bg-amber-900/20'"
        >
          <Icon :name="profile.status === 'frozen' ? 'ban' : 'clock'" size="lg" />
        </div>
        <h2 class="mt-4 font-semibold text-gray-900 dark:text-white">
          {{ profile.status === 'pending' ? '入驻资料审核中' : '供应商账号已冻结' }}
        </h2>
        <p class="mx-auto mt-2 max-w-lg text-sm text-gray-500 dark:text-dark-400">
          {{ profile.review_note || profile.freeze_reason || '请等待管理员处理。' }}
        </p>
      </section>

      <template v-else>
        <nav
          class="grid grid-cols-3 overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900"
          aria-label="供应商工作台模块"
        >
          <RouterLink
            v-for="item in moduleNav"
            :key="item.name"
            :to="{ name: item.name }"
            class="flex min-h-[68px] items-center justify-center gap-2 border-r border-gray-200 px-2 text-center text-sm font-medium text-gray-500 transition-colors last:border-r-0 hover:bg-gray-50 hover:text-gray-900 sm:px-4 dark:border-dark-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :class="isModuleActive(item.name) ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300' : ''"
          >
            <Icon :name="item.icon" size="sm" class="hidden flex-shrink-0 sm:block" />
            <span>{{ item.label }}</span>
          </RouterLink>
        </nav>

        <RouterView v-slot="{ Component }">
          <component :is="Component" :supplier="profile" />
        </RouterView>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { Icon } from '@/components/icons'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { supplierAPI, type Supplier } from '@/api/suppliers'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const route = useRoute()
const appStore = useAppStore()
const loading = ref(true)
const submitting = ref(false)
const profile = ref<Supplier | null>(null)
const supplierForm = reactive({ name: '', relay_url: '', application_note: '' })

const moduleNav = [
  { name: 'SupplierResourceSubmit', label: '提交中转资源', icon: 'plus' as const },
  { name: 'SupplierResourceRequests', label: '资源申请记录', icon: 'clipboard' as const },
  { name: 'SupplierBills', label: '收益账单', icon: 'chart' as const }
]

const profileStatusLabel = computed(() => ({
  pending: '入驻审核中',
  approved: '已入驻',
  rejected: '审核驳回',
  frozen: '已冻结'
}[profile.value?.status || 'pending']))

const profileStatusClass = computed(() => ({
  pending: 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300',
  approved: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300',
  rejected: 'bg-rose-50 text-rose-700 dark:bg-rose-900/20 dark:text-rose-300',
  frozen: 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
}[profile.value?.status || 'pending']))

function isModuleActive(name: string) {
  return route.name === name
}

async function loadProfile() {
  loading.value = true
  try {
    profile.value = await supplierAPI.me()
    Object.assign(supplierForm, {
      name: profile.value.name,
      relay_url: profile.value.relay_url,
      application_note: profile.value.application_note
    })
  } catch {
    profile.value = null
  } finally {
    loading.value = false
  }
}

async function submitSupplier() {
  submitting.value = true
  try {
    profile.value = await supplierAPI.apply(supplierForm)
    appStore.showSuccess('入驻资料已提交')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '提交入驻资料失败'))
  } finally {
    submitting.value = false
  }
}

onMounted(loadProfile)
</script>
