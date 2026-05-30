<template>
  <AppLayout>
    <div class="dashboard-shell space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <section class="dashboard-hero">
          <div class="hero-copy">
            <p class="hero-kicker">{{ appStore.siteName }}</p>
            <h1>{{ t('dashboard.title') }}</h1>
            <p>{{ greetingText }}</p>
          </div>

          <div class="hero-meter" v-if="!authStore.isSimpleMode">
            <div class="meter-topline">
              <span>{{ t('dashboard.balance') }}</span>
              <RouterLink to="/redeem">{{ t('dashboard.redeemTopUp') }}</RouterLink>
            </div>
            <div class="meter-value">${{ formatMoney(user?.balance || 0) }}</div>
            <div class="meter-grid">
              <div>
                <span>{{ t('dashboard.used') }}</span>
                <strong>${{ formatCost(stats.total_actual_cost || 0) }}</strong>
              </div>
              <div>
                <span>{{ t('dashboard.todayCost') }}</span>
                <strong>${{ formatCost(stats.today_actual_cost || 0) }}</strong>
              </div>
              <div>
                <span>RPM</span>
                <strong>{{ formatCompact(stats.rpm || 0) }}</strong>
              </div>
            </div>
          </div>
        </section>

        <section class="dashboard-grid">
          <article class="console-panel account-panel">
            <div class="panel-heading">
              <div>
                <h2>{{ t('dashboard.accountCredits') }}</h2>
                <p>{{ t('dashboard.accountCreditsHint') }}</p>
              </div>
              <RouterLink to="/redeem" class="panel-link">{{ t('dashboard.redeemTopUp') }} →</RouterLink>
            </div>

            <div class="credit-row">
              <div>
                <span>{{ t('dashboard.balanceBase') }}</span>
                <strong>${{ formatMoney(user?.balance || 0) }}</strong>
              </div>
              <div>
                <span>{{ t('dashboard.used') }}</span>
                <strong>${{ formatCost(stats.total_actual_cost || 0) }}</strong>
              </div>
            </div>

            <div class="signal-row">
              <div class="signal-pill">
                <span>{{ t('dashboard.todayRequests') }}</span>
                <strong>{{ formatCompact(stats.today_requests || 0) }}</strong>
              </div>
              <div class="signal-pill">
                <span>{{ t('dashboard.todayTokens') }}</span>
                <strong>{{ formatCompact(stats.today_tokens || 0) }}</strong>
              </div>
              <div class="signal-pill">
                <span>{{ t('dashboard.avgResponse') }}</span>
                <strong>{{ formatDuration(stats.average_duration_ms || 0) }}</strong>
              </div>
            </div>
          </article>

          <article class="console-panel announcement-panel">
            <div class="panel-heading">
              <div>
                <h2>{{ t('dashboard.systemAnnouncements') }}</h2>
                <p>{{ t('dashboard.systemAnnouncementsHint') }}</p>
              </div>
              <span v-if="announcementStore.unreadCount" class="notice-count">{{ announcementStore.unreadCount }}</span>
            </div>

            <div v-if="announcementStore.loading" class="notice-empty">
              <LoadingSpinner size="sm" />
            </div>
            <div v-else-if="latestAnnouncements.length" class="notice-list">
              <article v-for="item in latestAnnouncements" :key="item.id" class="notice-item">
                <h3>{{ item.title }}</h3>
                <p>{{ compactText(item.content) }}</p>
              </article>
            </div>
            <div v-else class="notice-empty">
              <strong>{{ t('dashboard.noAnnouncements') }}</strong>
              <span>{{ t('dashboard.noAnnouncementsHint') }}</span>
            </div>
          </article>
        </section>

        <section class="shortcut-grid" aria-label="Dashboard shortcuts">
          <RouterLink v-for="item in shortcutItems" :key="item.to" :to="item.to" class="shortcut-card">
            <span class="shortcut-icon" :class="item.tone">{{ item.symbol }}</span>
            <span>
              <strong>{{ item.title }}</strong>
              <small>{{ item.description }}</small>
            </span>
          </RouterLink>
        </section>

        <UserDashboardStats
          :stats="stats"
          :balance="user?.balance || 0"
          :is-simple="authStore.isSimpleMode"
          :platform-quotas="platformQuotas"
        />

        <UserDashboardCharts
          v-model:startDate="startDate"
          v-model:endDate="endDate"
          v-model:granularity="granularity"
          :loading="loadingCharts"
          :trend="trendData"
          :models="modelStats"
          @dateRangeChange="loadCharts"
          @granularityChange="loadCharts"
          @refresh="refreshAll"
        />

        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="lg:col-span-2">
            <UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" />
          </div>
          <div class="lg:col-span-1">
            <UserDashboardQuickActions />
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAnnouncementStore, useAppStore, useAuthStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import { getMyPlatformQuotas } from '@/api/user'
import type { ModelStat, PlatformQuotaItem, TrendDataPoint, UsageLog } from '@/types'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const announcementStore = useAnnouncementStore()
const user = computed(() => authStore.user)

