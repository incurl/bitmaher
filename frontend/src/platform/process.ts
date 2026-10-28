/**
 * The Tauri-compatible process API.
 *
 * BitMaher restarts itself through the backend, which owns the
 * sidecar and has to bring it up again.
 */
import { invoke } from './core';

/** Restarts the application. */
export async function relaunch(): Promise<void> {
  await invoke('app_relaunch', {});
}

/** Ends the application. */
export async function exit(code = 0): Promise<void> {
  await invoke('app_exit', { code });
}
