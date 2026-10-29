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

describe('the commands of the features BitMaher does not have', () => {
  it('answers the questions about the autostart with a no', () => {
    // The runtime answers a method that returns nothing with an empty
    // object, and an empty object is true in JavaScript. The frontend
    // read the answer of the question about the autostart as a yes,
    // took the silent branch, and hid the window: it flashed and
    // disappeared at start-up.
    for (const command of [
      'is_autostart_launch',
      'peek_pending_deep_links_silent',
      'peek_pending_external_inputs_silent',
      'float_window_needs_relaunch',
    ]) {
      expect(resolveCommand(command, {})).toEqual({
        method: APP_SERVICE + '.NeutralBool',
        args: [],
      })
    }
  })

  it('answers the questions about a value with nothing', () => {
    for (const command of [
      'check_for_update',
      'fetch_tracker_sources',
      'get_ed2k_bootstrap_status',
      'get_system_proxy',
      'take_pending_deep_links',
    ]) {
      expect(resolveCommand(command, {})).toEqual({
        method: APP_SERVICE + '.NeutralValue',
        args: [],
      })
    }
  })

  it('reports the commands it does not know', () => {
    expect(resolveCommand('a_command_that_does_not_exist', {})).toEqual({
      method: APP_SERVICE + '.UnsupportedCommand',
      args: ['a_command_that_does_not_exist'],
    })
  })
})
