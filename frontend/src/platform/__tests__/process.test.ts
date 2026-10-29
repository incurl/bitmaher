/**
 * @fileoverview The process shim: the end of the application.
 *
 * The quit used to be a command that no backend answered, so the
 * application never ended: the request fell through to the catch-all
 * of the command map and was only written to the log.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const bridge = vi.hoisted(() => ({ quit: vi.fn(), invoke: vi.fn() }))
vi.mock('@wailsio/runtime', () => ({ Application: { Quit: bridge.quit } }))
vi.mock('@tauri-apps/api/core', () => ({ invoke: bridge.invoke }))

import { exit, relaunch } from '../process'

beforeEach(() => {
  vi.clearAllMocks()
  bridge.quit.mockResolvedValue(undefined)
  bridge.invoke.mockResolvedValue(undefined)
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('ending the application', () => {
  it('quits through the Wails runtime', async () => {
    await exit(0)
    expect(bridge.quit).toHaveBeenCalledTimes(1)
  })

  it('does not ask the backend to quit', async () => {
    await exit(0)
    expect(bridge.invoke).not.toHaveBeenCalled()
  })

  it('quits whatever the exit code is', async () => {
    await exit(3)
    expect(bridge.quit).toHaveBeenCalledTimes(1)
  })
})

describe('restarting the application', () => {
  it('asks the backend, which owns the sidecar', async () => {
    await relaunch()
    expect(bridge.invoke).toHaveBeenCalledWith('app_relaunch', {})
    expect(bridge.quit).not.toHaveBeenCalled()
  })
})
