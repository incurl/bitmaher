/**
 * @fileoverview GeoIP utilities for BT peer country resolution.
 *
 * Provides:
 * - `countryCodeToFlag()` — ISO 3166-1 alpha-2 → emoji flag (zero dependencies)
 * - `lookupPeerIps()` — batch IP → country lookup via Tauri IPC
 *
 * Attribution: IP geolocation powered by DB-IP (https://db-ip.com, CC BY 4.0)
 */
import { invoke } from '@tauri-apps/api/core'

/** Geolocation result for a single IP address (mirrors Rust `GeoInfo`). */
export interface GeoInfo {
  country_code: string
  country_name: string
  continent: string
}

/**
 * Converts an ISO 3166-1 alpha-2 country code to its emoji flag.
 *
 * Uses Unicode Regional Indicator Symbols: each ASCII letter is offset
 * by 127397 to reach the corresponding regional indicator code point.
 * Two regional indicators form a flag emoji on all modern platforms.
 *
 * @example countryCodeToFlag('US') // '🇺🇸'
 * @example countryCodeToFlag('JP') // '🇯🇵'
 */
export function countryCodeToFlag(code: string): string {
  if (!code || code.length !== 2) return ''
  return String.fromCodePoint(
    ...code
      .toUpperCase()
      .split('')
      .map((c) => 127397 + c.charCodeAt(0)),
  )
}

/**
 * Batch-resolves IP addresses to country information via the Rust backend.
 *
 * Single IPC round-trip for all peers — avoids N individual calls that
 * would thrash the IPC bridge on swarms with 100+ peers.
 *
 * Country and continent names are returned in the user's locale when
 * available (de/en/es/fr/ja/pt-BR/ru/zh-CN), with English fallback.
 *
 * IPs that cannot be resolved are silently omitted from the result.
 */
export async function lookupPeerIps(ips: string[], locale: string): Promise<Record<string, GeoInfo>> {
  if (ips.length === 0) return {}
  return invoke<Record<string, GeoInfo>>('lookup_peer_ips', { ips, locale })
}
