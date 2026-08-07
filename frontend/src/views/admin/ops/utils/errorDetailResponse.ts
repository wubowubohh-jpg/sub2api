import type { OpsErrorDetail } from '@/api/admin/ops'

export function resolveUpstreamPayload(
  detail: Pick<OpsErrorDetail, 'upstream_error_detail' | 'upstream_errors' | 'upstream_error_message'> | null | undefined
): string {
  if (!detail) return ''

  const candidates = [
    detail.upstream_error_detail,
    detail.upstream_errors,
    detail.upstream_error_message
  ]

  for (const candidate of candidates) {
    const payload = String(candidate || '').trim()
    if (!payload) continue

    // Normalize common "empty but present" JSON placeholders.
    if (payload === '[]' || payload === '{}' || payload.toLowerCase() === 'null') {
      continue
    }

    return payload
  }

  return ''
}

export function resolvePrimaryResponseBody(
  detail: OpsErrorDetail | null,
  errorType?: 'request' | 'upstream'
): string {
  if (!detail) return ''

  const errorBody = String(detail.error_body || '').trim()

  if (errorType === 'upstream') {
    const upstreamPayload = resolveUpstreamPayload(detail)
    return upstreamPayload || errorBody
  }

  // Request details must reflect the actual client-visible response. Raw
  // provider diagnostics remain available in the dedicated upstream section.
  return errorBody
}
