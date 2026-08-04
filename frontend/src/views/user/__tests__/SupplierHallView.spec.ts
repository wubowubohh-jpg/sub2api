import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SupplierHallView from '../SupplierHallView.vue'

const { hall, showError } = vi.hoisted(() => ({
  hall: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/suppliers', () => ({
  supplierAPI: { hall },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError }),
}))

describe('SupplierHallView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hall.mockResolvedValue({
      groups: [{
        id: 1,
        name: 'A0001-codex plus',
        description: '',
        platform: 'openai',
        effective_rate: 0.05,
        base_rate: 0.04,
        admin_adjustment: 0.01,
        supplier_id: 1,
        supplier_name: 'hidden supplier',
        status: 'active',
        is_exclusive: false,
        metrics: { request_count: 0, timeline: [] },
      }],
    })
  })

  it('shows only the compact final rate', async () => {
    const wrapper = mount(SupplierHallView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('0.05')
    expect(wrapper.text()).not.toContain('0.0500')
    expect(wrapper.text()).not.toContain('基础 0.04')
    expect(wrapper.text()).not.toContain('调整 +0.01')
    expect(wrapper.text()).not.toContain('hidden supplier')
  })
})
