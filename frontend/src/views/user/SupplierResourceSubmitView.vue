<template>
  <section class="space-y-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">提交中转资源</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">填写上游连接、公开分组和模型范围。</p>
      </div>
      <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
        <span class="h-2 w-2 rounded-full bg-emerald-500" />
        上游倍率探测默认开启
      </div>
    </div>

    <form
      class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900"
      @submit.prevent="submitResource"
    >
      <div class="grid gap-5 p-5 sm:p-6 lg:grid-cols-2">
        <label class="block">
          <span class="input-label">大厅分组名称</span>
          <div class="flex rounded-xl border border-gray-200 bg-white transition focus-within:border-primary-500 focus-within:ring-2 focus-within:ring-primary-500/30 dark:border-dark-600 dark:bg-dark-800">
            <span class="flex items-center border-r border-gray-200 bg-gray-50 px-3 text-sm font-medium text-gray-600 dark:border-dark-600 dark:bg-dark-900 dark:text-dark-300">
              {{ groupPrefix }}-
            </span>
            <input
              v-model.trim="resourceForm.group_name_suffix"
              required
              maxlength="40"
              placeholder="1"
              class="min-w-0 flex-1 bg-transparent px-3 py-2.5 text-sm text-gray-900 outline-none placeholder:text-gray-400 dark:text-gray-100"
            />
          </div>
          <span class="input-hint">大厅最终显示：{{ fullGroupName }}</span>
        </label>

        <label class="block">
          <span class="input-label">供应商基础倍率</span>
          <input
            v-model.number="resourceForm.rate_multiplier"
            type="number"
            required
            min="0"
            step="0.001"
            placeholder="0.04"
            class="input w-full"
          />
          <span class="input-hint">例如填写 0.04；实际倍率为此数值加管理员全局或分组调整，上游探测结果仅用于参考。</span>
        </label>

        <label class="block">
          <span class="input-label">中转站名称</span>
          <input
            v-model.trim="resourceForm.relay_name"
            required
            maxlength="100"
            placeholder="例如：香港主线路"
            class="input w-full"
          />
          <span class="input-hint">仅用于识别该上游资源。</span>
        </label>

        <label class="block">
          <span class="input-label">API 基础地址</span>
          <input
            v-model.trim="resourceForm.relay_url"
            type="url"
            required
            placeholder="https://api.example.com"
            class="input w-full"
          />
          <span class="input-hint">填写 HTTPS 根地址，无需附加模型名称。</span>
        </label>

        <label class="block lg:col-span-2">
          <span class="input-label">API Key</span>
          <div class="relative">
            <input
              v-model="resourceForm.api_key"
              :type="showAPIKey ? 'text' : 'password'"
              required
              autocomplete="new-password"
              placeholder="sk-..."
              class="input w-full pr-11"
            />
            <button
              type="button"
              class="absolute right-1.5 top-1/2 -translate-y-1/2 rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200"
              :title="showAPIKey ? '隐藏 API Key' : '显示 API Key'"
              @click="showAPIKey = !showAPIKey"
            >
              <Icon :name="showAPIKey ? 'eyeOff' : 'eye'" size="sm" />
            </button>
          </div>
          <span class="input-hint">凭据将加密保存，可在申请记录中更新。</span>
        </label>
      </div>

      <div class="border-t border-gray-100 px-5 py-6 sm:px-6 dark:border-dark-700">
        <div class="flex flex-col gap-5 lg:grid lg:grid-cols-[1.45fr_0.75fr]">
          <fieldset>
            <legend class="text-sm font-medium text-gray-700 dark:text-gray-300">支持模型</legend>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">至少选择一个模型。</p>
            <div class="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
              <label
                v-for="model in modelCatalog"
                :key="model"
                class="flex min-h-10 cursor-pointer items-center gap-2.5 rounded-lg border px-3 py-2 text-sm transition-colors"
                :class="resourceForm.supported_models.includes(model)
                  ? 'border-primary-300 bg-primary-50 text-primary-800 dark:border-primary-700 dark:bg-primary-900/20 dark:text-primary-200'
                  : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-dark-600 dark:text-dark-300 dark:hover:border-dark-500'"
              >
                <input
                  :checked="resourceForm.supported_models.includes(model)"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  @change="toggleModel(model)"
                />
                <span class="min-w-0 truncate">{{ model }}</span>
              </label>
            </div>
            <div class="mt-3 flex flex-col gap-2 sm:flex-row">
              <input
                v-model.trim="customModel"
                class="input min-w-0 flex-1"
                placeholder="添加其他模型 ID"
                @keydown.enter.prevent="addCustomModel"
              />
              <button type="button" class="btn btn-secondary flex-shrink-0" @click="addCustomModel">
                <Icon name="plus" size="sm" />
                添加模型
              </button>
            </div>
          </fieldset>

          <div class="space-y-5 lg:border-l lg:border-gray-100 lg:pl-6 dark:lg:border-dark-700">
            <label class="block">
              <span class="input-label">默认监听模型</span>
              <Select
                v-model="resourceForm.monitor_model"
                :options="monitorModelOptions"
                :disabled="resourceForm.supported_models.length === 0"
                placeholder="选择监听模型"
              />
              <span class="input-hint">监听模型必须包含在支持模型中。</span>
            </label>

            <div class="flex items-start justify-between gap-4 border-t border-gray-100 pt-5 dark:border-dark-700">
              <div>
                <div class="text-sm font-medium text-gray-700 dark:text-gray-300">探测上游倍率</div>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">审核通过后随账号自动启用。</p>
              </div>
              <Toggle v-model="resourceForm.upstream_billing_probe_enabled" />
            </div>
          </div>
        </div>
      </div>

      <div class="flex flex-col-reverse gap-3 border-t border-gray-100 bg-gray-50 px-5 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6 dark:border-dark-700 dark:bg-dark-900/60">
        <div class="text-xs text-gray-500 dark:text-dark-400">
          {{ resourceForm.supported_models.length }} 个支持模型 · 监听 {{ resourceForm.monitor_model || '未选择' }}
        </div>
        <button type="submit" class="btn btn-primary" :disabled="submitting || !canSubmit">
          <LoadingSpinner v-if="submitting" size="sm" color="white" />
          <Icon v-else name="upload" size="sm" />
          {{ submitting ? '提交中' : '提交资源审核' }}
        </button>
      </div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Icon } from '@/components/icons'
