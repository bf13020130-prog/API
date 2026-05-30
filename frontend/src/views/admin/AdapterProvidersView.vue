<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <section class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="max-w-3xl">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.adapterProviders.title') }}
              </h2>
              <span
                class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium"
                :class="diagnostics?.observe_only ? 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300' : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'"
              >
                {{ diagnostics?.observe_only ? t('admin.adapterProviders.observeOnly') : t('admin.adapterProviders.enforced') }}
              </span>
              <span
                class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium"
                :class="diagnostics?.enforcement_enabled ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300' : 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-gray-300'"
              >
                {{ diagnostics?.enforcement_enabled ? t('admin.adapterProviders.enforcementEnabled') : t('admin.adapterProviders.enforcementDisabled') }}
              </span>
            </div>
            <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">
              {{ t('admin.adapterProviders.description') }}
            </p>
          </div>
          <div class="flex shrink-0 flex-wrap gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="reload">
              <Icon name="refresh" size="sm" class="mr-1.5" :class="{ 'animate-spin': loading }" />
              {{ loading ? t('common.loading') : t('common.refresh') }}
            </button>
            <button type="button" class="btn btn-primary btn-sm" @click="openCreateDialog">
              <Icon name="plus" size="sm" class="mr-1.5" />
              {{ t('admin.adapterProviders.createProvider') }}
            </button>
          </div>
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.adapterProviders.operator.title') }}
              </h3>
              <span
                class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium"
                :class="operatorNeedsAttention ? 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300' : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'"
              >
                {{ operatorNeedsAttention ? t('admin.adapterProviders.operator.needsAttention') : t('admin.adapterProviders.operator.noIssues') }}
              </span>
            </div>
            <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
              {{ t('admin.adapterProviders.operator.subtitle') }}
            </p>
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.adapterProviders.operator.readyRoutes') }}:
            <span class="font-mono text-gray-900 dark:text-white">{{ summary.activeSlugs }}</span>
          </div>
        </div>

        <div class="mt-4 grid grid-cols-2 gap-x-4 gap-y-5 md:grid-cols-3 xl:grid-cols-6">
          <button
            v-for="metric in operatorMetrics"
            :key="metric.label"
            type="button"
            class="min-w-0 border-l border-gray-200 pl-3 text-left transition-colors dark:border-dark-700"
            :class="metric.requestFocus
              ? 'rounded-r-md pr-2 py-1 hover:border-primary-400 hover:bg-primary-50/60 focus:outline-none focus:ring-2 focus:ring-primary-500/40 dark:hover:border-primary-500 dark:hover:bg-primary-900/10'
              : 'cursor-default'"
            :disabled="!metric.requestFocus"
            :aria-pressed="metric.requestFocus ? requestFocus === metric.requestFocus : undefined"
            :title="metric.requestFocus ? t('admin.adapterProviders.operator.focusRequests', { metric: metric.label }) : undefined"
            @click="metric.requestFocus && focusAdapterRequests(metric.requestFocus)"
          >
            <span class="block truncate text-xs font-medium text-gray-500 dark:text-gray-400">{{ metric.label }}</span>
            <span class="mt-1 block truncate text-xl font-semibold text-gray-900 dark:text-white">{{ metric.value }}</span>
          </button>
        </div>
      </section>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
        <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.adapterProviders.usage.totalRequests') }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ usageSummary?.total_requests || 0 }}</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.adapterProviders.usage.successRate') }}</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-300">{{ usageSuccessRate }}</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.adapterProviders.usage.totalCost') }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatCost(usageSummary?.cost_usd) }}</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.adapterProviders.usage.billableUnits') }}</p>
          <p class="mt-2 text-2xl font-semibold text-slate-600 dark:text-slate-300">{{ usageSummary?.billable_units || 0 }}</p>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
        <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.adapterProviders.totalConfigured') }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ summary.total }}</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.adapterProviders.activeConfigured') }}</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-300">{{ summary.active }}</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.adapterProviders.disabledConfigured') }}</p>
          <p class="mt-2 text-2xl font-semibold text-slate-600 dark:text-slate-300">{{ summary.disabled }}</p>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.adapterProviders.invalidConfigured') }}</p>
          <p class="mt-2 text-2xl font-semibold text-red-600 dark:text-red-300">{{ summary.invalid }}</p>
        </div>
      </div>

      <section class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.adapterProviders.activeSlugs') }}
            </h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.adapterProviders.activeSlugsHint') }}
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <span
              v-for="slug in diagnostics?.active_slugs || []"
              :key="slug"
              class="inline-flex items-center rounded-md bg-emerald-50 px-2 py-1 font-mono text-xs font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300"
            >
              {{ slug }}
            </span>
            <span v-if="!loading && summary.activeSlugs === 0" class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.adapterProviders.noActiveSlugs') }}
            </span>
          </div>
        </div>

        <DataTable :columns="columns" :data="diagnosticRows" :loading="loading" row-key="slug" :sticky-actions-column="false">
          <template #cell-slug="{ row }">
            <div class="flex min-w-0 flex-col">
              <span class="font-mono text-sm font-medium text-gray-900 dark:text-white">{{ row.slug || '-' }}</span>
              <span class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">{{ row.name || '-' }}</span>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium" :class="statusBadgeClass(row)">
              {{ statusLabel(row) }}
            </span>
          </template>

          <template #cell-capabilities="{ row }">
            <div class="flex max-w-xs flex-wrap gap-1">
              <span
                v-for="capability in row.capabilities"
                :key="capability"
                class="inline-flex rounded-md bg-gray-100 px-2 py-0.5 font-mono text-[11px] text-gray-700 dark:bg-dark-700 dark:text-gray-200"
              >
                {{ capability }}
              </span>
              <span v-if="!row.capabilities?.length" class="text-xs text-gray-400">-</span>
            </div>
          </template>

          <template #cell-base_url="{ row }">
            <span class="block max-w-sm truncate font-mono text-xs text-gray-700 dark:text-gray-300" :title="row.base_url">
              {{ row.base_url || '-' }}
            </span>
          </template>

          <template #cell-credential_keys="{ row }">
            <div class="flex max-w-xs flex-wrap gap-1">
              <span
                v-for="key in providerBySlug(row.slug)?.credential_keys || []"
                :key="key"
                class="inline-flex rounded-md bg-amber-50 px-2 py-0.5 font-mono text-[11px] text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
              >
                {{ key }}
              </span>
              <span v-if="!providerBySlug(row.slug)?.credential_keys?.length" class="text-xs text-gray-400">-</span>
            </div>
          </template>

          <template #cell-timeout_ms="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatTimeout(row.timeout_ms) }}</span>
          </template>

          <template #cell-reason="{ row }">
            <span class="block max-w-sm whitespace-normal break-words text-xs" :class="row.valid ? 'text-gray-500 dark:text-gray-400' : 'text-red-600 dark:text-red-300'">
              {{ reasonLabel(row.reason) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :title="t('common.edit')"
                @click="openEditDialog(row)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('common.delete')"
                @click="askDelete(row)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.adapterProviders.emptyTitle')"
              :description="t('admin.adapterProviders.emptyDescription')"
              :action-text="t('admin.adapterProviders.createProvider')"
              @action="openCreateDialog"
            />
          </template>
        </DataTable>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.adapterProviders.routePolicies.title') }}
            </h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.adapterProviders.routePolicies.description') }}
            </p>
          </div>
          <button type="button" class="btn btn-primary btn-sm" @click="openCreatePolicyDialog">
            <Icon name="plus" size="sm" class="mr-1.5" />
            {{ t('admin.adapterProviders.routePolicies.createPolicy') }}
          </button>
        </div>

        <DataTable :columns="policyColumns" :data="routePolicies" :loading="loading" row-key="id" :sticky-actions-column="false">
          <template #cell-name="{ row }">
            <div class="flex min-w-0 flex-col">
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
              <span v-if="row.description" class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">{{ row.description }}</span>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium" :class="row.status === 'active' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300' : 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-gray-300'">
              {{ row.status === 'active' ? t('admin.adapterProviders.enabled') : t('admin.adapterProviders.disabled') }}
            </span>
          </template>

          <template #cell-match="{ row }">
            <div class="space-y-1 font-mono text-[11px] text-gray-700 dark:text-gray-300">
              <div>{{ row.match_method || '*' }} {{ row.match_path || '*' }}</div>
              <div>{{ row.match_capability || row.match_model || row.match_group_platform || '*' }}</div>
            </div>
          </template>

          <template #cell-target="{ row }">
            <div class="space-y-1">
              <span class="font-mono text-xs text-gray-900 dark:text-white">{{ row.target }}</span>
              <div v-if="row.adapter_provider_id" class="text-xs text-gray-500 dark:text-gray-400">
                {{ providerLabel(row.adapter_provider_id) }}
              </div>
            </div>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :title="t('common.edit')"
                @click="openEditPolicyDialog(row)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('common.delete')"
                @click="policyDeleteTarget = row"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.adapterProviders.routePolicies.emptyTitle')"
              :description="t('admin.adapterProviders.routePolicies.emptyDescription')"
              :action-text="t('admin.adapterProviders.routePolicies.createPolicy')"
              @action="openCreatePolicyDialog"
            />
          </template>
        </DataTable>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.adapterProviders.requests.title') }}
            </h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.adapterProviders.requests.description') }}
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <select v-model="requestFilters.provider" class="input h-9 w-44 text-xs">
              <option value="">{{ t('admin.adapterProviders.requests.allProviders') }}</option>
              <option v-for="provider in providerRecords" :key="provider.id" :value="provider.slug">
                {{ provider.slug }}
              </option>
            </select>
            <select v-model="requestFilters.status" class="input h-9 w-36 text-xs">
              <option value="">{{ t('admin.adapterProviders.requests.allStatus') }}</option>
              <option value="success">{{ t('admin.adapterProviders.requests.success') }}</option>
              <option value="failed">{{ t('admin.adapterProviders.requests.failed') }}</option>
            </select>
            <select v-model="requestFilters.window" class="input h-9 w-32 text-xs">
              <option value="24h">{{ t('admin.adapterProviders.requests.window24h') }}</option>
              <option value="7d">{{ t('admin.adapterProviders.requests.window7d') }}</option>
              <option value="30d">{{ t('admin.adapterProviders.requests.window30d') }}</option>
              <option value="all">{{ t('admin.adapterProviders.requests.windowAll') }}</option>
            </select>
            <input
              v-model="requestFilters.request_id"
              type="search"
              class="input h-9 w-56 font-mono text-xs"
              :placeholder="t('admin.adapterProviders.requests.searchRequest')"
            />
            <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingRequests" @click="loadAdapterRequests">
              <Icon name="refresh" size="sm" class="mr-1.5" :class="{ 'animate-spin': loadingRequests }" />
              {{ t('common.refresh') }}
            </button>
          </div>
        </div>

        <DataTable :columns="requestColumns" :data="visibleAdapterRequests" :loading="loadingRequests" row-key="id" :sticky-actions-column="false">
          <template #cell-request="{ row }">
            <div class="flex min-w-0 flex-col">
              <span class="truncate font-mono text-xs font-medium text-gray-900 dark:text-white" :title="row.request_id">
                {{ row.request_id }}
              </span>
              <span class="mt-1 font-mono text-[11px] text-gray-500 dark:text-gray-400">
                {{ row.method }} {{ row.path }}
              </span>
            </div>
          </template>

          <template #cell-provider="{ row }">
            <div class="flex min-w-0 flex-col">
              <span class="font-mono text-xs text-gray-900 dark:text-white">{{ row.provider }}</span>
              <span class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">{{ row.capability || '-' }}</span>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium" :class="requestStatusClass(row)">
              {{ requestStatusLabel(row) }}
            </span>
          </template>

          <template #cell-signals="{ row }">
            <div class="flex max-w-xs flex-wrap gap-1">
              <span
                v-for="signal in requestSignals(row)"
                :key="`${signal.label}:${signal.detail || ''}`"
                class="inline-flex max-w-full items-center gap-1 rounded-md px-2 py-0.5 text-[11px] font-medium"
                :class="signal.class"
              >
                <span>{{ signal.label }}</span>
                <span
                  v-if="signal.detail"
                  class="truncate font-mono opacity-75"
                  :title="signal.detail"
                >
                  {{ signal.detail }}
                </span>
              </span>
              <span v-if="requestSignals(row).length === 0" class="text-xs text-gray-400">-</span>
            </div>
          </template>

          <template #cell-cost="{ row }">
            <div class="space-y-1 text-xs text-gray-700 dark:text-gray-300">
              <div class="font-mono">{{ formatCost(row.metadata?.cost_usd) }}</div>
              <div class="text-[11px] text-gray-500 dark:text-gray-400">
                {{ billingState(row) }}
              </div>
            </div>
          </template>

          <template #cell-duration="{ row }">
            <span class="font-mono text-xs text-gray-700 dark:text-gray-300">{{ formatDuration(row.duration_ms) }}</span>
          </template>

          <template #cell-created_at="{ row }">
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(row.created_at) }}</span>
          </template>

          <template #cell-error="{ row }">
            <span class="block max-w-xs truncate text-xs text-red-600 dark:text-red-300" :title="row.error_message || ''">
              {{ row.error_message || '-' }}
            </span>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.adapterProviders.requests.emptyTitle')"
              :description="t('admin.adapterProviders.requests.emptyDescription')"
            />
          </template>
        </DataTable>
        <div class="mt-4 flex flex-col gap-3 border-t border-gray-100 pt-4 dark:border-dark-800 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-xs text-gray-500 dark:text-gray-400">
              {{ adapterRequestsTotal === null
                ? t('admin.adapterProviders.requests.loadedCount', { count: String(adapterRequests.length) })
                : t('admin.adapterProviders.requests.loadedTotalCount', { count: String(adapterRequests.length), total: String(adapterRequestsTotal) })
              }}
            </span>
            <span v-if="adapterRequestsTotalPages > 1" class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.adapterProviders.requests.pageStatus', { page: String(requestPage), total: String(adapterRequestsTotalPages) }) }}
            </span>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              v-if="adapterRequestsTotalPages > 1"
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="loadingRequests || requestPage <= 1"
              @click="goToAdapterRequestPage(requestPage - 1)"
            >
              {{ t('admin.adapterProviders.requests.previousPage') }}
            </button>
            <input
              v-if="adapterRequestsTotalPages > 1"
              v-model.number="requestPageInput"
              type="number"
              min="1"
              :max="adapterRequestsTotalPages"
              class="input h-8 w-20 text-xs"
              :aria-label="t('admin.adapterProviders.requests.pageInputLabel')"
              @keyup.enter="goToAdapterRequestPage(requestPageInput)"
            />
            <button
              v-if="adapterRequestsTotalPages > 1"
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="loadingRequests"
              @click="goToAdapterRequestPage(requestPageInput)"
            >
              {{ t('admin.adapterProviders.requests.goToPage') }}
            </button>
            <button
              v-if="adapterRequestsTotalPages > 1"
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="loadingRequests || requestPage >= adapterRequestsTotalPages"
              @click="goToAdapterRequestPage(requestPage + 1)"
            >
              {{ t('admin.adapterProviders.requests.nextPage') }}
            </button>
            <button
              v-if="adapterRequestsHasMore"
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="loadingMoreRequests"
              @click="loadMoreAdapterRequests"
            >
              <Icon name="chevronDown" size="sm" class="mr-1.5" :class="{ 'animate-bounce': loadingMoreRequests }" />
              {{ loadingMoreRequests ? t('common.loading') : t('admin.adapterProviders.requests.loadMore') }}
            </button>
          </div>
        </div>
      </section>
    </div>

    <BaseDialog
      :show="formOpen"
      :title="editingProvider ? t('admin.adapterProviders.editProvider') : t('admin.adapterProviders.createProvider')"
      width="wide"
      @close="closeForm"
    >
      <form class="space-y-4" @submit.prevent="submitForm">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.name') }}</label>
            <input v-model="form.name" type="text" class="input" required maxlength="100" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.slug') }}</label>
            <input v-model="form.slug" type="text" class="input font-mono" required maxlength="64" placeholder="midjourney" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.status') }}</label>
            <select v-model="form.status" class="input">
              <option value="disabled">{{ t('admin.adapterProviders.disabled') }}</option>
              <option value="active">{{ t('admin.adapterProviders.enabled') }}</option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.adapterType') }}</label>
            <input v-model="form.adapter_type" type="text" class="input font-mono" required />
          </div>
          <div class="md:col-span-2">
            <label class="input-label">{{ t('admin.adapterProviders.form.baseUrl') }}</label>
            <input v-model="form.base_url" type="url" class="input font-mono" required placeholder="https://adapter.internal" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.authMode') }}</label>
            <select v-model="form.auth_mode" class="input">
              <option value="">{{ t('common.none') }}</option>
              <option value="bearer">bearer</option>
              <option value="header">header</option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.timeout') }}</label>
            <input v-model.number="form.timeout_ms" type="number" min="0" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.priority') }}</label>
            <input v-model.number="form.priority" type="number" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.capabilities') }}</label>
            <input v-model="capabilitiesText" type="text" class="input font-mono" required placeholder="image_generation, audio_generation" />
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.adapterProviders.form.credentials') }}</label>
          <textarea
            v-model="credentialsText"
            rows="5"
            class="input font-mono text-xs"
            :placeholder="credentialsPlaceholder"
          />
          <p v-if="editingProvider?.has_credentials" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.adapterProviders.form.existingCredentialKeys', { keys: editingProvider.credential_keys.join(', ') || '-' }) }}
          </p>
        </div>

        <div>
          <label class="input-label">{{ t('admin.adapterProviders.form.extra') }}</label>
          <textarea v-model="extraText" rows="4" class="input font-mono text-xs" placeholder="{}" />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeForm">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="saving" @click="submitForm">
            {{ saving ? t('common.saving') : (editingProvider ? t('common.update') : t('common.create')) }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="deleteTarget !== null"
      :title="t('admin.adapterProviders.deleteTitle')"
      :message="t('admin.adapterProviders.deleteConfirm', { name: deleteTarget?.name || deleteTarget?.slug || '' })"
      :confirm-text="t('common.delete')"
      danger
      @confirm="deleteProvider"
      @cancel="deleteTarget = null"
    />

    <BaseDialog
      :show="policyFormOpen"
      :title="editingPolicy ? t('admin.adapterProviders.routePolicies.editPolicy') : t('admin.adapterProviders.routePolicies.createPolicy')"
      width="wide"
      @close="closePolicyForm"
    >
      <form class="space-y-4" @submit.prevent="submitPolicyForm">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.name') }}</label>
            <input v-model="policyForm.name" type="text" class="input" required maxlength="100" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.form.status') }}</label>
            <select v-model="policyForm.status" class="input">
              <option value="disabled">{{ t('admin.adapterProviders.disabled') }}</option>
              <option value="active">{{ t('admin.adapterProviders.enabled') }}</option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.matchMethod') }}</label>
            <input v-model="policyForm.match_method" type="text" class="input font-mono" placeholder="POST" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.matchPath') }}</label>
            <input v-model="policyForm.match_path" type="text" class="input font-mono" placeholder="/v1/images/generations" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.matchCapability') }}</label>
            <input v-model="policyForm.match_capability" type="text" class="input font-mono" placeholder="image_generation" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.matchModel') }}</label>
            <input v-model="policyForm.match_model" type="text" class="input font-mono" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.matchGroupPlatform') }}</label>
            <input v-model="policyForm.match_group_platform" type="text" class="input font-mono" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.target') }}</label>
            <select v-model="policyForm.target" class="input">
              <option value="new_api_adapter">new_api_adapter</option>
              <option value="sub2api_native">sub2api_native</option>
              <option value="sub2api_upstream">sub2api_upstream</option>
              <option value="unsupported">unsupported</option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.adapterProvider') }}</label>
            <select v-model.number="policyForm.adapter_provider_id" class="input">
              <option :value="undefined">{{ t('common.none') }}</option>
              <option v-for="provider in providerRecords" :key="provider.id" :value="provider.id">
                {{ provider.name }} ({{ provider.slug }})
              </option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.priority') }}</label>
            <input v-model.number="policyForm.priority" type="number" class="input" />
          </div>
          <div class="md:col-span-2">
            <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.description') }}</label>
            <input v-model="policyForm.description" type="text" class="input" />
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.adapterProviders.routePolicies.form.conditions') }}</label>
          <textarea v-model="policyConditionsText" rows="4" class="input font-mono text-xs" placeholder="{}" />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closePolicyForm">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="savingPolicy" @click="submitPolicyForm">
            {{ savingPolicy ? t('common.saving') : (editingPolicy ? t('common.update') : t('common.create')) }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="policyDeleteTarget !== null"
      :title="t('admin.adapterProviders.routePolicies.deleteTitle')"
      :message="t('admin.adapterProviders.routePolicies.deleteConfirm', { name: policyDeleteTarget?.name || '' })"
      :confirm-text="t('common.delete')"
      danger
      @confirm="deletePolicy"
      @cancel="policyDeleteTarget = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  AdapterRequestRecord,
  AdapterUsageSummary,
  AdapterProvider,
  AdapterProviderDiagnostic,
  AdapterProviderDiagnostics,
  AdapterProviderPayload,
  RoutePolicy,
  RoutePolicyPayload
} from '@/api/admin/system'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const loadingRequests = ref(false)
const loadingMoreRequests = ref(false)
const saving = ref(false)
const diagnostics = ref<AdapterProviderDiagnostics | null>(null)
const providerRecords = ref<AdapterProvider[]>([])
const routePolicies = ref<RoutePolicy[]>([])
const adapterRequests = ref<AdapterRequestRecord[]>([])
const adapterRequestsTotal = ref<number | null>(null)
const usageSummary = ref<AdapterUsageSummary | null>(null)
const requestFocus = ref<'all' | 'failed' | 'stream' | 'websocket'>('all')
const adapterRequestPageSize = 100
const adapterRequestsHasMore = ref(false)
const requestPage = ref(1)
const requestPageInput = ref(1)
const requestFilters = ref({
  provider: '',
  status: '',
  request_id: '',
  window: '24h',
})
const formOpen = ref(false)
const editingProvider = ref<AdapterProvider | null>(null)
const deleteTarget = ref<AdapterProviderDiagnostic | null>(null)
const policyFormOpen = ref(false)
const editingPolicy = ref<RoutePolicy | null>(null)
const policyDeleteTarget = ref<RoutePolicy | null>(null)
const savingPolicy = ref(false)
const policyConditionsText = ref('{}')
const capabilitiesText = ref('')
const credentialsText = ref('')
const extraText = ref('{}')
const credentialsExample = '{\n  "token": "..."\n}'

const form = ref({
  name: '',
  slug: '',
  status: 'disabled',
  adapter_type: 'new-api',
  base_url: '',
  auth_mode: '',
  priority: 50,
  timeout_ms: 30000,
})

const policyForm = ref<Partial<RoutePolicyPayload>>({
  name: '',
  status: 'disabled',
  target: 'new_api_adapter',
  priority: 50,
})

const diagnosticRows = computed(() => diagnostics.value?.providers || [])

const providerMap = computed(() => {
  const map = new Map<string, AdapterProvider>()
  for (const provider of providerRecords.value) {
    map.set(provider.slug, provider)
  }
  return map
})

const summary = computed(() => {
  const rows = diagnosticRows.value
  return {
    total: rows.length,
    active: rows.filter((row) => row.enabled).length,
    disabled: rows.filter((row) => row.valid && !row.enabled).length,
    invalid: rows.filter((row) => !row.valid).length,
    activeSlugs: diagnostics.value?.active_slugs?.length || 0,
  }
})

const usageSuccessRate = computed(() => {
  const total = usageSummary.value?.total_requests || 0
  if (!total) return '-'
  const success = usageSummary.value?.success_requests || 0
  return `${Math.round((success / total) * 100)}%`
})

const providerHealth = computed(() => {
  const total = diagnosticRows.value.length
  const healthy = diagnosticRows.value.filter((row) => row.valid && row.enabled).length
  return `${healthy} / ${total}`
})

const activePolicyCount = computed(() => routePolicies.value.filter((policy) => policy.status === 'active').length)

const recentFailedCount = computed(() => adapterRequests.value.filter(requestFailed).length)

const streamFinalizedCount = computed(() => adapterRequests.value.filter((request) => (
  request.metadata?.stream_usage_finalized === true
)).length)

const websocketCount = computed(() => adapterRequests.value.filter((request) => (
  request.metadata?.websocket === true || request.metadata?.transport === 'websocket'
)).length)

const operatorNeedsAttention = computed(() => summary.value.invalid > 0 || recentFailedCount.value > 0)

const operatorMetrics = computed(() => [
  { label: t('admin.adapterProviders.operator.providerHealth'), value: providerHealth.value },
  { label: t('admin.adapterProviders.operator.activePolicies'), value: activePolicyCount.value },
  { label: t('admin.adapterProviders.operator.recentFailures'), value: recentFailedCount.value, requestFocus: 'failed' as const },
  { label: t('admin.adapterProviders.operator.streamFinalized'), value: streamFinalizedCount.value, requestFocus: 'stream' as const },
  { label: t('admin.adapterProviders.operator.websocketTraffic'), value: websocketCount.value, requestFocus: 'websocket' as const },
  { label: t('admin.adapterProviders.operator.costToday'), value: formatCost(usageSummary.value?.cost_usd) },
])

const visibleAdapterRequests = computed(() => {
  switch (requestFocus.value) {
    case 'failed':
      return adapterRequests.value.filter(requestFailed)
    case 'stream':
      return adapterRequests.value.filter((request) => request.metadata?.stream_usage_finalized === true)
    case 'websocket':
      return adapterRequests.value.filter((request) => (
        request.metadata?.websocket === true || request.metadata?.transport === 'websocket'
      ))
    default:
      return adapterRequests.value
  }
})

const adapterRequestsTotalPages = computed(() => {
  const total = adapterRequestsTotal.value || 0
  if (total <= 0) return 1
  return Math.max(1, Math.ceil(total / adapterRequestPageSize))
})

const columns = computed<Column[]>(() => [
  { key: 'slug', label: t('admin.adapterProviders.columns.provider'), sortable: true },
  { key: 'status', label: t('admin.adapterProviders.columns.status'), sortable: true },
  { key: 'capabilities', label: t('admin.adapterProviders.columns.capabilities'), sortable: false },
  { key: 'base_url', label: t('admin.adapterProviders.columns.baseUrl'), sortable: false },
  { key: 'credential_keys', label: t('admin.adapterProviders.columns.credentials'), sortable: false },
  { key: 'timeout_ms', label: t('admin.adapterProviders.columns.timeout'), sortable: true },
  { key: 'reason', label: t('admin.adapterProviders.columns.reason'), sortable: false },
  { key: 'actions', label: t('common.actions'), sortable: false },
])

const policyColumns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.adapterProviders.routePolicies.columns.policy'), sortable: true },
  { key: 'status', label: t('admin.adapterProviders.columns.status'), sortable: true },
  { key: 'match', label: t('admin.adapterProviders.routePolicies.columns.match'), sortable: false },
  { key: 'target', label: t('admin.adapterProviders.routePolicies.columns.target'), sortable: true },
  { key: 'priority', label: t('admin.adapterProviders.columns.priority'), sortable: true },
  { key: 'actions', label: t('common.actions'), sortable: false },
])

