/**
 * The Tauri-compatible application API.
 *
 * The version and the identifier come from the backend, which is the
 * one that knows them.
 */
import { invoke } from './core';

/** The version of the application. */
export async function getVersion(): Promise<string> {
  return invoke<string>('app_version', {});
}

/** The name of the application. */
export async function getName(): Promise<string> {
  return invoke<string>('app_name', {});
}

/** The identifier of the application. */
export async function getTauriVersion(): Promise<string> {
  return invoke<string>('app_version', {});
}
