import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ChannelStatusView from '../ChannelStatusView.vue'

const { hall, listMonitors, monitorStatus, showError } = vi.hoisted(() => ({
  hall: vi.fn(),
  listMonitors: vi.fn(),
  monitorStatus: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('vue-i18n', async importOriginal => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const labels: Record<string, string> = {
          'channelStatus.title': '供应商大厅',
          'channelStatus.monitoring': '监测中',
          'channelStatus.refreshing': '刷新中',
          'channelStatus.refresh': '刷新',
          'channelStatus.public': '公开分组',
          'channelStatus.noData': '暂无数据',
          'channelStatus.noRecord': '暂无记录',
          'channelStatus.monitorDetail': '监控详情',
          'channelStatus.useGroup': '使用此分组',
          'monitorCommon.status.operational': '正常',
          'monitorCommon.status.degraded': '降级',
          'monitorCommon.status.failed': '失败',
          'monitorCommon.providers.openai': 'OpenAI',
        }
        if (key === 'channelStatus.requestCount') return `窗口内 ${params?.count} 次调用`
        if (key === 'channelStatus.updatedAt') return `更新于 ${params?.time}`
        return labels[key] || key
      },
    }),
  }
})

vi.mock('@/api/suppliers', () => ({
  supplierAPI: { hall },
}))

vi.mock('@/api/channelMonitor', () => ({
  list: listMonitors,
  status: monitorStatus,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError }),
}))

describe('ChannelStatusView supplier hall', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    hall.mockResolvedValue({
      groups: [{
        id: 7,
        name: 'A0001-codex-plus',
        description: '稳定的 Codex 中转分组',
        platform: 'openai',
        effective_rate: 0.05,
        status: 'active',
        is_exclusive: false,
        metrics: {
          request_count: 18,
          avg_latency_ms: 1800,
          avg_first_token_ms: 420,
          probe_latency_ms: 330,
          cache_hit_rate: 72.5,
          tps: 24.8,
          availability: 99.5,
          latest_probe_at: '2026-08-05T01:00:00Z',
          timeline: [],
        },
      }],
    })
    listMonitors.mockResolvedValue({
      items: [{
        id: 21,
        name: 'A0001-codex-plus monitor',
        provider: 'openai',
        group_name: 'A0001-codex-plus',
        primary_model: 'gpt-5.5',
        primary_status: 'operational',
        primary_latency_ms: 390,
        primary_ping_latency_ms: 42,
        availability_7d: 99.2,
        extra_models: [{ model: 'gpt-5.4', status: 'degraded', latency_ms: 680 }],
        timeline: [
          { status: 'operational', latency_ms: 390, ping_latency_ms: 42, checked_at: '2026-08-05T01:00:00Z' },
          { status: 'failed', latency_ms: null, ping_latency_ms: 50, checked_at: '2026-08-05T00:55:00Z' },
        ],
      }],
    })
  })

  it('shows hall pricing, monitor card details and record statuses in one list', async () => {
    const wrapper = mount(ChannelStatusView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: { template: '<i />' },
          RouterLink: { template: '<a><slot /></a>' },
          MonitorDetailDialog: { template: '<div />' },
          MonitorTimeline: {
            props: ['buckets'],
            template: '<div data-test="timeline">{{ buckets.map((item) => item.status).join(",") }}</div>',
          },
        },
      },
    })
    await flushPromises()

    const text = wrapper.text()
    expect(hall).toHaveBeenCalledWith('6h')
    expect(listMonitors).toHaveBeenCalledOnce()
    expect(text).toContain('供应商大厅')
    expect(text).toContain('A0001-codex-plus')
    expect(text).toContain('0.05x')
    expect(text).toContain('gpt-5.5')
    expect(text).toContain('gpt-5.4')
    expect(text).toContain('390ms')
    expect(text).toContain('42ms')
    expect(text).toContain('99.5%')
    expect(text).toContain('窗口内 18 次调用')
    expect(wrapper.get('[data-test="timeline"]').text()).toBe('operational,failed')
    expect(text).toContain('使用此分组')

    wrapper.unmount()
  })
})
