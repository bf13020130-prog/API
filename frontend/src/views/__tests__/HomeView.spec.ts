import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import HomeView from '../HomeView.vue'

const checkAuth = vi.fn()
const fetchPublicSettings = vi.fn()

const messages: Record<string, string> = {
  'home.viewDocs': 'Docs',
  'home.switchToLight': 'Light',
  'home.switchToDark': 'Dark',
  'home.dashboard': 'Dashboard',
  'home.login': 'Login',
  'home.getStarted': 'Get Started',
  'home.goToDashboard': 'Go to Console',
  'home.tags.accountPool': 'Account Pool',
  'home.tags.smartRouting': 'Smart Routing',
  'home.tags.requestTracing': 'Request Tracing',
  'home.features.sub2apiCore': 'Sub2API Core',
  'home.features.sub2apiCoreDesc': 'Keep the stronger OpenAI, Gemini, Claude, Codex and Antigravity account pool capabilities.',
  'home.features.fusionConsole': 'Fusion Console',
  'home.features.fusionConsoleDesc': 'A quieter operating surface for channels, accounts, groups, usage and billing.',
  'home.features.traceableOps': 'Traceable Ops',
  'home.features.traceableOpsDesc': 'Request correlation and error context stay available for debugging failed calls.',
  'home.providers.title': 'Provider coverage',
  'home.providers.description': 'Deep adapters first, OpenAI-compatible expansion second.',
  'home.providers.claude': 'Claude',
  'home.providers.gemini': 'Gemini',
  'home.providers.antigravity': 'Antigravity',
  'home.providers.supported': 'Supported',
  'home.providers.more': 'More',
  'home.providers.soon': 'Planned',
  'home.docs': 'Docs',
  'home.footer.allRightsReserved': 'All rights reserved.',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    isAuthenticated: false,
    isAdmin: false,
    user: null,
    checkAuth,
  }),
  useAppStore: () => ({
    cachedPublicSettings: null,
    siteName: 'Sub2API',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings,
  }),
}))

describe('HomeView fusion defaults', () => {
  beforeEach(() => {
    checkAuth.mockReset()
    fetchPublicSettings.mockReset()
    localStorage.clear()

    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockReturnValue({ matches: false }),
    })
  })

  it('rebrands the default upstream home page for the fusion build', () => {
    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          LocaleSwitcher: true,
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('API Fusion')
    expect(wrapper.text()).toContain('Sub2API Core')
    expect(wrapper.text()).toContain('Deep adapters first')
    expect(wrapper.text()).not.toContain('AI API Gateway Platform')
  })
})
