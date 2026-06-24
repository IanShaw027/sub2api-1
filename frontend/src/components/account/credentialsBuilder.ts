export function applyInterceptWarmup(
  credentials: Record<string, unknown>,
  enabled: boolean,
  mode: 'create' | 'edit',
  currentCredentials?: Record<string, unknown>
): void {
  if (enabled) {
    credentials.intercept_warmup_requests = true
  } else if (mode === 'edit') {
    if (currentCredentials?.intercept_warmup_requests === true) {
      credentials.intercept_warmup_requests = false
    } else {
      delete credentials.intercept_warmup_requests
    }
  }
}
