import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SupplierResourceSubmitView from '../SupplierResourceSubmitView.vue'

const { createResourceRequest, push, showError, showSuccess } = vi.hoisted(() => ({
  createResourceRequest: vi.fn(),
  push: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/suppliers', () => ({
  supplierAPI: { createResourceRequest },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push }),
}))

const supportedModels = [
  'gpt-5.4',
  'gpt-5.4-mini',
  'gpt-5.5',
  'gpt-5.6',
  'gpt-5.6-sol',
  'gpt-5.6-terra',
  'gpt-5.6-luna',
  'gpt-5.3-codex-spark',
]

describe('SupplierResourceSubmitView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    createResourceRequest.mockResolvedValue({ id: 1 })
    push.mockResolvedValue(undefined)
  })

  it('selects and submits all built-in models by default', async () => {
    const wrapper = mount(SupplierResourceSubmitView, {
      props: {
        supplier: {
          id: 7,
          user_id: 9,
          name: 'Supplier',
          relay_url: 'https://supplier.example',
          application_note: '',
          status: 'approved',
          review_note: '',
          freeze_reason: '',
          pending_balance_cny: 0,
          available_balance_cny: 0,
          frozen_balance_cny: 0,
          group_name_prefix: 'A0007',
          created_at: '2026-08-04T00:00:00Z',
        },
      },
      global: {
        stubs: {
          Icon: true,
          LoadingSpinner: true,
          Select: true,
          Toggle: true,
        },
      },
    })

    const modelCheckboxes = wrapper.findAll('fieldset input[type="checkbox"]')
    expect(modelCheckboxes).toHaveLength(supportedModels.length)
    expect(modelCheckboxes.every(input => (input.element as HTMLInputElement).checked)).toBe(true)
    for (const model of supportedModels) {
      expect(wrapper.find('fieldset').text()).toContain(model)
    }

    await wrapper.find('input[placeholder="1"]').setValue('main')
    await wrapper.find('input[placeholder="例如：香港主线路"]').setValue('Main relay')
    await wrapper.find('input[placeholder="https://api.example.com"]').setValue('https://relay.example')
    await wrapper.find('input[placeholder="sk-..."]').setValue('sk-test')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(createResourceRequest).toHaveBeenCalledWith(expect.objectContaining({
      model: 'gpt-5.5',
      probe_model: 'gpt-5.5',
      supported_models: supportedModels,
    }))
  })
})
