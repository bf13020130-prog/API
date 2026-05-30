import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'

import DashboardView from '../DashboardView.vue'

const { getDashboardStats, getDashboardTrend, getDashboardModels, getByDateRange, getMyPlatformQuotas, refreshUser, fetchAnnouncements } = vi.hoisted(() => ({
  getDashboardStats: vi.fn(),
  getDashboardTrend: vi.fn(),
  getDashboardModels: vi.fn(),
  getByDateRange: vi.fn(),
  getMyPlatformQuotas: vi.fn(),
  refreshUser: vi.fn(),
  fetchAnnouncements: vi.fn(),
}))

const userRef = ref({
  id: 1,
  username: '小黄人',
  email: 'demo@example.com',
  role: 'user',
  balance: 12.5,
  concurrency: 1,
  status: 'active',
  allowed_groups: null,
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
})

const announcementsRef = ref([
  {
    id: 1,
    title: '系统公告',
    content: 'claude 限时特价，codex 倍率优化',
    notify_mode: 'silent',
    eligible: true,
  },
])

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats,
    getDashboardTrend,
    getDashboardModels,
    getByDateRange,
  },
}))

vi.mock('@/api/user', () => ({
  getMyPlatformQuotas,
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    user: userRef.value,
    isSimpleMode: false,
    refreshUser,
  }),
  useAppStore: () => ({
    siteName: 'Sub2API',
  }),
  useAnnouncementStore: () => ({
    announcements: announcementsRef.value,
    loading: false,
    unreadCount: 1,
    fetchAnnouncements,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'dashboard.title': '仪表板',
    'dashboard.welcomeBack': '欢迎回来，{name}',
    'dashboard.balance': '余额',
    'dashboard.redeemTopUp': '兑换充值',
    'dashboard.used': '已使用',
    'dashboard.todayCost': '今日消费',
    'dashboard.todayRequests': '今日请求',
    'dashboard.todayTokens': '今日 Token',
    'dashboard.avgResponse': '平均响应',
    'dashboard.accountCredits': '账户额度',
    'dashboard.accountCreditsHint': '余额、消耗与当前请求态势',
    'dashboard.balanceBase': '余额基数',
    'dashboard.systemAnnouncements': '系统公告',
    'dashboard.systemAnnouncementsHint': '最新站内消息和运营提示',
    'dashboard.noAnnouncements': '暂无公告',
    'dashboard.noAnnouncementsHint': '系统运行正常，新的消息会显示在这里。',
    'dashboard.modelSquare': '模型广场',
    'dashboard.modelSquareHint': '查看可用渠道、模型与接入方式',
    'dashboard.inviteRebate': '邀请返佣',
    'dashboard.inviteRebateHint': '分享邀请链接，好友使用后获得返利',
    'dashboard.redeemRecharge': '兑换充值',
    'dashboard.redeemRechargeHint': '输入兑换码，额度即时到账',
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

describe('user DashboardView', () => {
  beforeEach(() => {
    getDashboardStats.mockReset()
    getDashboardTrend.mockReset()
    getDashboardModels.mockReset()
    getByDateRange.mockReset()
    getMyPlatformQuotas.mockReset()
    refreshUser.mockReset()
    fetchAnnouncements.mockReset()

    refreshUser.mockResolvedValue(userRef.value)
    getDashboardStats.mockResolvedValue({
      total_api_keys: 2,
      active_api_keys: 1,
      total_requests: 100,
      total_input_tokens: 1000,
      total_output_tokens: 500,
      total_cache_creation_tokens: 0,
      total_cache_read_tokens: 0,
      total_tokens: 1500,
      total_cost: 1.2,
      total_actual_cost: 0.8,
      today_requests: 7,
      today_input_tokens: 100,
      today_output_tokens: 40,
      today_cache_creation_tokens: 0,
      today_cache_read_tokens: 0,
      today_tokens: 140,
      today_cost: 0.1,
      today_actual_cost: 0.08,
      average_duration_ms: 320,
      rpm: 3,
      tpm: 120,
      by_platform: [],
    })
    getDashboardTrend.mockResolvedValue({ trend: [], start_date: '', end_date: '', granularity: 'day' })
    getDashboardModels.mockResolvedValue({ models: [], start_date: '', end_date: '' })
    getByDateRange.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getMyPlatformQuotas.mockResolvedValue({ platform_quotas: [] })
  })

  it('renders NYCATAI-style account, announcement, and shortcut sections', async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          UserDashboardStats: true,
          UserDashboardCharts: true,
          UserDashboardRecentUsage: true,
          UserDashboardQuickActions: true,
          RouterLink: { props: ['to'], template: '<a><slot /></a>' },
        },
      },
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('欢迎回来，小黄人')
    expect(text).toContain('账户额度')
    expect(text).toContain('$12.50')
    expect(text).toContain('系统公告')
    expect(text).toContain('claude 限时特价')
    expect(text).toContain('模型广场')
    expect(text).toContain('邀请返佣')
    expect(text).toContain('兑换充值')
    expect(fetchAnnouncements).toHaveBeenCalled()
  })
})
