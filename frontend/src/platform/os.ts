/**
 * The Tauri-compatible operating system API.
 *
 * The Wails runtime reports the platform the same way.
 */
import { System } from '@wailsio/runtime';

/**
 * The platform the application runs on.
 *
 * The Tauri plugin exposes it as a function, so the callers call it.
 */
export function platform(): 'linux' | 'macos' | 'windows' | 'android' | 'ios' | 'unknown' {
  if (System.IsMac()) return 'macos';
  if (System.IsWindows()) return 'windows';
  if (System.IsLinux()) return 'linux';
  return 'unknown';
}

/** The version of the operating system. */
export function version(): string {
  return 'unknown';
}

/** The architecture of the processor. */
export function arch(): string {
  return System.IsARM64() ? 'aarch64' : System.IsAMD64() ? 'x86_64' : 'unknown';
}

/** The name of the operating system. */
export function family(): string {
  const current = platform();
  return current === 'macos' ? 'macOS' : current === 'windows' ? 'Windows' : 'Linux';
}

/**
 * Reads the locale of the system.
 *
 * The Tauri plugin asked the operating system. A webview knows the
 * locale of the browser, which is the one the user reads.
 */
export function locale(): string {
  return typeof navigator !== 'undefined' ? navigator.language : 'en-US';
}
