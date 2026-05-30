<template>
  <div class="auth-shell relative min-h-screen overflow-hidden">
    <div class="auth-shell-grid pointer-events-none absolute inset-0"></div>

    <div
      class="relative z-10 mx-auto grid min-h-screen w-full max-w-7xl content-start gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[minmax(0,1fr)_minmax(360px,460px)] lg:items-center lg:content-center lg:gap-10 lg:px-8"
    >
      <section class="auth-visual flex flex-col justify-between rounded-lg p-5 sm:p-6 lg:min-h-[620px] lg:p-8">
        <div>
          <p class="auth-kicker text-xs font-semibold uppercase">
            {{ t('auth.layout.kicker') }}
          </p>

          <div v-if="settingsLoaded" class="mt-5 flex items-center gap-4">
            <div class="auth-logo flex h-14 w-14 items-center justify-center overflow-hidden rounded-lg">
              <img
                v-if="siteLogo"
                :src="siteLogo"
                alt="Logo"
                class="h-full w-full object-contain"
              />
              <span v-else class="text-lg font-black">AF</span>
            </div>
            <div class="min-w-0">
              <h1 class="truncate text-3xl font-black text-gray-950 dark:text-white">
                {{ siteName }}
              </h1>
              <p class="mt-1 max-w-xl text-sm leading-6 text-gray-600 dark:text-dark-300">
                {{ siteSubtitle }}
              </p>
            </div>
          </div>

          <div class="mt-8 hidden gap-3 sm:grid sm:grid-cols-3 lg:grid-cols-1">
            <div class="auth-signal-row">
              <span class="auth-signal-mark bg-teal-500"></span>
              <div>
                <p class="text-sm font-semibold text-gray-950 dark:text-white">
                  {{ t('auth.layout.providerPool') }}
                </p>
                <p class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('auth.layout.providerPoolDesc') }}
                </p>
              </div>
            </div>
            <div class="auth-signal-row">
              <span class="auth-signal-mark bg-sky-500"></span>
              <div>
                <p class="text-sm font-semibold text-gray-950 dark:text-white">
                  {{ t('auth.layout.routing') }}
                </p>
                <p class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('auth.layout.routingDesc') }}
                </p>
              </div>
            </div>
            <div class="auth-signal-row">
              <span class="auth-signal-mark bg-amber-500"></span>
              <div>
                <p class="text-sm font-semibold text-gray-950 dark:text-white">
                  {{ t('auth.layout.tracing') }}
                </p>
                <p class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('auth.layout.tracingDesc') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <div class="auth-route-board mt-8 hidden lg:block">
          <div class="flex items-center justify-between text-xs font-semibold text-gray-500 dark:text-dark-400">
            <span>{{ t('auth.layout.consoleReady') }}</span>
            <span>{{ currentYear }}</span>
          </div>
          <div class="mt-4 grid grid-cols-[90px_1fr_82px] gap-2 text-xs">
            <span class="auth-route-pill">OpenAI</span>
            <span class="auth-route-line"></span>
            <span class="auth-route-pill">Trace</span>
            <span class="auth-route-pill">Gemini</span>
            <span class="auth-route-line auth-route-line-alt"></span>
            <span class="auth-route-pill">Cost</span>
            <span class="auth-route-pill">Claude</span>
            <span class="auth-route-line"></span>
            <span class="auth-route-pill">Pool</span>
          </div>
        </div>
      </section>

      <main class="w-full">
        <div class="auth-card rounded-lg p-6 shadow-xl shadow-black/5 sm:p-8">
          <slot />
        </div>

        <div class="mt-5 text-center text-sm">
          <slot name="footer" />
        </div>

        <div class="mt-6 text-center text-xs text-gray-500 dark:text-dark-500">
          &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { isUpstreamDefaultSiteSubtitle, resolveFusionSiteName } from '@/utils/fusionBranding'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()
const { t } = useI18n()

const siteName = computed(() => resolveFusionSiteName(appStore.siteName))
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => {
  const subtitle = appStore.cachedPublicSettings?.site_subtitle?.trim()
  if (subtitle && !isUpstreamDefaultSiteSubtitle(subtitle)) {
    return subtitle
  }
  return t('auth.layout.subtitle')
})
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-shell {
  background:
    linear-gradient(135deg, rgba(244, 241, 234, 0.98), rgba(237, 244, 240, 0.98) 48%, rgba(243, 246, 248, 0.98));
}

.dark .auth-shell {
  background:
    linear-gradient(135deg, rgba(20, 22, 20, 1), rgba(18, 29, 28, 1) 52%, rgba(28, 26, 21, 1));
}

.auth-shell-grid {
  background-image:
    linear-gradient(rgba(20, 84, 74, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(20, 84, 74, 0.08) 1px, transparent 1px),
    linear-gradient(135deg, transparent 0%, transparent 58%, rgba(217, 119, 6, 0.1) 58%, transparent 72%);
  background-size: 56px 56px, 56px 56px, 100% 100%;
}

.auth-visual {
  border: 1px solid rgba(20, 84, 74, 0.14);
  background: rgba(255, 255, 255, 0.58);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.78);
}

.dark .auth-visual {
  border-color: rgba(148, 163, 184, 0.14);
  background: rgba(14, 18, 17, 0.62);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.auth-kicker {
  color: #0f766e;
}

.dark .auth-kicker {
  color: #5eead4;
}

.auth-logo {
  border: 1px solid rgba(15, 118, 110, 0.18);
  background: #0f172a;
  color: #f8fafc;
}

.dark .auth-logo {
  background: #f8fafc;
  color: #0f172a;
}

.auth-signal-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-height: 76px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.72);
  padding: 0.875rem;
}

.dark .auth-signal-row {
  border-color: rgba(148, 163, 184, 0.14);
  background: rgba(15, 23, 42, 0.36);
}

.auth-signal-mark {
  width: 0.65rem;
  height: 2.75rem;
  flex: 0 0 auto;
  border-radius: 999px;
}

.auth-route-board {
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.58);
  padding: 1rem;
}

.dark .auth-route-board {
  border-color: rgba(148, 163, 184, 0.14);
  background: rgba(2, 6, 23, 0.26);
}

.auth-route-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 30px;
  border-radius: 999px;
  border: 1px solid rgba(15, 118, 110, 0.2);
  background: rgba(240, 253, 250, 0.88);
  color: #115e59;
  font-weight: 700;
}

.dark .auth-route-pill {
  border-color: rgba(94, 234, 212, 0.24);
  background: rgba(20, 184, 166, 0.12);
  color: #99f6e4;
}

.auth-route-line {
  align-self: center;
  height: 2px;
  border-radius: 999px;
  background: linear-gradient(90deg, #14b8a6, #0284c7);
}

.auth-route-line-alt {
  background: linear-gradient(90deg, #f59e0b, #14b8a6);
}

.auth-card {
  border: 1px solid rgba(15, 23, 42, 0.08);
  background: rgba(255, 255, 255, 0.86);
  backdrop-filter: blur(20px);
}

.dark .auth-card {
  border-color: rgba(148, 163, 184, 0.16);
  background: rgba(15, 23, 42, 0.76);
}
</style>
