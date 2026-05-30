<template>
  <div class="relative">
    <template v-if="isAdmin">
      <button
        ref="triggerRef"
        @click="toggleDropdown"
        class="inline-flex items-center gap-1.5 rounded-md border px-2 py-1 text-[11px] font-semibold transition-colors"
        :class="[
          hasUpdate
            ? 'border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100 dark:border-amber-700/50 dark:bg-amber-900/20 dark:text-amber-300'
            : 'border-slate-200 bg-slate-50 text-slate-600 hover:bg-slate-100 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300'
        ]"
        :title="hasUpdate ? t('version.upstreamUpdateAvailable') : t('version.upToDate')"
      >
        <span v-if="currentVersion">v{{ currentVersion }}</span>
        <span v-else class="h-3 w-12 animate-pulse rounded bg-slate-200 dark:bg-dark-600"></span>
        <span v-if="hasUpdate" class="h-1.5 w-1.5 rounded-full bg-amber-500"></span>
      </button>

      <transition name="dropdown">
        <div
          v-if="dropdownOpen"
          ref="dropdownRef"
          class="absolute left-0 z-50 mt-2 w-72 overflow-hidden rounded-lg border border-slate-200 bg-white shadow-xl shadow-slate-900/10 dark:border-dark-700 dark:bg-dark-800"
        >
          <div class="flex items-start justify-between gap-3 border-b border-slate-100 px-4 py-3 dark:border-dark-700">
            <div>
              <p class="text-sm font-semibold text-slate-900 dark:text-white">
                {{ t('version.fusionBuild') }}
              </p>
              <p class="mt-0.5 text-xs text-slate-500 dark:text-dark-400">
                {{ t('version.officialUpdateDisabled') }}
              </p>
            </div>
            <button
              @click="refreshVersion(true)"
              class="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-700 dark:hover:text-dark-200"
              :disabled="loading"
              :title="t('version.refresh')"
            >
              <Icon
                name="refresh"
                size="sm"
                :stroke-width="2"
                :class="{ 'animate-spin': loading }"
              />
            </button>
          </div>

          <div class="space-y-3 p-4">
            <div v-if="loading" class="flex items-center justify-center py-6">
              <div class="spinner text-primary-500"></div>
            </div>

            <template v-else>
              <div class="grid grid-cols-2 gap-2">
                <div class="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                  <p class="text-[11px] font-medium text-slate-500 dark:text-dark-400">
                    {{ t('version.currentVersion') }}
                  </p>
                  <p class="mt-1 text-base font-bold text-slate-950 dark:text-white">
                    v{{ currentVersion || '--' }}
                  </p>
                </div>
                <div class="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
                  <p class="text-[11px] font-medium text-slate-500 dark:text-dark-400">
                    {{ t('version.upstreamVersion') }}
                  </p>
                  <p class="mt-1 text-base font-bold text-slate-950 dark:text-white">
                    v{{ latestVersion || currentVersion || '--' }}
                  </p>
                </div>
              </div>

              <div
                v-if="hasUpdate"
                class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5 dark:border-amber-800/60 dark:bg-amber-900/20"
              >
                <div class="flex items-start gap-2">
                  <Icon name="exclamationCircle" size="sm" class="mt-0.5 text-amber-600 dark:text-amber-300" />
                  <div>
                    <p class="text-sm font-semibold text-amber-800 dark:text-amber-200">
                      {{ t('version.upstreamUpdateAvailable') }}
                    </p>
                    <p class="mt-1 text-xs leading-5 text-amber-700/80 dark:text-amber-200/80">
                      {{ t('version.manualMergeHint') }}
                    </p>
                  </div>
                </div>
              </div>

              <div
                v-else
                class="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2.5 text-sm font-medium text-emerald-700 dark:border-emerald-800/60 dark:bg-emerald-900/20 dark:text-emerald-300"
              >
                {{ t('version.upToDate') }}
              </div>

              <a
                v-if="releaseInfo?.html_url && releaseInfo.html_url !== '#'"
                :href="releaseInfo.html_url"
                target="_blank"
                rel="noopener noreferrer"
                class="flex items-center justify-center gap-1.5 rounded-md border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 transition-colors hover:border-slate-300 hover:bg-slate-50 hover:text-slate-900 dark:border-dark-700 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
              >
                {{ t('version.viewUpstreamRelease') }}
                <Icon name="externalLink" size="xs" :stroke-width="2" />
              </a>
            </template>
          </div>
        </div>
      </transition>
    </template>

    <span v-else-if="version" class="text-xs text-slate-500 dark:text-dark-400">
      v{{ version }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const props = defineProps<{
  version?: string
}>()

const authStore = useAuthStore()
const appStore = useAppStore()

const isAdmin = computed(() => authStore.isAdmin)
const dropdownOpen = ref(false)
const triggerRef = ref<HTMLElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)

const loading = computed(() => appStore.versionLoading)
const currentVersion = computed(() => appStore.currentVersion || props.version || '')
const latestVersion = computed(() => appStore.latestVersion)
const hasUpdate = computed(() => appStore.hasUpdate)
const releaseInfo = computed(() => appStore.releaseInfo)

function toggleDropdown() {
  dropdownOpen.value = !dropdownOpen.value
}

function closeDropdown() {
  dropdownOpen.value = false
}

async function refreshVersion(force = true) {
  if (!isAdmin.value) return
  await appStore.fetchVersion(force)
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as Node
  if (
    dropdownRef.value &&
    !dropdownRef.value.contains(target) &&
    triggerRef.value &&
    !triggerRef.value.contains(target)
  ) {
    closeDropdown()
  }
}

onMounted(() => {
  if (isAdmin.value) {
    appStore.fetchVersion(false)
  }
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.16s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}
</style>
