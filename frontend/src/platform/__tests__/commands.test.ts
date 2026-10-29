/**
 * @fileoverview The command map: the lifecycle commands.
 *
 * The commands of the end of the application used to fall through to
 * the catch-all of the map, which only wrote them to the log. The
 * application could therefore not be ended from the interface.
 */
import { describe, expect, it } from 'vitest'
import { resolveCommand } from '../commands'

const APP_SERVICE = 'github.com/incurl/bitmaher/internal/bindings.AppService'

describe('the lifecycle commands', () => {
  it('ends the application through the backend', () => {
    expect(resolveCommand('app_exit', { code: 0 })).toEqual({
      method: APP_SERVICE + '.AppExit',
      args: [0],
    })
  })

  it('keeps the exit code the caller asked for', () => {
    expect(resolveCommand('app_exit', { code: 3 }).args).toEqual([3])
  })

  it('ends with no code when the caller did not give one', () => {
    expect(resolveCommand('app_exit', {}).args).toEqual([0])
  })

  it('restarts the application through the backend', () => {
    expect(resolveCommand('app_relaunch', {})).toEqual({
      method: APP_SERVICE + '.AppRelaunch',
      args: [],
    })
  })

  it('releases the close of the window', () => {
    expect(resolveCommand('release_window_close', {})).toEqual({
      method: APP_SERVICE + '.ReleaseWindowClose',
      args: [],
    })
  })

  it('sends the window to the tray', () => {
    expect(resolveCommand('minimize_window_to_tray', {})).toEqual({
      method: APP_SERVICE + '.MinimiseWindowToTray',
      args: [],
    })
  })

  it('never falls through to the catch-all', () => {
    for (const command of [
      'app_exit',
      'app_relaunch',
      'release_window_close',
      'minimize_window_to_tray',
    ]) {
      expect(resolveCommand(command, {}).method).not.toContain('UnsupportedCommand')
    }
  })
})
