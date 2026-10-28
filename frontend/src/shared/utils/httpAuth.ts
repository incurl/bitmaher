/** @fileoverview Utilities for matching saved HTTP authentication credentials. */

const DEFAULT_PORTS: Record<string, string> = {
  'http:': '80',
  'https:': '443',
}

export function normalizeHttpAuthOrigin(rawUrl: string): string | null {
  try {
    const parsed = new URL(rawUrl.trim())
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null

    const protocol = parsed.protocol.slice(0, -1).toLowerCase()
    const host = parsed.hostname.toLowerCase()
    const port = parsed.port && parsed.port !== DEFAULT_PORTS[parsed.protocol] ? `:${parsed.port}` : ''
    if (!host) return null
    return `${protocol}://${host}${port}`
  } catch {
    return null
  }
}
