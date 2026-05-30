import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AdapterProvidersView from '../AdapterProvidersView.vue'

const {
  getAdapterProviderDiagnostics,
  listAdapterProviders,
  listRoutePolicies,
  listAdapterRequests,
  countAdapterRequests,
  getAdapterUsageSummary,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getAdapterProviderDiagnostics: vi.fn(),
  listAdapterProviders: vi.fn(),
  listRoutePolicies: vi.fn(),
  listAdapterRequests: vi.fn(),
  countAdapterRequests: vi.fn(),
  getAdapterUsageSummary: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    system: {
      getAdapterProviderDiagnostics,
      listAdapterProviders,
      listRoutePolicies,
      listAdapterRequests,
      countAdapterRequests,
      getAdapterUsageSummary,
      createAdapterProvider: vi.fn(),
      updateAdapterProvider: vi.fn(),
      deleteAdapterProvider: vi.fn(),
      createRoutePolicy: vi.fn(),
      updateRoutePolicy: vi.fn(),
      deleteRoutePolicy: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.adapterProviders.title': '适配器供应商',
    'admin.adapterProviders.description': '管理 DB 中的长尾适配器供应商',
    'admin.adapterProviders.operator.title': '适配器运营驾驶舱',
    'admin.adapterProviders.operator.subtitle': '供应商、策略、流式和 WebSocket 运行态',
    'admin.adapterProviders.operator.providerHealth': '供应商健康',
    'admin.adapterProviders.operator.activePolicies': '启用策略',
    'admin.adapterProviders.operator.recentFailures': '近期失败',
    'admin.adapterProviders.operator.streamFinalized': '流式结算',
    'admin.adapterProviders.operator.websocketTraffic': 'WebSocket',
    'admin.adapterProviders.operator.costToday': '累计成本',
    'admin.adapterProviders.operator.noIssues': '运行正常',
    'admin.adapterProviders.operator.needsAttention': '需要关注',
    'admin.adapterProviders.operator.readyRoutes': '可路由供应商',
    'admin.adapterProviders.observeOnly': '观察模式',
    'admin.adapterProviders.enforced': '强制模式',
    'admin.adapterProviders.enforcementEnabled': '已启用强制路由',
    'admin.adapterProviders.enforcementDisabled': '强制路由关闭',
    'admin.adapterProviders.createProvider': '新建供应商',
    'admin.adapterProviders.usage.totalRequests': '适配器请求',
    'admin.adapterProviders.usage.successRate': '成功率',
    'admin.adapterProviders.usage.totalCost': '适配器费用',
    'admin.adapterProviders.usage.billableUnits': '计费单位',
    'admin.adapterProviders.totalConfigured': '已配置',
    'admin.adapterProviders.activeConfigured': '已激活',
    'admin.adapterProviders.disabledConfigured': '已停用',
    'admin.adapterProviders.invalidConfigured': '配置异常',
    'admin.adapterProviders.activeSlugs': '路由激活 Slug',
    'admin.adapterProviders.activeSlugsHint': '只有有效且 active 的长尾供应商会进入 CapabilityRouter。',
    'admin.adapterProviders.enabled': '启用',
    'admin.adapterProviders.disabled': '停用',
    'admin.adapterProviders.invalid': '异常',
    'admin.adapterProviders.reasonEnabled': '已进入适配器路由候选',
    'admin.adapterProviders.reasonDisabled': '供应商已停用',
    'admin.adapterProviders.requests.window24h': '近 24 小时',
    'admin.adapterProviders.requests.window7d': '近 7 天',
    'admin.adapterProviders.requests.window30d': '近 30 天',
    'admin.adapterProviders.requests.windowAll': '全部时间',
    'admin.adapterProviders.requests.loadedCount': '已加载 {count} 条',
    'admin.adapterProviders.requests.loadedTotalCount': '已加载 {count} / 共 {total} 条',
    'admin.adapterProviders.requests.loadMore': '加载更多',
    'admin.adapterProviders.requests.pageStatus': '第 {page} / {total} 页',
    'admin.adapterProviders.requests.pageInputLabel': '页码',
    'admin.adapterProviders.requests.previousPage': '上一页',
    'admin.adapterProviders.requests.nextPage': '下一页',
    'admin.adapterProviders.requests.goToPage': '跳转',
    'admin.adapterProviders.requests.markers.streamFinalized': '流式结算',
    'admin.adapterProviders.requests.markers.websocketFinalized': 'WS结算',
    'admin.adapterProviders.requests.markers.websocketTunnel': 'WS通道',
    'common.refresh': '刷新',
    'common.loading': '加载中',
    'common.actions': '操作',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        const value = messages[key] ?? key
        return params ? value.replace(/\{(\w+)\}/g, (_, name) => params[name] ?? '') : value
      },
    }),
  }
})

