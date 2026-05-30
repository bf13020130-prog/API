/**
 * System API endpoints for admin operations
 */

import { apiClient } from '../client'

export interface ReleaseInfo {
  name: string
  body: string
  published_at: string
  html_url: string
}

export interface VersionInfo {
  current_version: string
  latest_version: string
  has_update: boolean
  release_info?: ReleaseInfo
  cached: boolean
  warning?: string
  build_type: string // "source" for manual builds, "release" for CI builds
}

export interface AdapterProviderDiagnostic {
  name: string
  slug: string
  status: string
  adapter_type: string
  base_url: string
  auth_mode?: string
  capabilities: string[]
  priority: number
  timeout_ms: number
  valid: boolean
  enabled: boolean
  reason?: string
}

export interface AdapterProviderDiagnostics {
  observe_only: boolean
  enforcement_enabled: boolean
  active_slugs: string[]
  providers: AdapterProviderDiagnostic[]
}

export interface AdapterProvider {
  id: number
  name: string
  slug: string
  status: string
  adapter_type: string
  base_url: string
  auth_mode?: string
  capabilities: string[]
  priority: number
  timeout_ms: number
  extra?: Record<string, unknown>
  has_credentials: boolean
  credential_keys: string[]
  created_at: string
  updated_at: string
}

export interface AdapterProviderPayload {
  name: string
  slug: string
  status?: string
  adapter_type?: string
  base_url: string
  auth_mode?: string
  credentials?: Record<string, string>
  capabilities: string[]
  priority?: number
  timeout_ms?: number
  extra?: Record<string, unknown>
}

export interface RoutePolicy {
  id: number
  name: string
  status: string
  match_method?: string
  match_path?: string
  match_model?: string
  match_capability?: string
  match_group_platform?: string
  target: string
  platform?: string
  adapter_provider_id?: number
  priority: number
  conditions?: Record<string, unknown>
  description?: string
  created_at: string
  updated_at: string
}

export interface RoutePolicyPayload {
  name: string
  status?: string
  match_method?: string
  match_path?: string
  match_model?: string
  match_capability?: string
  match_group_platform?: string
  target: string
  platform?: string
  adapter_provider_id?: number
  priority?: number
  conditions?: Record<string, unknown>
  description?: string
}

