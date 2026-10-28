export interface SyncDueInput {
  enabled: boolean
  intervalHours: number
  lastSyncTime: number
  now: number
  startup: boolean
}

export function checkSyncDue(input: SyncDueInput): boolean {
  if (!input.enabled) return false
  if (input.intervalHours <= 0) return input.startup
  const lastSyncTime = Number(input.lastSyncTime) || 0
  if (lastSyncTime <= 0) return true
  return input.now - lastSyncTime >= input.intervalHours * 60 * 60 * 1000
}
