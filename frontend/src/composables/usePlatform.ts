/**
 * @fileoverview Centralised platform detection composable.
 *
 * Provides **atomic** boolean flags (`isMac`, `isWindows`, `isLinux`) plus
 * human-readable labels.  Consumers compose flags inline:
 *
 * ```ts
 * const { isMac, isWindows } = usePlatform()
 * // v-if="isMac || isWindows"   ← replaces old isMacOrWin
 * ```
 *
 * The module initialises once on first `usePlatform()` call (singleton).
 * Repeated calls return the same reactive refs — no extra `platform()` calls.
 */
import { computed, ref } from 'vue'
import type { ComputedRef, Ref } from 'vue'
import { platform } from '@tauri-apps/plugin-os'
import { logger } from '@shared/logger'

// ─── Module-level singleton ────────────────────────────────────────────
const _platform = ref('')
let _initialised = false

function _ensureInit(): void {
  if (_initialised) return
  _initialised = true
  try {
    _platform.value = platform()
  } catch (e) {
    logger.debug('Platform', `platform() unavailable (SSR/test context): ${e}`)
  }
}

const _isMac = computed(() => _platform.value === 'macos')
const _isWindows = computed(() => _platform.value === 'windows')
const _isLinux = computed(() => _platform.value === 'linux')

const PLATFORM_LABELS: Record<string, string> = {
  macos: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
}

const _platformLabel = computed(() => PLATFORM_LABELS[_platform.value] ?? _platform.value)

// ─── Architecture label utility ────────────────────────────────────────

const ARCH_LABELS: Record<string, string> = {
  aarch64: 'ARM64',
  x86_64: 'x64',
  x86: 'x86',
}

/**
 * Map a raw architecture string to a human-readable label.
 * On macOS, `aarch64` → "Apple Silicon" and `x86_64` → "Intel".
 */
function _archLabel(arch: string): string {
  if (_isMac.value) {
    return arch === 'aarch64' ? 'Apple Silicon' : 'Intel'
  }
  return ARCH_LABELS[arch] ?? arch
}

// ─── Public API ─────────────────────────────────────────────────────────

export interface UsePlatformReturn {
  /** Raw platform string: `'macos'`, `'windows'`, `'linux'`, or `''`. */
  platform: Ref<string>
  /** `true` when running on macOS. */
  isMac: ComputedRef<boolean>
  /** `true` when running on Windows. */
  isWindows: ComputedRef<boolean>
  /** `true` when running on Linux. */
  isLinux: ComputedRef<boolean>
  /** Human-readable platform name: `'macOS'`, `'Windows'`, `'Linux'`. */
  platformLabel: ComputedRef<string>
  /** Map an architecture string to a human-readable label. */
  archLabel: (arch: string) => string
}

/**
 * Composable providing platform detection primitives.
 *
 * Returns the **same** singleton refs on every call — safe to call from
 * multiple components without redundant `platform()` invocations.
 */
export function usePlatform(): UsePlatformReturn {
  _ensureInit()
  return {
    platform: _platform,
    isMac: _isMac,
    isWindows: _isWindows,
    isLinux: _isLinux,
    platformLabel: _platformLabel,
    archLabel: _archLabel,
  }
}