export interface AdapterRequestRecord {
  id: number
  request_id: string
  user_id: number
  api_key_id: number
  group_id?: number
  adapter_provider_id: number
  provider: string
  capability: string
  route_target: string
  method: string
  path: string
  model?: string
  status_code?: number
  duration_ms?: number
  error_message?: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface AdapterRequestFilters {
  provider?: string
  request_id?: string
  status?: string
  focus?: 'failed' | 'stream' | 'websocket' | string
  created_from?: string
  created_to?: string
  limit?: number
  offset?: number
}

export interface AdapterRequestCount {
  total: number
}

export interface AdapterUsageRecord {
  id: number
  request_id: string
  user_id: number
  api_key_id: number
  group_id?: number
  adapter_provider_id: number
  route_policy_id?: number
  provider: string
  capability: string
  model?: string
  method: string
  path: string
  status: string
  status_code?: number
  duration_ms?: number
  error_message?: string
  input_units: number
  output_units: number
  billable_units: number
  cost_usd: number
  billable_unit: number
  billing_applied: boolean
  billing_fingerprint?: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface AdapterUsageFilters {
  provider?: string
  request_id?: string
  status?: string
  limit?: number
}

export interface AdapterUsageProviderSummary {
  provider: string
  total_requests: number
  success_requests: number
  failed_requests: number
  input_units: number
  output_units: number
  billable_units: number
  cost_usd: number
}

export interface AdapterUsageSummary {
  total_requests: number
  success_requests: number
  failed_requests: number
  input_units: number
  output_units: number
  billable_units: number
  cost_usd: number
  providers: AdapterUsageProviderSummary[]
}

/**
 * Get current version
 */
export async function getVersion(): Promise<{ version: string }> {
  const { data } = await apiClient.get<{ version: string }>('/admin/system/version')
  return data
}

/**
 * Check for updates
 * @param force - Force refresh from GitHub API
 */
export async function checkUpdates(force = false): Promise<VersionInfo> {
  const { data } = await apiClient.get<VersionInfo>('/admin/system/check-updates', {
    params: force ? { force: 'true' } : undefined
  })
  return data
}

/**
 * Get read-only diagnostics for long-tail adapter providers.
 */
export async function getAdapterProviderDiagnostics(): Promise<AdapterProviderDiagnostics> {
  const { data } = await apiClient.get<AdapterProviderDiagnostics>('/admin/system/adapter-providers/diagnostics')
  return data
}

/**
 * List DB-backed long-tail adapter providers without credential values.
 */
export async function listAdapterProviders(): Promise<AdapterProvider[]> {
  const { data } = await apiClient.get<AdapterProvider[]>('/admin/system/adapter-providers')
  return data
}

/**
 * Create a long-tail adapter provider.
 */
export async function createAdapterProvider(payload: AdapterProviderPayload): Promise<AdapterProvider> {
  const { data } = await apiClient.post<AdapterProvider>('/admin/system/adapter-providers', payload)
  return data
}

/**
 * Update a long-tail adapter provider. Omit credentials to keep existing values.
 */
export async function updateAdapterProvider(id: number, payload: AdapterProviderPayload): Promise<AdapterProvider> {
  const { data } = await apiClient.put<AdapterProvider>(`/admin/system/adapter-providers/${id}`, payload)
  return data
}

/**
 * Delete a long-tail adapter provider.
 */
export async function deleteAdapterProvider(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/system/adapter-providers/${id}`)
  return data
}

/**
 * List DB-backed route policies.
 */
export async function listRoutePolicies(): Promise<RoutePolicy[]> {
  const { data } = await apiClient.get<RoutePolicy[]>('/admin/system/route-policies')
  return data
}

/**
 * Create a route policy.
 */
export async function createRoutePolicy(payload: RoutePolicyPayload): Promise<RoutePolicy> {
  const { data } = await apiClient.post<RoutePolicy>('/admin/system/route-policies', payload)
  return data
}

/**
 * Update a route policy.
 */
export async function updateRoutePolicy(id: number, payload: RoutePolicyPayload): Promise<RoutePolicy> {
  const { data } = await apiClient.put<RoutePolicy>(`/admin/system/route-policies/${id}`, payload)
  return data
}

/**
 * Delete a route policy.
 */
export async function deleteRoutePolicy(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/system/route-policies/${id}`)
  return data
}

/**
 * List adapter execution audit records.
 */
export async function listAdapterRequests(filters: AdapterRequestFilters = {}): Promise<AdapterRequestRecord[]> {
  const { data } = await apiClient.get<AdapterRequestRecord[]>('/admin/system/adapter-requests', {
    params: filters
  })
  return data
}

/**
 * Count adapter execution audit records for the same filters as listAdapterRequests.
 */
export async function countAdapterRequests(filters: AdapterRequestFilters = {}): Promise<AdapterRequestCount> {
  const { data } = await apiClient.get<AdapterRequestCount>('/admin/system/adapter-requests/count', {
    params: {
      ...filters,
      limit: undefined,
      offset: undefined
    }
  })
  return data
}

/**
 * List adapter usage analytics records.
 */
export async function listAdapterUsages(filters: AdapterUsageFilters = {}): Promise<AdapterUsageRecord[]> {
  const { data } = await apiClient.get<AdapterUsageRecord[]>('/admin/system/adapter-usages', {
    params: filters
  })
  return data
}

/**
 * Get adapter usage analytics summary.
 */
export async function getAdapterUsageSummary(filters: AdapterUsageFilters = {}): Promise<AdapterUsageSummary> {
  const { data } = await apiClient.get<AdapterUsageSummary>('/admin/system/adapter-usages/summary', {
    params: filters
  })
  return data
}

export interface UpdateResult {
  message: string
  need_restart: boolean
}

/**
 * Perform system update
 * Downloads and applies the latest version
 */
export async function performUpdate(): Promise<UpdateResult> {
  const { data } = await apiClient.post<UpdateResult>('/admin/system/update')
  return data
}

/**
 * Rollback to previous version
 */
export async function rollback(): Promise<UpdateResult> {
  const { data } = await apiClient.post<UpdateResult>('/admin/system/rollback')
  return data
}

/**
 * Restart the service
 */
export async function restartService(): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/admin/system/restart')
  return data
}

export const systemAPI = {
  getVersion,
  checkUpdates,
  getAdapterProviderDiagnostics,
  listAdapterProviders,
  createAdapterProvider,
  updateAdapterProvider,
  deleteAdapterProvider,
  listRoutePolicies,
  createRoutePolicy,
  updateRoutePolicy,
  deleteRoutePolicy,
  listAdapterRequests,
  countAdapterRequests,
  listAdapterUsages,
  getAdapterUsageSummary,
  performUpdate,
  rollback,
  restartService
}

export default systemAPI
