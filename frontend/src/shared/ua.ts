/** @fileoverview User-agent string presets for HTTP request identity simulation. */

// ─── Browser User-Agents (by global market share) ────────────────────────────
// Platform: Windows NT 10.0 for Chromium/Firefox (61% desktop OS share).
// Safari uses macOS because it is an Apple-exclusive browser.
// Version numbers reflect Chrome UA Reduction (minor frozen at 0.0.0).

export const CHROME_UA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36'

export const EDGE_UA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0'

export const SAFARI_UA =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Safari/605.1.15'

export const FIREFOX_UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:150.0) Gecko/20100101 Firefox/150.0'

// ─── Preset Map (insertion order = UI button order) ──────────────────────────

const userAgentMap: Record<string, string> = {
  chrome: CHROME_UA,
  edge: EDGE_UA,
  safari: SAFARI_UA,
  firefox: FIREFOX_UA,
}

export default userAgentMap
