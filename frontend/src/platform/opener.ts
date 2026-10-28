/**
 * The Tauri-compatible opener API.
 *
 * Opening a URL or a file is a job of the operating system, which only
 * the backend can ask for.
 */
import { invoke } from './core';

/** Opens a URL in the default browser. */
export async function openUrl(url: string): Promise<void> {
  await invoke('open_external_url', { url });
}

/** Opens a file with the program the operating system chooses. */
export async function openPath(path: string): Promise<void> {
  await invoke('open_path_normalized', { path });
}