describe('admin AdapterProvidersView', () => {
  function mountView() {
    return mount(AdapterProvidersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: {
            props: ['columns', 'data'],
            template: `
              <div class="data-table">
                <div v-for="row in data" :key="row.id || row.slug || row.name" class="data-row">
                  <div v-for="column in columns" :key="column.key" class="data-cell">
                    <slot :name="'cell-' + column.key" :row="row" :value="row[column.key]">
                      <span v-if="row[column.key]">{{ row[column.key] }}</span>
                    </slot>
                  </div>
                </div>
                <slot name="empty" />
              </div>
            `,
          },
          EmptyState: true,
          BaseDialog: true,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })
  }

  beforeEach(() => {
    getAdapterProviderDiagnostics.mockReset()
    listAdapterProviders.mockReset()
    listRoutePolicies.mockReset()
    listAdapterRequests.mockReset()
    countAdapterRequests.mockReset()
    getAdapterUsageSummary.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getAdapterProviderDiagnostics.mockResolvedValue({
      observe_only: false,
      enforcement_enabled: true,
      active_slugs: ['midjourney', 'runway'],
      providers: [
        {
          name: 'Midjourney',
          slug: 'midjourney',
          status: 'active',
          adapter_type: 'new-api',
          base_url: 'https://adapter.example.com',
          capabilities: ['image_generation'],
          priority: 10,
          timeout_ms: 30000,
          valid: true,
          enabled: true,
          reason: 'enabled',
        },
        {
          name: 'Broken',
          slug: 'broken',
          status: 'active',
          adapter_type: 'new-api',
          base_url: '',
          capabilities: ['chat'],
          priority: 99,
          timeout_ms: 30000,
          valid: false,
          enabled: false,
          reason: 'valid http base url is required',
        },
      ],
    })
    listAdapterProviders.mockResolvedValue([])
    listRoutePolicies.mockResolvedValue([
      { id: 1, name: 'MJ', status: 'active', target: 'new_api_adapter', priority: 10, created_at: '', updated_at: '' },
      { id: 2, name: 'Dormant', status: 'disabled', target: 'new_api_adapter', priority: 20, created_at: '', updated_at: '' },
    ])
    listAdapterRequests.mockResolvedValue([
      {
        id: 1,
        request_id: 'req-ok',
        user_id: 1,
        api_key_id: 2,
        adapter_provider_id: 1,
        provider: 'midjourney',
        capability: 'image_generation',
        route_target: 'new_api_adapter',
        method: 'POST',
        path: '/v1/images/generations',
        status_code: 200,
        duration_ms: 120,
        metadata: { stream_usage_finalized: true, usage_source: 'sse_final_chunk' },
        created_at: '2026-05-30T00:00:00Z',
      },
      {
        id: 2,
        request_id: 'req-ws',
        user_id: 1,
        api_key_id: 2,
        adapter_provider_id: 1,
        provider: 'runway',
        capability: 'chat',
        route_target: 'new_api_adapter',
        method: 'GET',
        path: '/v1/responses',
        status_code: 101,
        duration_ms: 80,
        metadata: {
          websocket: true,
          websocket_usage_finalized: true,
          transport: 'websocket',
          usage_source: 'websocket_event',
        },
        created_at: '2026-05-30T00:01:00Z',
      },
      {
        id: 3,
        request_id: 'req-failed',
        user_id: 1,
        api_key_id: 2,
        adapter_provider_id: 1,
        provider: 'broken',
        capability: 'chat',
        route_target: 'new_api_adapter',
        method: 'POST',
        path: '/v1/chat/completions',
        status_code: 502,
        error_message: 'adapter failed',
        metadata: {},
        created_at: '2026-05-30T00:02:00Z',
      },
    ])
    getAdapterUsageSummary.mockResolvedValue({
      total_requests: 3,
      success_requests: 2,
      failed_requests: 1,
      input_units: 100,
      output_units: 30,
      billable_units: 130,
      cost_usd: 0.42,
      providers: [],
    })
    countAdapterRequests.mockResolvedValue({ total: 3 })
  })

  it('renders operator cockpit metrics for adapter operations', async () => {
    const wrapper = mountView()

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('适配器运营驾驶舱')
    expect(text).toContain('供应商健康')
    expect(text).toContain('1 / 2')
    expect(text).toContain('启用策略')
    expect(text).toContain('1')
    expect(text).toContain('近期失败')
    expect(text).toContain('1')
    expect(text).toContain('流式结算')
    expect(text).toContain('1')
    expect(text).toContain('WebSocket')
    expect(text).toContain('1')
    expect(text).toContain('$0.420000')
  })

  it('shows finalized stream and WebSocket usage markers in request audit rows', async () => {
    const wrapper = mountView()

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('流式结算')
    expect(text).toContain('sse_final_chunk')
    expect(text).toContain('WS结算')
    expect(text).toContain('websocket_event')
  })

  it('focuses the request audit table from the recent failure metric', async () => {
    const wrapper = mountView()

    await flushPromises()

    expect(wrapper.text()).toContain('req-ok')
    expect(wrapper.text()).toContain('req-ws')
    expect(wrapper.text()).toContain('req-failed')

    const failureButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('近期失败'))
    expect(failureButton).toBeTruthy()

    await failureButton!.trigger('click')
    await flushPromises()

    expect(listAdapterRequests).toHaveBeenLastCalledWith(expect.objectContaining({ focus: 'failed' }))

    const text = wrapper.text()
    expect(text).not.toContain('req-ok')
    expect(text).not.toContain('req-ws')
    expect(text).toContain('req-failed')
  })

  it('sends the selected audit time window to the adapter request API', async () => {
    const wrapper = mountView()

    await flushPromises()

    expect(listAdapterRequests).toHaveBeenLastCalledWith(expect.objectContaining({
      created_from: expect.any(String),
      created_to: expect.any(String),
      limit: 100,
    }))

    const windowSelect = wrapper
      .findAll('select')
      .find((select) => select.text().includes('全部时间'))
    expect(windowSelect).toBeTruthy()

    await windowSelect!.setValue('all')
    await wrapper.find('button.btn-secondary').trigger('click')
    await flushPromises()

    expect(listAdapterRequests).toHaveBeenLastCalledWith(expect.objectContaining({
      created_from: undefined,
      created_to: undefined,
      limit: 100,
    }))
  })

  it('loads the next adapter request page and appends it to the audit table', async () => {
    listAdapterRequests.mockImplementation((filters) => {
      if (filters.offset === 100) {
        return Promise.resolve([
          {
            id: 4,
            request_id: 'req-next-page',
            user_id: 1,
            api_key_id: 2,
            adapter_provider_id: 1,
            provider: 'midjourney',
            capability: 'image_generation',
            route_target: 'new_api_adapter',
            method: 'POST',
            path: '/v1/images/generations',
            status_code: 200,
            metadata: {},
            created_at: '2026-05-30T00:03:00Z',
          },
        ])
      }
      return Promise.resolve(Array.from({ length: 100 }, (_, index) => ({
        id: index + 1,
        request_id: `req-page-${index + 1}`,
        user_id: 1,
        api_key_id: 2,
        adapter_provider_id: 1,
        provider: 'midjourney',
        capability: 'image_generation',
        route_target: 'new_api_adapter',
        method: 'POST',
        path: '/v1/images/generations',
        status_code: 200,
        metadata: {},
        created_at: '2026-05-30T00:00:00Z',
      })))
    })
    countAdapterRequests.mockResolvedValue({ total: 101 })

    const wrapper = mountView()

    await flushPromises()

    expect(wrapper.text()).toContain('req-page-1')
    expect(wrapper.text()).not.toContain('req-next-page')
    expect(wrapper.text()).toContain('已加载 100 / 共 101 条')
    expect(listAdapterRequests).toHaveBeenLastCalledWith(expect.objectContaining({
      offset: 0,
      limit: 100,
    }))
    expect(countAdapterRequests).toHaveBeenLastCalledWith(expect.objectContaining({
      offset: undefined,
      limit: undefined,
    }))

    const loadMoreButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('加载更多'))
    expect(loadMoreButton).toBeTruthy()

    await loadMoreButton!.trigger('click')
    await flushPromises()

    expect(listAdapterRequests).toHaveBeenLastCalledWith(expect.objectContaining({
      offset: 100,
      limit: 100,
    }))
    expect(wrapper.text()).toContain('req-page-1')
    expect(wrapper.text()).toContain('req-next-page')
    expect(wrapper.text()).toContain('已加载 101 / 共 101 条')
  })

  it('jumps directly to an adapter request page using the filtered total', async () => {
    listAdapterRequests.mockImplementation((filters) => {
      if (filters.offset === 200) {
        return Promise.resolve([
          {
            id: 201,
            request_id: 'req-page-3',
            user_id: 1,
            api_key_id: 2,
            adapter_provider_id: 1,
            provider: 'midjourney',
            capability: 'image_generation',
            route_target: 'new_api_adapter',
            method: 'POST',
            path: '/v1/images/generations',
            status_code: 200,
            metadata: {},
            created_at: '2026-05-30T00:10:00Z',
          },
        ])
      }
      return Promise.resolve(Array.from({ length: 100 }, (_, index) => ({
        id: index + 1,
        request_id: `req-page-1-${index + 1}`,
        user_id: 1,
        api_key_id: 2,
        adapter_provider_id: 1,
        provider: 'midjourney',
        capability: 'image_generation',
        route_target: 'new_api_adapter',
        method: 'POST',
        path: '/v1/images/generations',
        status_code: 200,
        metadata: {},
        created_at: '2026-05-30T00:00:00Z',
      })))
    })
    countAdapterRequests.mockResolvedValue({ total: 250 })

    const wrapper = mountView()

    await flushPromises()

    expect(wrapper.text()).toContain('第 1 / 3 页')
    expect(wrapper.text()).toContain('req-page-1-1')
    expect(wrapper.text()).not.toContain('req-page-3')

    const pageInput = wrapper.find('input[aria-label="页码"]')
    expect(pageInput.exists()).toBe(true)
    await pageInput.setValue('3')
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('跳转'))!
      .trigger('click')
    await flushPromises()

    expect(listAdapterRequests).toHaveBeenLastCalledWith(expect.objectContaining({
      offset: 200,
      limit: 100,
    }))
    expect(countAdapterRequests).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('第 3 / 3 页')
    expect(wrapper.text()).not.toContain('req-page-1-1')
    expect(wrapper.text()).toContain('req-page-3')
  })
})
