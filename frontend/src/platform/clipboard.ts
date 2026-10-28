/**
 * The Tauri-compatible clipboard API.
 *
 * The Wails runtime writes to the clipboard of the system. A webview
 * does not always have a secure context, so the frontend keeps the
 * fallback it had: a hidden text area and `document.execCommand`.
 */
import { Clipboard } from '@wailsio/runtime';

/** Writes text to the clipboard. */
export async function writeText(text: string): Promise<void> {
  try {
    await Clipboard.SetText(text);
    return;
  } catch {
    // The runtime refused, which happens in a webview without a secure
    // context. The fallback below takes over.
  }

  const area = document.createElement('textarea');
  area.value = text;
  area.setAttribute('readonly', '');
  // The fallback must stay in the layout: a hidden element cannot be
  // selected, and the copy would fail.
  area.style.position = 'fixed';
  area.style.left = '-9999px';
  area.style.opacity = '0';
  document.body.appendChild(area);
  area.select();
  try {
    document.execCommand('copy');
  } finally {
    document.body.removeChild(area);
  }
}

/** Reads text from the clipboard. */
export async function readText(): Promise<string> {
  return Clipboard.Text();
}
