import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AuthLayout from '../AuthLayout.vue'

const { appState, fetchPublicSettings } = vi.hoisted(() => ({
  appState: {
    siteName: 'Sub2API',
    siteLogo: '',
    cachedPublicSettings: null as null | { site_subtitle?: string },
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn()
  },
  fetchPublicSettings: vi.fn()
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    ...appState,
    fetchPublicSettings
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'auth.layout.kicker': 'Fusion Access',
    'auth.layout.subtitle': 'Sub2API core online, API Fusion shell ready.',
    'auth.layout.providerPool': 'Provider pool',
    'auth.layout.providerPoolDesc': 'OpenAI / Gemini / Claude',
    'auth.layout.routing': 'Adaptive routing',
    'auth.layout.routingDesc': 'Groups, channels and quotas',
    'auth.layout.tracing': 'Request tracing',
    'auth.layout.tracingDesc': 'Correlate failed calls quickly',
    'auth.layout.consoleReady': 'Console ready'
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

describe('AuthLayout fusion shell', () => {
  beforeEach(() => {
    appState.siteName = 'Sub2API'
    appState.siteLogo = ''
    appState.cachedPublicSettings = null
    appState.publicSettingsLoaded = true
    fetchPublicSettings.mockReset()
  })

  it('rebrands the default auth entrance and exposes provider operations cues', () => {
    const wrapper = mount(AuthLayout, {
      slots: {
        default: '<form>Sign in form</form>',
        footer: '<a>Footer link</a>'
      }
    })

    const text = wrapper.text()
    expect(text).toContain('API Fusion')
    expect(text).toContain('Fusion Access')
    expect(text).toContain('Sub2API core online')
    expect(text).toContain('Provider pool')
    expect(text).toContain('Adaptive routing')
    expect(text).toContain('Request tracing')
    expect(text).not.toContain('Subscription to API Conversion Platform')
  })
})