import { supplierAPI, type Supplier } from '@/api/suppliers'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ supplier: Supplier }>()
const router = useRouter()
const appStore = useAppStore()
const submitting = ref(false)
const showAPIKey = ref(false)
const customModel = ref('')
const builtInModels = [
  'gpt-5.4',
  'gpt-5.4-mini',
  'gpt-5.5',
  'gpt-5.6',
  'gpt-5.6-sol',
  'gpt-5.6-terra',
  'gpt-5.6-luna',
  'gpt-5.3-codex-spark'
]
const extraModels = ref<string[]>([])
const resourceForm = reactive({
  group_name_suffix: '',
  relay_name: '',
  relay_url: '',
  api_key: '',
  rate_multiplier: 0.04,
  monitor_model: 'gpt-5.5',
  supported_models: [...builtInModels] as string[],
  upstream_billing_probe_enabled: true
})

const groupPrefix = computed(() => {
  const value = props.supplier.group_name_prefix || props.supplier.supplier_code
  return String(value || `A${String(props.supplier.id).padStart(4, '0')}`).replace(/-+$/, '')
})
const fullGroupName = computed(() => `${groupPrefix.value}-${resourceForm.group_name_suffix || '自定义后缀'}`)
const modelCatalog = computed(() => [...new Set([...builtInModels, ...extraModels.value])])
const monitorModelOptions = computed(() => resourceForm.supported_models.map(model => ({ value: model, label: model })))
const canSubmit = computed(() => Boolean(
  resourceForm.group_name_suffix &&
  resourceForm.relay_name &&
  resourceForm.relay_url &&
  resourceForm.api_key &&
  Number.isFinite(resourceForm.rate_multiplier) &&
  resourceForm.rate_multiplier >= 0 &&
  resourceForm.supported_models.length &&
  resourceForm.monitor_model
))

watch(() => [...resourceForm.supported_models], models => {
  if (!models.includes(resourceForm.monitor_model)) {
    resourceForm.monitor_model = models[0] || ''
  }
})

function toggleModel(model: string) {
  const index = resourceForm.supported_models.indexOf(model)
  if (index >= 0) {
    resourceForm.supported_models.splice(index, 1)
  } else {
    resourceForm.supported_models.push(model)
  }
}

function addCustomModel() {
  const model = customModel.value.trim()
  if (!model) return
  if (!extraModels.value.includes(model) && !builtInModels.includes(model)) {
    extraModels.value.push(model)
  }
  if (!resourceForm.supported_models.includes(model)) {
    resourceForm.supported_models.push(model)
  }
  customModel.value = ''
}

async function submitResource() {
  if (!canSubmit.value) return
  submitting.value = true
  try {
    await supplierAPI.createResourceRequest({
      group_name: resourceForm.group_name_suffix,
      relay_name: resourceForm.relay_name,
      relay_url: resourceForm.relay_url,
      api_key: resourceForm.api_key,
      model: resourceForm.monitor_model,
      probe_model: resourceForm.monitor_model,
      supported_models: [...resourceForm.supported_models],
      upstream_billing_probe_enabled: resourceForm.upstream_billing_probe_enabled,
      rate_multiplier: resourceForm.rate_multiplier
    })
    appStore.showSuccess('中转资源已提交审核')
    await router.push({ name: 'SupplierResourceRequests' })
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '提交资源失败'))
  } finally {
    submitting.value = false
  }
}
</script>
