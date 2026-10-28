/**
 * The Tauri-compatible filesystem API.
 *
 * The frontend reads a few files — the contents of a magnet link, the
 * text of a log — and asks the backend for them, because the webview
 * has no access to the disk.
 */
import { invoke } from './core';

/** Reads a file as text. */
export async function readTextFile(path: string): Promise<string> {
  return invoke<string>('read_local_file', { path });
}

/** Reads a file as bytes. */
export async function readFile(path: string): Promise<Uint8Array> {
  const content = await invoke<string>('read_local_file', { path });
  return new TextEncoder().encode(content);
}

/** Answers whether a path exists. */
export async function exists(path: string): Promise<boolean> {
  return invoke<boolean>('check_path_exists', { path });
}

/** Writes a file. */
export async function writeTextFile(path: string, contents: string): Promise<void> {
  await invoke('write_local_file', { path, contents });
}
