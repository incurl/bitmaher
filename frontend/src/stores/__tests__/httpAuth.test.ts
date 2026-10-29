/**
 * @fileoverview The store of the saved HTTP credentials.
 *
 * The credentials used to have no backend at all: the commands fell
 * through to the catch-all of the command map, and the credential the
 * user saved was lost.
 *
 * The database is the record of truth: what the user saved is what the
 * next download reads.
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { invoke } from '@tauri-apps/api/core'

vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn(),
}))

vi.mock('@tauri-apps/api/event', () => ({
  listen: vi.fn(() => Promise.resolve(() => {})),
}))

import { useHttpAuthStore } from '@/stores/httpAuth'

const invokeMock = vi.mocked(invoke)

beforeEach(() => {
  setActivePinia(createPinia())
  invokeMock.mockReset()
  invokeMock.mockResolvedValue(undefined as never)
})

describe('the saved HTTP credentials', () => {
  it('saves the credential through the backend', async () => {
    const store = useHttpAuthStore()

    await store.saveCredential({
      url: 'https://example.com/file.zip',
      username: 'peter',
      password: 'hunter2',
    })

    expect(invokeMock).toHaveBeenCalledWith('http_auth_save', {
      url: 'https://example.com/file.zip',
      username: 'peter',
      password: 'hunter2',
    })
  })

  it('reads the credential of a site', async () => {
    invokeMock.mockResolvedValue({
      id: 'abc123',
      origin: 'https://example.com',
      username: 'peter',
      password: 'hunter2',
    } as never)

    const store = useHttpAuthStore()
    const credential = await store.findByUrl('https://example.com/file.zip')

    expect(invokeMock).toHaveBeenCalledWith('http_auth_find', { url: 'https://example.com/file.zip' })
    expect(credential?.username).toBe('peter')
    expect(credential?.password).toBe('hunter2')
  })

  it('does not ask for the credential of a site that is not http', async () => {
    const store = useHttpAuthStore()

    const credential = await store.findByUrl('magnet:?xt=urn:btih:abcdef')

    expect(credential).toBeNull()
    expect(invokeMock).not.toHaveBeenCalledWith('http_auth_find', expect.anything())
  })

  it('records that the credential was used', async () => {
    const store = useHttpAuthStore()

    await store.markUsed('abc123')

    expect(invokeMock).toHaveBeenCalledWith('http_auth_mark_used', { id: 'abc123' })
  })
})