const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)

const formatLD = (d: Date) => d.toISOString().split('T')[0]
const startDate = ref(formatLD(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatLD(new Date()))
const granularity = ref<'day' | 'hour'>('day')

const greetingText = computed(() => t('dashboard.welcomeBack', { name: user.value?.username || user.value?.email || '' }))
const latestAnnouncements = computed(() => announcementStore.announcements.slice(0, 2))

const shortcutItems = computed(() => [
  {
    to: '/available-channels',
    symbol: 'M',
    tone: 'tone-blue',
    title: t('dashboard.modelSquare'),
    description: t('dashboard.modelSquareHint')
  },
  {
    to: '/affiliate',
    symbol: '%',
    tone: 'tone-emerald',
    title: t('dashboard.inviteRebate'),
    description: t('dashboard.inviteRebateHint')
  },
  {
    to: '/redeem',
    symbol: '$',
    tone: 'tone-amber',
    title: t('dashboard.redeemRecharge'),
    description: t('dashboard.redeemRechargeHint')
  }
])

const loadStats = async () => {
  loading.value = true
  try {
    await authStore.refreshUser()
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
  } finally {
    loading.value = false
  }
}

const loadCharts = async () => {
  loadingCharts.value = true
  try {
    const [trend, models] = await Promise.all([
      usageAPI.getDashboardTrend({
        start_date: startDate.value,
        end_date: endDate.value,
        granularity: granularity.value
      }),
      usageAPI.getDashboardModels({
        start_date: startDate.value,
        end_date: endDate.value
      })
    ])
    trendData.value = trend.trend || []
    modelStats.value = models.models || []
  } catch (error) {
    console.error('Failed to load charts:', error)
  } finally {
    loadingCharts.value = false
  }
}

const loadRecent = async () => {
  loadingUsage.value = true
  try {
    const res = await usageAPI.getByDateRange(startDate.value, endDate.value)
    recentUsage.value = res.items.slice(0, 5)
  } catch (error) {
    console.error('Failed to load recent usage:', error)
  } finally {
    loadingUsage.value = false
  }
}

const loadPlatformQuotas = async () => {
  try {
    const data = await getMyPlatformQuotas()
    platformQuotas.value = data.platform_quotas ?? []
  } catch (error) {
    console.warn('Failed to load platform quotas:', error)
    platformQuotas.value = []
  }
}

const refreshAll = () => {
  loadStats()
  loadCharts()
  loadRecent()
  loadPlatformQuotas()
  announcementStore.fetchAnnouncements()
}

function formatMoney(value: number): string {
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(value || 0)
}

function formatCost(value: number): string {
  return (value || 0).toFixed(4)
}

function formatCompact(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return Math.round(value || 0).toString()
}

function formatDuration(ms: number): string {
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms || 0)}ms`
}

function compactText(value: string): string {
  return value.replace(/<[^>]*>/g, '').replace(/\s+/g, ' ').trim()
}

onMounted(() => {
  refreshAll()
})
</script>

<style scoped>
.dashboard-shell {
  color: rgb(17 24 39);
}

.dashboard-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 1rem;
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.96), rgba(20, 83, 45, 0.92)),
    radial-gradient(circle at 92% 12%, rgba(245, 158, 11, 0.28), transparent 32%);
  padding: clamp(1.25rem, 3vw, 2rem);
  color: white;
}

.hero-copy {
  max-width: 46rem;
}

.hero-kicker {
  margin-bottom: 0.75rem;
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(190 242 100);
}

.hero-copy h1 {
  margin: 0;
  font-size: clamp(2rem, 4vw, 4rem);
  font-weight: 800;
  line-height: 0.95;
  letter-spacing: 0;
}

.hero-copy p:last-child {
  margin-top: 0.85rem;
  max-width: 32rem;
  color: rgb(209 250 229);
}

.hero-meter,
.console-panel,
.shortcut-card {
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background: rgb(255 255 255);
  box-shadow: 0 18px 55px rgba(15, 23, 42, 0.08);
}

.hero-meter {
  color: rgb(15 23 42);
  padding: 1rem;
}

.meter-topline,
.panel-heading,
.credit-row,
.signal-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.meter-topline span,
.credit-row span,
.signal-pill span {
  font-size: 0.75rem;
  color: rgb(100 116 139);
}

.meter-topline a,
.panel-link {
  font-size: 0.8rem;
  font-weight: 700;
  color: rgb(5 150 105);
}

.meter-value {
  margin-top: 0.5rem;
  font-size: clamp(2rem, 5vw, 3.4rem);
  font-weight: 800;
  letter-spacing: 0;
}

.meter-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
  margin-top: 1rem;
}

.meter-grid div,
.signal-pill {
  border-radius: 8px;
  background: rgb(248 250 252);
  padding: 0.75rem;
}

.meter-grid span,
.meter-grid strong {
  display: block;
}

.meter-grid span {
  font-size: 0.72rem;
  color: rgb(100 116 139);
}

.meter-grid strong {
  margin-top: 0.2rem;
  font-size: 0.95rem;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 1rem;
}

.console-panel {
  padding: 1.25rem;
}

.panel-heading {
  margin-bottom: 1rem;
}

.panel-heading h2 {
  margin: 0;
  font-size: 1rem;
  font-weight: 800;
}

.panel-heading p {
  margin-top: 0.25rem;
  font-size: 0.82rem;
  color: rgb(100 116 139);
}

.credit-row {
  align-items: stretch;
}

.credit-row div {
  flex: 1;
  border-radius: 8px;
  background: rgb(248 250 252);
  padding: 1rem;
}

.credit-row strong {
  display: block;
  margin-top: 0.35rem;
  font-size: clamp(1.45rem, 4vw, 2.5rem);
  line-height: 1;
}

.signal-row {
  margin-top: 1rem;
  align-items: stretch;
}

.signal-pill {
  flex: 1;
}

.signal-pill strong {
  display: block;
  margin-top: 0.3rem;
  font-size: 1.1rem;
}

.notice-count {
  display: inline-flex;
  min-width: 1.75rem;
  height: 1.75rem;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: rgb(220 38 38);
  color: white;
  font-size: 0.8rem;
  font-weight: 800;
}

.notice-list {
  display: grid;
  gap: 0.75rem;
}

.notice-item {
  border-left: 3px solid rgb(16 185 129);
  border-radius: 8px;
  background: rgb(248 250 252);
  padding: 0.9rem 1rem;
}

.notice-item h3 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 800;
}

.notice-item p {
  margin-top: 0.35rem;
  display: -webkit-box;
  overflow: hidden;
  color: rgb(71 85 105);
  font-size: 0.85rem;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
}

.notice-empty {
  display: grid;
  min-height: 7rem;
  place-items: center;
  border-radius: 8px;
  background: rgb(248 250 252);
  color: rgb(100 116 139);
  text-align: center;
}

.notice-empty strong {
  color: rgb(15 23 42);
}

.shortcut-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1rem;
}

.shortcut-card {
  display: flex;
  min-height: 6rem;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  color: rgb(15 23 42);
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    box-shadow 0.18s ease;
}

.shortcut-card:hover {
  transform: translateY(-2px);
  border-color: rgb(16 185 129);
  box-shadow: 0 20px 60px rgba(15, 23, 42, 0.12);
}

.shortcut-icon {
  display: inline-flex;
  width: 3rem;
  height: 3rem;
  flex: 0 0 3rem;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  font-weight: 900;
  color: white;
}

.shortcut-card strong,
.shortcut-card small {
  display: block;
}

.shortcut-card strong {
  font-size: 0.98rem;
}

.shortcut-card small {
  margin-top: 0.3rem;
  color: rgb(100 116 139);
  line-height: 1.45;
}

.tone-blue {
  background: rgb(37 99 235);
}

.tone-emerald {
  background: rgb(5 150 105);
}

.tone-amber {
  background: rgb(217 119 6);
}

:global(.dark) .dashboard-shell {
  color: rgb(243 244 246);
}

:global(.dark) .hero-meter,
:global(.dark) .console-panel,
:global(.dark) .shortcut-card {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
}

:global(.dark) .hero-meter,
:global(.dark) .shortcut-card {
  color: rgb(243 244 246);
}

:global(.dark) .meter-grid div,
:global(.dark) .credit-row div,
:global(.dark) .signal-pill,
:global(.dark) .notice-item,
:global(.dark) .notice-empty {
  background: rgba(31, 41, 55, 0.72);
}

:global(.dark) .panel-heading p,
:global(.dark) .meter-grid span,
:global(.dark) .credit-row span,
:global(.dark) .signal-pill span,
:global(.dark) .shortcut-card small,
:global(.dark) .notice-item p {
  color: rgb(156 163 175);
}

:global(.dark) .notice-empty strong {
  color: rgb(243 244 246);
}

@media (min-width: 768px) {
  .dashboard-hero {
    grid-template-columns: minmax(0, 1.35fr) minmax(20rem, 0.65fr);
    align-items: end;
  }

  .dashboard-grid {
    grid-template-columns: minmax(0, 1fr) minmax(20rem, 0.72fr);
  }

  .shortcut-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .credit-row,
  .signal-row,
  .meter-grid {
    grid-template-columns: 1fr;
    flex-direction: column;
  }

  .meter-grid {
    display: grid;
  }
}
</style>
