/** @fileoverview Tests for tracker source fetch via Rust backend and proxy parameter computation. */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { PROXY_SCOPES } from '@shared/constants'

// ── Module mocks ────────────────────────────────────────────────
const mockInvoke = vi.fn()
vi.mock('@tauri-apps/api/core', () => ({
  invoke: (...args: unknown[]) => mockInvoke(...args),
}))

vi.mock('@shared/logger', () => ({
  logger: {
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
    error: vi.fn(),
  },
}))

import { computeTrackerProxyServer, fetchBtTrackerFromSource, type FetchTrackerSourcesResult } from '../tracker'

// ── computeTrackerProxyServer ───────────────────────────────────

describe('computeTrackerProxyServer', () => {
  it('returns server when proxy enabled with UPDATE_TRACKERS scope', () => {
    expect(
      computeTrackerProxyServer({
        mode: 'manual',
        server: 'http://proxy.example.com:8080',
        scope: [PROXY_SCOPES.UPDATE_TRACKERS],
      }),
    ).toBe('http://proxy.example.com:8080')
  })

  it('adds encoded proxy credentials for app-side tracker requests', () => {
    expect(
      computeTrackerProxyServer({
        mode: 'manual',
        server: 'http://proxy.example.com:8080',
        username: 'user@example.com',
        password: 'pa:ss word',
        scope: [PROXY_SCOPES.UPDATE_TRACKERS],
      }),
    ).toBe('http://user%40example.com:pa%3Ass%20word@proxy.example.com:8080/')
  })

  it('returns server when scope has multiple entries including UPDATE_TRACKERS', () => {
    expect(
      computeTrackerProxyServer({
        mode: 'manual',
        server: 'socks5://localhost:1080',
        scope: [PROXY_SCOPES.DOWNLOAD, PROXY_SCOPES.UPDATE_TRACKERS],
      }),
    ).toBe('socks5://localhost:1080')
  })

  it('returns null when proxy disabled', () => {
    expect(
      computeTrackerProxyServer({
        mode: 'direct',
        server: 'http://proxy.example.com:8080',
        scope: [PROXY_SCOPES.UPDATE_TRACKERS],
      }),
    ).toBeNull()
  })

  it('returns null when scope does not include UPDATE_TRACKERS', () => {
    expect(
      computeTrackerProxyServer({
        mode: 'manual',
        server: 'http://proxy.example.com:8080',
        scope: [PROXY_SCOPES.DOWNLOAD],
      }),
    ).toBeNull()
  })

  it('returns null when scope is empty', () => {
    expect(
      computeTrackerProxyServer({
        mode: 'manual',
        server: 'http://proxy.example.com:8080',
        scope: [],
      }),
    ).toBeNull()
  })

  it('returns null when server is empty string', () => {
    expect(
      computeTrackerProxyServer({
        mode: 'manual',
        server: '',
        scope: [PROXY_SCOPES.UPDATE_TRACKERS],
      }),
    ).toBeNull()
  })

  it('returns null when server is undefined', () => {
    expect(
      computeTrackerProxyServer({
        mode: 'manual',
        scope: [PROXY_SCOPES.UPDATE_TRACKERS],
      }),
    ).toBeNull()
  })

  it('returns null for empty config object', () => {
    expect(computeTrackerProxyServer({})).toBeNull()
  })

  it('defaults scope to empty array when not provided', () => {
    expect(
      computeTrackerProxyServer({
        mode: 'manual',
        server: 'http://proxy.example.com:8080',
      }),
    ).toBeNull()
  })
})

// ── fetchBtTrackerFromSource ────────────────────────────────────

