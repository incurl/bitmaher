/**
 * The Tauri-compatible locale plugin.
 *
 * The plugin asked the operating system for the locale. A webview knows
 * the locale of the browser, which is the one the user reads, and it
 * falls back to the language of the navigator.
 */
import { locale } from './os';

/**
 * Returns the locale of the system.
 *
 * The answer is the tag of the browser, such as "en-GB" or "zh-CN".
 */
export async function getLocale(): Promise<string> {
  return locale() || 'en-US';
}
