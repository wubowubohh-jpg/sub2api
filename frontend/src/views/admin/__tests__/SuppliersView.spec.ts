import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SuppliersView from '../SuppliersView.vue'

const {
  list,
  resourceRequests,
  settings,
  updateResourceRequest,
  withdrawals,
} = vi.hoisted(() => ({
  list: vi.fn(),
  resourceRequests: vi.fn(),
  settings: vi.fn(),
  updateResourceRequest: vi.fn(),
  withdrawals: vi.fn(),
}))

vi.mock('@/api/suppliers', () => ({
  adminSupplierAPI: {
    list,
    resourceRequests,
    settings,
    updateResourceRequest,
    withdrawals,
    updateSettings: vi.fn(),
    review: vi.fn(),
    freeze: vi.fn(),
    unfreeze: vi.fn(),
    reviewResourceRequest: vi.fn(),
    reviewWithdrawal: vi.fn(),
  },
}))

const resource = {
  id: 41,
  supplier_id: 7,
  group_name: 'A0007-main',
  group_name_suffix: 'main',
  relay_name: 'Main Relay',
  relay_url: 'https://old.example.com/v1',
  model: 'gpt-5.5',
  supported_models: ['gpt-5.5', 'gpt-5.6'],
  rate_multiplier: 0.04,
  admin_rate_adjustment: 0.01,
  effective_rate_multiplier: 0.05,
  status: 'approved',
  review_note: 'approved',
  group_id: 81,
  account_id: 91,
  monitor_id: 101,
  upstream_billing_probe_enabled: true,
  created_at: '2026-08-05T00:00:00Z',
}

function field(wrapper: ReturnType<typeof mount>, label: string) {
  const target = wrapper.findAll('label').find(item => item.find('.input-label').exists() && item.find('.input-label').text() === label)
  expect(target, `field ${label}`).toBeDefined()
  return target!
}

describe('SuppliersView resource editor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    settings.mockResolvedValue({ global_rate_adjustment: 0.01, settlement_delay_days: 7 })
    list.mockResolvedValue({ items: [] })
    resourceRequests.mockResolvedValue({ items: [{ ...resource }] })
    withdrawals.mockResolvedValue({ items: [] })
    updateResourceRequest.mockResolvedValue({
      ...resource,
      group_name: 'A0007-premium',
      group_name_suffix: 'premium',
      relay_name: 'Premium Relay',
      relay_url: 'https://new.example.com/api',
      rate_multiplier: 0.075,
      admin_rate_adjustment: 0.015,
      effective_rate_multiplier: 0.09,
      review_note: 'updated by admin',
    })
  })

  it('allows an administrator to edit and submit all resource fields', async () => {
    const wrapper = mount(SuppliersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          AccountTestModal: true,
          BaseDialog: {
            props: ['show', 'title'],
            template: '<div v-if="show" class="modal-content"><slot /><slot name="footer" /></div>',
          },
          Toggle: true,
        },
      },
    })
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('资源审核'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('编辑资源'))!.trigger('click')

    expect(wrapper.text()).toContain('替换 API Key')
    expect(wrapper.text()).toContain('支持模型')
    expect(wrapper.text()).toContain('默认监听模型')
    await field(wrapper, '大厅分组后缀').find('input').setValue('premium')
    await field(wrapper, '中转站名称').find('input').setValue('Premium Relay')
    await field(wrapper, 'API 基础地址').find('input').setValue('https://new.example.com/api')
    await field(wrapper, '替换 API Key').find('input').setValue('sk-replaced')
    await field(wrapper, '供应商基础倍率').find('input').setValue('0.075')
    await field(wrapper, '管理员增加倍率').find('input').setValue('0.015')
    await field(wrapper, '审核备注').find('textarea').setValue('updated by admin')

    await wrapper.findAll('button').find(button => button.text().includes('保存全部修改'))!.trigger('click')
    await flushPromises()

    expect(updateResourceRequest).toHaveBeenCalledWith(41, {
      group_name: 'premium',
      relay_name: 'Premium Relay',
      relay_url: 'https://new.example.com/api',
      api_key: 'sk-replaced',
      monitor_model: 'gpt-5.5',
      supported_models: ['gpt-5.5', 'gpt-5.6'],
      upstream_billing_probe_enabled: true,
      rate_multiplier: 0.075,
      admin_rate_adjustment: 0.015,
      review_note: 'updated by admin',
    })
    wrapper.unmount()
  })
})
