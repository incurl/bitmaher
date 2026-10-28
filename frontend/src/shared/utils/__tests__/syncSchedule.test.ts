import { describe, expect, it } from 'vitest'
import { checkSyncDue } from '../syncSchedule'

const now = Date.UTC(2026, 5, 2, 12, 0, 0)

describe('checkSyncDue', () => {
  it('does not run when automatic sync is disabled', () => {
    expect(checkSyncDue({ enabled: false, intervalHours: 0, lastSyncTime: 0, now, startup: true })).toBe(false)
  })

  it('runs every-startup jobs only during startup checks', () => {
    expect(checkSyncDue({ enabled: true, intervalHours: 0, lastSyncTime: now, now, startup: true })).toBe(true)
    expect(checkSyncDue({ enabled: true, intervalHours: 0, lastSyncTime: now, now, startup: false })).toBe(false)
  })

  it('runs interval jobs when their elapsed time has passed', () => {
    expect(
      checkSyncDue({
        enabled: true,
        intervalHours: 12,
        lastSyncTime: now - 12 * 60 * 60 * 1000,
        now,
        startup: false,
      }),
    ).toBe(true)
    expect(
      checkSyncDue({
        enabled: true,
        intervalHours: 12,
        lastSyncTime: now - 11 * 60 * 60 * 1000,
        now,
        startup: false,
      }),
    ).toBe(false)
  })
})