describe('fetchBtTrackerFromSource', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns empty result without calling invoke for empty source list', async () => {
    const result = await fetchBtTrackerFromSource([])
    expect(result.data).toEqual([])
    expect(result.failures).toEqual([])
    expect(mockInvoke).not.toHaveBeenCalled()
  })

  it('calls invoke with correct command name and url parameters', async () => {
    const mockResult: FetchTrackerSourcesResult = { data: ['body1'], failures: [] }
    mockInvoke.mockResolvedValueOnce(mockResult)

    await fetchBtTrackerFromSource(['https://example.com/trackers.txt'], { mode: 'direct' })

    expect(mockInvoke).toHaveBeenCalledTimes(1)
    expect(mockInvoke).toHaveBeenCalledWith('fetch_tracker_sources', {
      urls: ['https://example.com/trackers.txt'],
      proxyServer: null,
    })
  })

  it('passes multiple URLs to invoke', async () => {
    const mockResult: FetchTrackerSourcesResult = { data: ['a', 'b'], failures: [] }
    mockInvoke.mockResolvedValueOnce(mockResult)

    await fetchBtTrackerFromSource(['https://example.com/trackers1.txt', 'https://example.com/trackers2.txt'])

    expect(mockInvoke).toHaveBeenCalledWith('fetch_tracker_sources', {
      urls: ['https://example.com/trackers1.txt', 'https://example.com/trackers2.txt'],
      proxyServer: null,
    })
  })

  it('passes proxy server when proxy is enabled with correct scope', async () => {
    const mockResult: FetchTrackerSourcesResult = { data: [], failures: [] }
    mockInvoke.mockResolvedValueOnce(mockResult)

    await fetchBtTrackerFromSource(['https://example.com/trackers.txt'], {
      mode: 'manual',
      server: 'http://proxy:8080',
      scope: [PROXY_SCOPES.UPDATE_TRACKERS],
    })

    expect(mockInvoke).toHaveBeenCalledWith('fetch_tracker_sources', {
      urls: ['https://example.com/trackers.txt'],
      proxyServer: 'http://proxy:8080',
    })
  })

  it('passes null proxy when proxy enabled but wrong scope', async () => {
    const mockResult: FetchTrackerSourcesResult = { data: [], failures: [] }
    mockInvoke.mockResolvedValueOnce(mockResult)

    await fetchBtTrackerFromSource(['https://example.com/trackers.txt'], {
      mode: 'manual',
      server: 'http://proxy:8080',
      scope: [PROXY_SCOPES.DOWNLOAD],
    })

    expect(mockInvoke).toHaveBeenCalledWith('fetch_tracker_sources', {
      urls: ['https://example.com/trackers.txt'],
      proxyServer: null,
    })
  })

  it('returns data from fully successful fetch', async () => {
    const body = 'udp://tracker1:6969\nudp://tracker2:6969'
    const mockResult: FetchTrackerSourcesResult = { data: [body], failures: [] }
    mockInvoke.mockResolvedValueOnce(mockResult)

    const result = await fetchBtTrackerFromSource(['https://example.com/trackers.txt'])
    expect(result.data).toEqual([body])
    expect(result.failures).toEqual([])
  })

  it('returns both data and failures for partial success', async () => {
    const mockResult: FetchTrackerSourcesResult = {
      data: ['udp://tracker1:6969'],
      failures: [{ url: 'https://bad.example.com/trackers.txt', reason: 'Network Error' }],
    }
    mockInvoke.mockResolvedValueOnce(mockResult)

    const result = await fetchBtTrackerFromSource([
      'https://good.example.com/trackers.txt',
      'https://bad.example.com/trackers.txt',
    ])
    expect(result.data).toHaveLength(1)
    expect(result.failures).toHaveLength(1)
    expect(result.failures[0].url).toBe('https://bad.example.com/trackers.txt')
    expect(result.failures[0].reason).toBe('Network Error')
  })

  it('returns empty data with all failures for total failure', async () => {
    const mockResult: FetchTrackerSourcesResult = {
      data: [],
      failures: [
        { url: 'https://a.example.com', reason: 'timeout' },
        { url: 'https://b.example.com', reason: 'DNS error' },
      ],
    }
    mockInvoke.mockResolvedValueOnce(mockResult)

    const result = await fetchBtTrackerFromSource(['https://a.example.com', 'https://b.example.com'])
    expect(result.data).toEqual([])
    expect(result.failures).toHaveLength(2)
    expect(result.failures[0].reason).toBe('timeout')
    expect(result.failures[1].reason).toBe('DNS error')
  })

  it('propagates invoke errors as rejections', async () => {
    mockInvoke.mockRejectedValueOnce(new Error('IPC failure'))

    await expect(fetchBtTrackerFromSource(['https://example.com/trackers.txt'])).rejects.toThrow('IPC failure')
  })

  it('uses default empty proxyConfig when not provided', async () => {
    const mockResult: FetchTrackerSourcesResult = { data: ['body'], failures: [] }
    mockInvoke.mockResolvedValueOnce(mockResult)

    await fetchBtTrackerFromSource(['https://example.com/trackers.txt'])

    expect(mockInvoke).toHaveBeenCalledWith('fetch_tracker_sources', {
      urls: ['https://example.com/trackers.txt'],
      proxyServer: null,
    })
  })
})
