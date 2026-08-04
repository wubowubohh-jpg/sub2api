import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import SupplierResourceRequestsView from '../SupplierResourceRequestsView.vue'

const { resourceRequests, updateResourceRate, updateResourceModels, showError, showSuccess } = vi.hoisted(() => ({
  resourceRequests: vi.fn(),
  updateResourceRate: vi.fn(),
  updateResourceModels: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/suppliers', () => ({
  supplierAPI: {
    resourceRequests,
    updateResourceRate,
    updateResourceModels,
    updateResourceProbe: vi.fn(),
    updateResourceRequestAPIKey: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

const request = {
  id: 11,
  group_name: 'A0001-main',
  relay_name: 'Main relay',
  relay_url: 'https://relay.example',
  model: 'gpt-5.5',
  supported_models: ['gpt-5.5'],
  rate_multiplier: 0.04,
  rate_source: 'configured' as const,
  applied_rate_multiplier: 0.04,
  admin_rate_adjustment: 0.01,
  effective_rate_multiplier: 0.05,
  status: 'approved' as const,
  review_note: '',
  upstream_billing_probe_enabled: true,
  upstream_rate: 0.06,
  created_at: '2026-08-04T00:00:00Z',
}

describe('SupplierResourceRequestsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    resourceRequests.mockResolvedValue({ items: [{ ...request }] })
    updateResourceRate.mockResolvedValue({
      ...request,
      rate_multiplier: 0.08,
      applied_rate_multiplier: 0.08,
      effective_rate_multiplier: 0.09,
    })
    updateResourceModels.mockResolvedValue({
      ...request,
      model: 'gpt-custom',
      supported_models: ['gpt-5.5', 'gpt-custom'],
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('shows the submitted and effective rates and lets the supplier update the submitted rate', async () => {
    const wrapper = mount(SupplierResourceRequestsView, {
      attachTo: document.body,
      props: { supplier: {} },
      global: {
        stubs: {
          Icon: true,
          LoadingSpinner: true,
          RouterLink: true,
          Toggle: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('供应商提交倍率 0.0400')
    expect(wrapper.text()).toContain('当前有效倍率 0.0500')
    expect(wrapper.text()).toContain('设置倍率 0.0400 + 管理员增加 0.0100')

    const editButton = wrapper.findAll('button').find(button => button.text().includes('修改倍率'))
    expect(editButton).toBeDefined()
    await editButton!.trigger('click')
    await nextTick()

    const input = document.querySelector<HTMLInputElement>('.modal-content input[type="number"]')
    expect(input).not.toBeNull()
    input!.value = '0.08'
    input!.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    const saveButton = Array.from(document.querySelectorAll<HTMLButtonElement>('.modal-content button'))
      .find(button => button.textContent?.includes('保存倍率'))
    expect(saveButton).toBeDefined()
    saveButton!.click()
    await flushPromises()

    expect(updateResourceRate).toHaveBeenCalledWith(11, 0.08)
    expect(showSuccess).toHaveBeenCalledWith('供应商倍率已更新并实时生效')
    wrapper.unmount()
  })

  it('updates supported and monitor models without creating another review', async () => {
    const wrapper = mount(SupplierResourceRequestsView, {
      attachTo: document.body,
      props: { supplier: {} },
      global: {
        stubs: {
          Icon: true,
          LoadingSpinner: true,
          RouterLink: true,
          Toggle: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('[data-edit-models]').trigger('click')
    await nextTick()
    const customModel = document.querySelector<HTMLInputElement>('.modal-content [data-custom-model]')
    expect(customModel).not.toBeNull()
    customModel!.value = 'gpt-custom'
    customModel!.dispatchEvent(new Event('input', { bubbles: true }))
    customModel!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await nextTick()

    const monitorModel = document.querySelector<HTMLSelectElement>('.modal-content [data-monitor-model]')
    expect(monitorModel).not.toBeNull()
    monitorModel!.value = 'gpt-custom'
    monitorModel!.dispatchEvent(new Event('change', { bubbles: true }))
    await nextTick()
    const saveButton = document.querySelector<HTMLButtonElement>('.modal-content [data-save-models]')
    expect(saveButton?.disabled).toBe(false)
    saveButton!.click()
    await flushPromises()

    expect(updateResourceModels).toHaveBeenCalledWith(11, {
      monitor_model: 'gpt-custom',
      supported_models: ['gpt-5.5', 'gpt-custom'],
    })
    expect(showSuccess).toHaveBeenCalledWith('模型配置已更新并实时生效')
    expect(resourceRequests).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
})
