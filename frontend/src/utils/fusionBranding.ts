export const DEFAULT_FUSION_SITE_NAME = 'API Fusion'
export const UPSTREAM_DEFAULT_SITE_NAME = 'Sub2API'
export const UPSTREAM_DEFAULT_SITE_SUBTITLE = 'Subscription to API Conversion Platform'

export function resolveFusionSiteName(value: string | undefined | null): string {
  const trimmed = value?.trim()
  if (!trimmed || trimmed === UPSTREAM_DEFAULT_SITE_NAME) {
    return DEFAULT_FUSION_SITE_NAME
  }
  return trimmed
}

export function isUpstreamDefaultSiteSubtitle(value: string | undefined | null): boolean {
  return value?.trim() === UPSTREAM_DEFAULT_SITE_SUBTITLE
}