const requestColumns = computed<Column[]>(() => [
  { key: 'request', label: t('admin.adapterProviders.requests.columns.request'), sortable: false },
  { key: 'provider', label: t('admin.adapterProviders.requests.columns.provider'), sortable: true },
  { key: 'status', label: t('admin.adapterProviders.requests.columns.status'), sortable: true },
  { key: 'signals', label: t('admin.adapterProviders.requests.columns.signals'), sortable: false },
  { key: 'cost', label: t('admin.adapterProviders.requests.columns.cost'), sortable: false },
  { key: 'duration', label: t('admin.adapterProviders.requests.columns.duration'), sortable: true },
  { key: 'created_at', label: t('admin.adapterProviders.requests.columns.createdAt'), sortable: true },
  { key: 'error', label: t('admin.adapterProviders.requests.columns.error'), sortable: false },
])

const credentialsPlaceholder = computed(() => (
  editingProvider.value ? t('admin.adapterProviders.form.credentialsKeepPlaceholder') : credentialsExample
))

async function reload() {
  loading.value = true
  try {
    const [nextDiagnostics, providers, policies, requestResult, adapterUsageSummary] = await Promise.all([
      adminAPI.system.getAdapterProviderDiagnostics(),
      adminAPI.system.listAdapterProviders(),
      adminAPI.system.listRoutePolicies(),
      fetchAdapterRequestsWithTotal(),
      adminAPI.system.getAdapterUsageSummary(),
    ])
    diagnostics.value = nextDiagnostics
    providerRecords.value = providers
    routePolicies.value = policies
    adapterRequests.value = requestResult.requests
    adapterRequestsTotal.value = requestResult.total
    requestPage.value = 1
    requestPageInput.value = 1
    usageSummary.value = adapterUsageSummary
    adapterRequestsHasMore.value = requestResult.requests.length === adapterRequestPageSize
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.adapterProviders.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function loadAdapterRequests() {
  requestPage.value = 1
  requestPageInput.value = 1
  loadingRequests.value = true
  try {
    const requestResult = await fetchAdapterRequestsWithTotal(0)
    adapterRequests.value = requestResult.requests
    adapterRequestsTotal.value = requestResult.total
    adapterRequestsHasMore.value = requestResult.requests.length === adapterRequestPageSize
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.adapterProviders.requests.loadFailed')))
  } finally {
    loadingRequests.value = false
  }
}

async function loadMoreAdapterRequests() {
  if (loadingMoreRequests.value || !adapterRequestsHasMore.value) return
  loadingMoreRequests.value = true
  try {
    const requests = await fetchAdapterRequests(adapterRequests.value.length)
    adapterRequests.value = adapterRequests.value.concat(requests)
    adapterRequestsHasMore.value = requests.length === adapterRequestPageSize
    requestPage.value = Math.max(1, Math.ceil(adapterRequests.value.length / adapterRequestPageSize))
    requestPageInput.value = requestPage.value
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.adapterProviders.requests.loadFailed')))
  } finally {
    loadingMoreRequests.value = false
  }
}

async function goToAdapterRequestPage(page: number) {
  const normalizedPage = Math.min(Math.max(Math.trunc(Number(page) || 1), 1), adapterRequestsTotalPages.value)
  loadingRequests.value = true
  try {
    const requests = await fetchAdapterRequests((normalizedPage - 1) * adapterRequestPageSize)
    adapterRequests.value = requests
    adapterRequestsHasMore.value = normalizedPage < adapterRequestsTotalPages.value && requests.length === adapterRequestPageSize
    requestPage.value = normalizedPage
    requestPageInput.value = normalizedPage
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.adapterProviders.requests.loadFailed')))
  } finally {
    loadingRequests.value = false
  }
}

function fetchAdapterRequests(offset = 0) {
  return adminAPI.system.listAdapterRequests(adapterRequestFilters(offset))
}

async function fetchAdapterRequestsWithTotal(offset = 0) {
  const [requests, count] = await Promise.all([
    adminAPI.system.listAdapterRequests(adapterRequestFilters(offset)),
    adminAPI.system.countAdapterRequests(adapterRequestFilters()),
  ])
  return { requests, total: count.total }
}

function adapterRequestFilters(offset?: number) {
  const createdWindow = adapterRequestCreatedWindow()
  return {
    provider: stringOrUndefined(requestFilters.value.provider),
    status: stringOrUndefined(requestFilters.value.status),
    request_id: stringOrUndefined(requestFilters.value.request_id),
    focus: requestFocus.value === 'all' ? undefined : requestFocus.value,
    created_from: createdWindow.createdFrom,
    created_to: createdWindow.createdTo,
    offset,
    limit: offset === undefined ? undefined : adapterRequestPageSize,
  }
}

async function focusAdapterRequests(focus: 'failed' | 'stream' | 'websocket') {
  requestFocus.value = requestFocus.value === focus ? 'all' : focus
  await loadAdapterRequests()
}

function adapterRequestCreatedWindow() {
  const now = new Date()
  const window = requestFilters.value.window
  if (window === 'all') {
    return { createdFrom: undefined, createdTo: undefined }
  }
  const createdFrom = new Date(now)
  switch (window) {
    case '7d':
      createdFrom.setDate(createdFrom.getDate() - 7)
      break
    case '30d':
      createdFrom.setDate(createdFrom.getDate() - 30)
      break
    default:
      createdFrom.setHours(createdFrom.getHours() - 24)
      break
  }
  return {
    createdFrom: createdFrom.toISOString(),
    createdTo: now.toISOString(),
  }
}

function providerBySlug(slug: string) {
  return providerMap.value.get(slug)
}

function providerLabel(id?: number) {
  if (!id) return '-'
  const provider = providerRecords.value.find((item) => item.id === id)
  return provider ? `${provider.name} (${provider.slug})` : `#${id}`
}

function openCreateDialog() {
  editingProvider.value = null
  form.value = {
    name: '',
    slug: '',
    status: 'disabled',
    adapter_type: 'new-api',
    base_url: '',
    auth_mode: '',
    priority: 50,
    timeout_ms: 30000,
  }
  capabilitiesText.value = ''
  credentialsText.value = ''
  extraText.value = '{}'
  formOpen.value = true
}

function openEditDialog(row: AdapterProviderDiagnostic) {
  const provider = providerBySlug(row.slug)
  if (!provider) {
    appStore.showError(t('admin.adapterProviders.providerNotFound'))
    return
  }
  editingProvider.value = provider
  form.value = {
    name: provider.name,
    slug: provider.slug,
    status: provider.status || 'disabled',
    adapter_type: provider.adapter_type || 'new-api',
    base_url: provider.base_url,
    auth_mode: provider.auth_mode || '',
    priority: provider.priority,
    timeout_ms: provider.timeout_ms,
  }
  capabilitiesText.value = (provider.capabilities || []).join(', ')
  credentialsText.value = ''
  extraText.value = JSON.stringify(provider.extra || {}, null, 2)
  formOpen.value = true
}

function closeForm() {
  if (saving.value) return
  formOpen.value = false
  editingProvider.value = null
}

function askDelete(row: AdapterProviderDiagnostic) {
  deleteTarget.value = row
}

async function submitForm() {
  let payload: AdapterProviderPayload
  try {
    payload = buildPayload()
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : t('admin.adapterProviders.invalidJson'))
    return
  }

  saving.value = true
  try {
    if (editingProvider.value) {
      await adminAPI.system.updateAdapterProvider(editingProvider.value.id, payload)
      appStore.showSuccess(t('admin.adapterProviders.updated'))
    } else {
      await adminAPI.system.createAdapterProvider(payload)
      appStore.showSuccess(t('admin.adapterProviders.created'))
    }
    closeForm()
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.adapterProviders.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function deleteProvider() {
  if (!deleteTarget.value) return
  const provider = providerBySlug(deleteTarget.value.slug)
  if (!provider) {
    deleteTarget.value = null
    appStore.showError(t('admin.adapterProviders.providerNotFound'))
    return
  }
  try {
    await adminAPI.system.deleteAdapterProvider(provider.id)
    appStore.showSuccess(t('common.deleted'))
    deleteTarget.value = null
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.adapterProviders.deleteFailed')))
  }
}

function openCreatePolicyDialog() {
  editingPolicy.value = null
  policyForm.value = {
    name: '',
    status: 'disabled',
    match_method: '',
    match_path: '',
    match_model: '',
    match_capability: '',
    match_group_platform: '',
    target: 'new_api_adapter',
    adapter_provider_id: undefined,
    priority: 50,
    description: '',
  }
  policyConditionsText.value = '{}'
  policyFormOpen.value = true
}

function openEditPolicyDialog(policy: RoutePolicy) {
  editingPolicy.value = policy
  policyForm.value = {
    name: policy.name,
    status: policy.status || 'disabled',
    match_method: policy.match_method || '',
    match_path: policy.match_path || '',
    match_model: policy.match_model || '',
    match_capability: policy.match_capability || '',
    match_group_platform: policy.match_group_platform || '',
    target: policy.target || 'new_api_adapter',
    platform: policy.platform || '',
    adapter_provider_id: policy.adapter_provider_id,
    priority: policy.priority,
    description: policy.description || '',
  }
  policyConditionsText.value = JSON.stringify(policy.conditions || {}, null, 2)
  policyFormOpen.value = true
}

function closePolicyForm() {
  if (savingPolicy.value) return
  policyFormOpen.value = false
  editingPolicy.value = null
}

async function submitPolicyForm() {
  let payload: RoutePolicyPayload
  try {
    payload = buildPolicyPayload()
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : t('admin.adapterProviders.invalidJson'))
    return
  }

  savingPolicy.value = true
  try {
    if (editingPolicy.value) {
      await adminAPI.system.updateRoutePolicy(editingPolicy.value.id, payload)
      appStore.showSuccess(t('admin.adapterProviders.routePolicies.updated'))
    } else {
      await adminAPI.system.createRoutePolicy(payload)
      appStore.showSuccess(t('admin.adapterProviders.routePolicies.created'))
    }
    closePolicyForm()
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.adapterProviders.routePolicies.saveFailed')))
  } finally {
    savingPolicy.value = false
  }
}

async function deletePolicy() {
  if (!policyDeleteTarget.value) return
  try {
    await adminAPI.system.deleteRoutePolicy(policyDeleteTarget.value.id)
    appStore.showSuccess(t('common.deleted'))
    policyDeleteTarget.value = null
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.adapterProviders.routePolicies.deleteFailed')))
  }
}

function buildPayload(): AdapterProviderPayload {
  const capabilities = capabilitiesText.value
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
  if (!capabilities.length) {
    throw new Error(t('admin.adapterProviders.form.capabilitiesRequired'))
  }

  const payload: AdapterProviderPayload = {
    name: form.value.name.trim(),
    slug: form.value.slug.trim(),
    status: form.value.status,
    adapter_type: form.value.adapter_type.trim() || 'new-api',
    base_url: form.value.base_url.trim(),
    auth_mode: form.value.auth_mode.trim() || undefined,
    capabilities,
    priority: Number(form.value.priority) || 0,
    timeout_ms: Number(form.value.timeout_ms) || 0,
    extra: parseObjectJSON(extraText.value, 'extra'),
  }

  const credentialsRaw = credentialsText.value.trim()
  if (credentialsRaw) {
    payload.credentials = parseStringMapJSON(credentialsRaw, 'credentials')
  }
  return payload
}

function buildPolicyPayload(): RoutePolicyPayload {
  const adapterProviderID = Number(policyForm.value.adapter_provider_id) || undefined
  return {
    name: String(policyForm.value.name || '').trim(),
    status: String(policyForm.value.status || 'disabled'),
    match_method: stringOrUndefined(policyForm.value.match_method),
    match_path: stringOrUndefined(policyForm.value.match_path),
    match_model: stringOrUndefined(policyForm.value.match_model),
    match_capability: stringOrUndefined(policyForm.value.match_capability),
    match_group_platform: stringOrUndefined(policyForm.value.match_group_platform),
    target: String(policyForm.value.target || 'new_api_adapter'),
    platform: stringOrUndefined(policyForm.value.platform),
    adapter_provider_id: adapterProviderID,
    priority: Number(policyForm.value.priority) || 0,
    conditions: parseObjectJSON(policyConditionsText.value, 'conditions'),
    description: stringOrUndefined(policyForm.value.description),
  }
}

function stringOrUndefined(value: unknown): string | undefined {
  const text = String(value || '').trim()
  return text || undefined
}

function parseObjectJSON(raw: string, label: string): Record<string, unknown> {
  const trimmed = raw.trim()
  if (!trimmed) return {}
  const parsed = JSON.parse(trimmed)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(t('admin.adapterProviders.form.objectJsonRequired', { field: label }))
  }
  return parsed as Record<string, unknown>
}

function parseStringMapJSON(raw: string, label: string): Record<string, string> {
  const parsed = parseObjectJSON(raw, label)
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(parsed)) {
    out[key] = String(value)
  }
  return out
}

