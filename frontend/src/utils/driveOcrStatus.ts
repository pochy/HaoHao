export type DriveOcrActionStatus = 'idle' | 'requesting' | 'queued' | 'polling' | 'succeeded' | 'failed'
export type DriveOcrTone = 'neutral' | 'success' | 'warning' | 'danger'

const activeStatuses = new Set(['pending', 'running'])
const terminalStatuses = new Set(['completed', 'failed', 'skipped'])

export function isDriveOcrActiveStatus(status?: string | null) {
  return Boolean(status && activeStatuses.has(status))
}

export function isDriveOcrTerminalStatus(status?: string | null) {
  return Boolean(status && terminalStatuses.has(status))
}

export function driveOcrActionStatusFromRunStatus(status?: string | null): DriveOcrActionStatus {
  switch (status) {
    case 'pending':
      return 'queued'
    case 'running':
      return 'polling'
    case 'completed':
      return 'succeeded'
    case 'failed':
    case 'skipped':
      return 'failed'
    default:
      return 'idle'
  }
}

export function driveOcrToneFromStatus(status?: string | null): DriveOcrTone {
  switch (status) {
    case 'completed':
      return 'success'
    case 'pending':
    case 'running':
    case 'skipped':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'neutral'
  }
}
