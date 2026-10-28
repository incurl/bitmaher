/**
 * The Tauri-compatible path API.
 *
 * The paths are resolved by the backend, which is the only process that
 * knows where the data directory is. The frontend asks for the ones it
 * needs and remembers them.
 */
import { invoke } from './core';

/** The paths of the application. */
interface Paths {
  appData: string;
  appLog: string;
  download: string;
  home: string;
  temp: string;
}

let paths: Paths | null = null;

/** Reads the paths of the application. */
async function load(): Promise<Paths> {
  if (paths) return paths;
  const result = await invoke<Partial<Paths>>('platform_paths', {});
  paths = {
    appData: result.appData ?? '',
    appLog: result.appLog ?? '',
    download: result.download ?? '',
    home: result.home ?? '',
    temp: result.temp ?? '',
  };
  return paths;
}

/** The directory the application keeps its data in. */
export async function appDataDir(): Promise<string> {
  return (await load()).appData;
}

/** The directory the application writes its logs in. */
export async function appLogDir(): Promise<string> {
  return (await load()).appLog;
}

/** The directory the downloads go to by default. */
export async function downloadDir(): Promise<string> {
  return (await load()).download;
}

/** The home directory of the user. */
export async function homeDir(): Promise<string> {
  return (await load()).home;
}

/** The temporary directory of the system. */
export async function tempDir(): Promise<string> {
  return (await load()).temp;
}

/** The last part of a path. */
export function basename(path: string, ext?: string): string {
  const name = path.split(/[\\/]/).pop() ?? path;
  if (ext && name.endsWith(ext)) return name.slice(0, -ext.length);
  return name;
}

/** The directory of a path. */
export function dirname(path: string): string {
  const parts = path.split(/[\\/]/);
  parts.pop();
  return parts.join('/') || '.';
}

/** The extension of a path. */
export function extname(path: string): string {
  const name = basename(path);
  const dot = name.lastIndexOf('.');
  return dot > 0 ? name.slice(dot) : '';
}

/**
 * Joins the parts of a path.
 *
 * The separator is the one of the platform: a webview on Windows joins
 * with a backslash.
 */
export function join(...parts: string[]): string {
  const separator = navigator.userAgent.includes('Windows') ? '\\' : '/';
  return parts
    .filter((part) => part.length > 0)
    .join(separator)
    .replace(/[\\/]{2,}/g, separator);
}
