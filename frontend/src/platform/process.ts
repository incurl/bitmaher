/**
 * The Tauri-compatible process API.
 *
 * The end of the application is the Wails runtime's own quit: the
 * runtime tears the webview down and the backend stops the sidecar.
 *
 * The restart is the backend's business, because the backend owns the
 * sidecar and has to bring it up again: the engine is a child process,
 * and a child process cannot survive the parent that starts it again.
 */
import { Application } from '@wailsio/runtime';
import { invoke } from './core';

/**
 * Restarts the application.
 *
 * The backend starts the new instance as this one goes down. The
 * request never gets an answer, because the webview is gone by the
 * time it could.
 */
export async function relaunch(): Promise<void> {
  await invoke('app_relaunch', {});
}

/**
 * Ends the application.
 *
 * The Wails runtime's quit is the one that ends the process: it runs
 * the shutdown of the backend, which stops the sidecar, and then it
 * closes the window and leaves.
 *
 * The promise is left hanging on purpose: the webview is torn down
 * before the answer could arrive.
 */
export async function exit(code = 0): Promise<void> {
  // The code is the one the process ends with. The desktop platforms
  // only know a failure from a success, so it is not worth a call of
  // its own: the backend records it when the shutdown runs.
  void code;
  await Application.Quit();
}
