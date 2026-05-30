import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('fusion branding locale copy', () => {
  it('uses API Fusion as the onboarding welcome brand', () => {
    expect(zh.onboarding.admin.welcome.title).toContain('API Fusion')
    expect(zh.onboarding.user.welcome.title).toContain('API Fusion')
    expect(en.onboarding.admin.welcome.title).toContain('API Fusion')
    expect(en.onboarding.user.welcome.title).toContain('API Fusion')

    expect(zh.onboarding.admin.welcome.title).not.toContain('Sub2API')
    expect(zh.onboarding.user.welcome.title).not.toContain('Sub2API')
    expect(en.onboarding.admin.welcome.title).not.toContain('Sub2API')
    expect(en.onboarding.user.welcome.title).not.toContain('Sub2API')

    expect(zh.onboarding.admin.welcome.description).toContain('API Fusion')
    expect(zh.onboarding.user.welcome.description).toContain('API Fusion')
    expect(en.onboarding.admin.welcome.description).toContain('API Fusion')
    expect(en.onboarding.user.welcome.description).toContain('API Fusion')

    expect(zh.onboarding.admin.welcome.description).not.toContain('Sub2API 是一个强大的 AI 服务中转平台')
    expect(en.onboarding.admin.welcome.description).not.toContain('Sub2API is a powerful AI service gateway platform')
  })

  it('uses fusion-specific copy for the public auth entrance', () => {
    expect(zh.auth.fusionLoginTitle).toContain('API Fusion')
    expect(zh.auth.fusionRegisterTitle).toContain('API Fusion')
    expect(en.auth.fusionLoginTitle).toContain('API Fusion')
    expect(en.auth.fusionRegisterTitle).toContain('API Fusion')

    expect(zh.auth.layout.subtitle).toContain('Sub2API')
    expect(en.auth.layout.subtitle).toContain('Sub2API')
    expect(zh.auth.layout.providerPool).toContain('供应商')
    expect(en.auth.layout.providerPool).toContain('Provider')
  })
})
