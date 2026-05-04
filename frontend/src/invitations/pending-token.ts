const pendingTenantInvitationTokenKey = 'haohao.pendingTenantInvitationToken'

export function savePendingTenantInvitationToken(token: string) {
  const value = token.trim()
  if (!value) {
    return
  }
  try {
    window.sessionStorage.setItem(pendingTenantInvitationTokenKey, value)
  } catch {
    // Ignore storage failures; the visible accept URL remains the fallback.
  }
}

export function readPendingTenantInvitationToken(): string {
  try {
    return window.sessionStorage.getItem(pendingTenantInvitationTokenKey)?.trim() ?? ''
  } catch {
    return ''
  }
}

export function clearPendingTenantInvitationToken() {
  try {
    window.sessionStorage.removeItem(pendingTenantInvitationTokenKey)
  } catch {
    // Ignore storage failures.
  }
}
