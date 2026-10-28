/** @fileoverview Clipboard helpers shared by app copy actions and auto-detection. */
let lastAppClipboardText = ''

function normalizeClipboardText(text: string): string {
  return text.trim()
}

export function markAppClipboardText(text: string): void {
  lastAppClipboardText = normalizeClipboardText(text)
}

export function shouldIgnoreClipboardTextForAutoDetect(text: string): boolean {
  const normalized = normalizeClipboardText(text)
  return !!normalized && normalized === lastAppClipboardText
}

export async function writeAppClipboardText(text: string): Promise<void> {
  await navigator.clipboard.writeText(text)
  markAppClipboardText(text)
}