function statusBadgeClass(row: AdapterProviderDiagnostic) {
  if (!row.valid) {
    return 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
  }
  if (row.enabled) {
    return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
  }
  return 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-gray-300'
}

function statusLabel(row: AdapterProviderDiagnostic) {
  if (!row.valid) return t('admin.adapterProviders.invalid')
  if (row.enabled) return t('admin.adapterProviders.enabled')
  return t('admin.adapterProviders.disabled')
}

function reasonLabel(reason?: string) {
  if (!reason) return '-'
  if (reason === 'enabled') return t('admin.adapterProviders.reasonEnabled')
  if (reason === 'provider_disabled') return t('admin.adapterProviders.reasonDisabled')
  return reason
}

function formatTimeout(timeoutMS: number) {
  if (!timeoutMS) return '-'
  return `${timeoutMS} ms`
}

function requestStatusClass(row: AdapterRequestRecord) {
  if (requestFailed(row)) {
    return 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
  }
  return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
}

function requestStatusLabel(row: AdapterRequestRecord) {
  return requestFailed(row)
    ? t('admin.adapterProviders.requests.failed')
    : t('admin.adapterProviders.requests.success')
}

function requestSignals(row: AdapterRequestRecord) {
  const metadata = row.metadata || {}
  const signals: Array<{ label: string; detail?: string; class: string }> = []
  const usageSource = metadataString(metadata.usage_source)

  if (metadata.stream_usage_finalized === true) {
    signals.push({
      label: t('admin.adapterProviders.requests.markers.streamFinalized'),
      detail: usageSource,
      class: 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-300',
    })
  }

  if (metadata.websocket_usage_finalized === true) {
    signals.push({
      label: t('admin.adapterProviders.requests.markers.websocketFinalized'),
      detail: usageSource,
      class: 'bg-indigo-50 text-indigo-700 dark:bg-indigo-900/20 dark:text-indigo-300',
    })
  } else if (metadata.websocket === true || metadata.transport === 'websocket') {
    signals.push({
      label: t('admin.adapterProviders.requests.markers.websocketTunnel'),
      detail: usageSource || metadataString(metadata.transport),
      class: 'bg-slate-100 text-slate-700 dark:bg-dark-700 dark:text-gray-200',
    })
  }

  return signals
}

function metadataString(value: unknown) {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function requestFailed(row: AdapterRequestRecord) {
  return Boolean(row.error_message) || (typeof row.status_code === 'number' && row.status_code >= 400)
}

function formatCost(value: unknown) {
  if (value === null || value === undefined || value === '') return '-'
  const amount = Number(value)
  if (!Number.isFinite(amount)) return '-'
  return `$${amount.toFixed(6)}`
}

function billingState(row: AdapterRequestRecord) {
  const metadata = row.metadata || {}
  if (metadata.billing_applied === true) {
    return t('admin.adapterProviders.requests.billingApplied')
  }
  if (typeof metadata.billing_error === 'string' && metadata.billing_error) {
    return t('admin.adapterProviders.requests.billingError')
  }
  if (typeof metadata.billing_skipped_reason === 'string' && metadata.billing_skipped_reason) {
    return metadata.billing_skipped_reason
  }
  return t('admin.adapterProviders.requests.billingSkipped')
}

function formatDuration(ms?: number) {
  if (typeof ms !== 'number') return '-'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`
  return `${Math.round(ms)} ms`
}

onMounted(reload)
</script>
