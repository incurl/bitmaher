/**
 * @fileoverview The window shim: the window of the application.
 *
 * The window used to be a set of events that nothing in the backend
 * listened for: the calls went nowhere, and the caption buttons did
 * nothing at all. They are now the window of the Wails runtime.
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const bridge = vi.hoisted(() => ({
  close: vi.fn(),
  focus: vi.fn(),
  hide: vi.fn(),
  isMaximised: vi.fn(),
  maximise: vi.fn(),
  minimise: vi.fn(),
  show: vi.fn(),
  toggleMaximise: vi.fn(),
  unMaximise: vi.fn(),
  unMinimise: vi.fn(),
  invoke: vi.fn(),
  emit: vi.fn(),
}))

vi.mock('@wailsio/runtime', () => ({
  Window: {
    Close: bridge.close,
    Focus: bridge.focus,
    Hide: bridge.hide,
    IsMaximised: bridge.isMaximised,
    Maximise: bridge.maximise,
    Minimise: bridge.minimise,
    Show: bridge.show,
    ToggleMaximise: bridge.toggleMaximise,
    UnMaximise: bridge.unMaximise,
    UnMinimise: bridge.unMinimise,
  },
  Events: { Emit: bridge.emit },
}))
vi.mock('@tauri-apps/api/core', () => ({ invoke: bridge.invoke }))

import { getCurrentWindow } from '../window'

beforeEach(() => {
  vi.clearAllMocks()
  for (const fn of Object.values(bridge)) {
    if (typeof fn === 'function') fn.mockResolvedValue(undefined)
  }
  bridge.isMaximised.mockResolvedValue(false)
})

describe('the window of the application', () => {
  it('hides the window through the runtime', async () => {
    await getCurrentWindow().hide()
    expect(bridge.hide).toHaveBeenCalledTimes(1)
    expect(bridge.emit).not.toHaveBeenCalled()
  })

  it('shows the window through the runtime', async () => {
    await getCurrentWindow().show()
    expect(bridge.show).toHaveBeenCalledTimes(1)
    expect(bridge.emit).not.toHaveBeenCalled()
  })

  it('minimises the window through the runtime', async () => {
    await getCurrentWindow().minimize()
    expect(bridge.minimise).toHaveBeenCalledTimes(1)
    expect(bridge.emit).not.toHaveBeenCalled()
  })

  it('restores the window from the minimised state', async () => {
    await getCurrentWindow().unminimize()
    expect(bridge.unMinimise).toHaveBeenCalledTimes(1)
  })

  it('maximises the window through the runtime', async () => {
    await getCurrentWindow().maximize()
    expect(bridge.maximise).toHaveBeenCalledTimes(1)
    expect(bridge.emit).not.toHaveBeenCalled()
  })

  it('unmaximises the window through the runtime', async () => {
    await getCurrentWindow().unmaximize()
    expect(bridge.unMaximise).toHaveBeenCalledTimes(1)
  })

  it('toggles the maximised state through the runtime', async () => {
    await getCurrentWindow().toggleMaximize()
    expect(bridge.toggleMaximise).toHaveBeenCalledTimes(1)
    expect(bridge.emit).not.toHaveBeenCalled()
  })

  it('closes the window through the runtime', async () => {
    await getCurrentWindow().close()
    expect(bridge.close).toHaveBeenCalledTimes(1)
    expect(bridge.emit).not.toHaveBeenCalled()
  })

  it('destroys the window by closing it', async () => {
    await getCurrentWindow().destroy()
    expect(bridge.close).toHaveBeenCalledTimes(1)
  })

  it('focuses the window through the runtime', async () => {
    await getCurrentWindow().setFocus()
    expect(bridge.focus).toHaveBeenCalledTimes(1)
    expect(bridge.emit).not.toHaveBeenCalled()
  })

  it('answers whether the window is maximised', async () => {
    bridge.isMaximised.mockResolvedValue(true)
    expect(await getCurrentWindow().isMaximized()).toBe(true)
    expect(bridge.isMaximised).toHaveBeenCalledTimes(1)
  })

  it('never emits an event of its own', async () => {
    const view = getCurrentWindow()
    await view.hide()
    await view.show()
    await view.minimize()
    await view.maximize()
    await view.unmaximize()
    await view.toggleMaximize()
    await view.close()
    await view.destroy()
    await view.setFocus()
    expect(bridge.emit).not.toHaveBeenCalled()
  })

  it('leaves the frame to the backend', async () => {
    await getCurrentWindow().setDecorations(true)
    expect(bridge.invoke).toHaveBeenCalledWith('set_window_decorations', { visible: true })
  })
})
