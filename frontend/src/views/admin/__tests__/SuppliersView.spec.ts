import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SuppliersView from '../SuppliersView.vue'

const {
  bills,
  list,
  resourceRequests,
  setGroupForcedOffline,
  settings,
  updateResourceRequest,
  withdrawals,
} = vi.hoisted(() => ({
  bills: vi.fn(),
  list: vi.fn(),
  resourceRequests: vi.fn(),
  setGroupForcedOffline: vi.fn(),
  settings: vi.fn(),
  updateResourceRequest: vi.fn(),
  withdrawals: vi.fn(),
}))

vi.mock('@/api/suppliers', () => ({
  adminSupplierAPI: {
    bills,
    list,
    resourceRequests,
    setGroupForcedOffline,
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
  resource_online: true,
  forced_offline: false,
  upstream_billing_probe_enabled: true,
  created_at: '2026-08-05T00:00:00Z',
}

const supplier = {
  id: 7,
  user_id: 17,
  name: '账单供应商',
  relay_url: 'https://relay.example.com',
  application_note: '',
  status: 'approved',
  review_note: '',
  freeze_reason: '',
  pending_balance_cny: 12.34,
  available_balance_cny: 5.67,
  frozen_balance_cny: 0,
  supplier_earning_cny: 18.01,
  admin_markup_earning_cny: 4.5,
  settlement_total_cny: 22.51,
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
    list.mockResolvedValue({ items: [{ ...supplier }] })
    resourceRequests.mockResolvedValue({ items: [{ ...resource }] })
    setGroupForcedOffline.mockResolvedValue({ id: 81, status: 'active', supplier_forced_offline: true })
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
    bills.mockResolvedValue({
      items: [{
        id: 99,
        supplier_id: supplier.id,
        group_id: 81,
        group_name: 'A0007-main',
        usage_log_id: 501,
        request_id: 'req-supplier-bill',
        user_id: 18,
        user_email: 'customer@example.com',
        username: 'customer',
        api_key_id: 31,
        account_id: 91,
        model: 'gpt-5.5',
        input_tokens: 100,
        output_tokens: 200,
        cache_read_tokens: 25,
        base_rate: 0.04,
        admin_adjustment: 0.01,
        effective_rate: 0.05,
        model_cost_usd: 1.25,
        recharge_ratio: 0.125,
        earning_usd: 0.05,
        amount_cny: 0.05,
        supplier_earning_usd: 0.05,
        supplier_earning_cny: 0.4,
        admin_markup_earning_usd: 0.0125,
        admin_markup_earning_cny: 0.1,
        settlement_total_usd: 0.0625,
        settlement_total_cny: 0.5,
        entry_type: 'earning',
        status: 'pending',
        created_at: '2026-08-05T00:00:00Z',
      }],
      total: 1,
      limit: 20,
      offset: 0,
      summary: {
        supplier_earning_cny: 18.01,
        admin_markup_earning_cny: 4.5,
        settlement_total_cny: 22.51,
      },
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

  it('shows the selected supplier ledger with the originating user details', async () => {
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

    await wrapper.findAll('button').find(button => button.text().includes('查看账单'))!.trigger('click')
    await flushPromises()

    expect(bills).toHaveBeenCalledWith(7, '', 20, 0)
    expect(wrapper.text()).toContain('customer@example.com')
    expect(wrapper.text()).toContain('req-supplier-bill')
    expect(wrapper.text()).toContain('账号 91')
    expect(wrapper.text()).toContain('Key 31')
    expect(wrapper.text()).toContain('平台加价收益合计')
    expect(wrapper.text()).toContain('管理员增加倍率累计')
    expect(wrapper.text()).toContain('22.51')
    wrapper.unmount()
  })

  it('lets an administrator force an approved resource offline', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(SuppliersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          AccountTestModal: true,
          BaseDialog: true,
          Toggle: true,
        },
      },
    })
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text().includes('资源审核'))!.trigger('click')
    await flushPromises()

    const moderationButton = wrapper.get('[data-resource-moderation]')
    expect(moderationButton.text()).toContain('强制下架')
    expect(wrapper.text()).toContain('已上架')
    await moderationButton.trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledOnce()
    expect(setGroupForcedOffline).toHaveBeenCalledWith(81, true)
    expect(resourceRequests).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    confirmSpy.mockRestore()
  })
})
