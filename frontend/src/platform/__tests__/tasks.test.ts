/**
 * @fileoverview The adapter between the records and the ported tasks.
 *
 * The record carries the name the backend resolved for a download, and
 * the adapter used to drop it. The engine reports no file list for a
 * live download until it has one, so the card had nothing to show but
 * the "Getting task name..." placeholder — for as long as the download
 * lasted.
 */
import { describe, expect, it } from 'vitest'
import { toTask } from '../tasks'
import { getTaskName } from '@shared/utils/task'
import type { Job } from '../../../bindings/github.com/incurl/bitmaher/internal/downloads/models'

/** The record the backend keeps for one download, with every field set. */
function createMockJob(overrides: Partial<Job> = {}): Job {
  return {
    id: 'rec1',
    gid: 'g1',
    kind: 'http',
    status: 'active',
    title: 'payload.bin',
    uri: 'https://example.test/files/payload.bin',
    dir: '/downloads',
    total_length: 1024,
    completed_length: 512,
    upload_length: 0,
    download_speed: 256,
    upload_speed: 0,
    connections: 8,
    seeders: 0,
    peers: 0,
    info_hash: '',
    error_code: 0,
    error_message: '',
    torrent_file: '',
    selected_files: null,
    options: null,
    created: '2026-01-01T00:00:00Z',
    updated: '2026-01-01T00:00:01Z',
    ...overrides,
  }
}

describe('toTask', () => {
  it('carries the title of the record onto the task', () => {
    expect(toTask(createMockJob()).title).toBe('payload.bin')
  })

  it('carries a torrent title the same way', () => {
    const job = createMockJob({ kind: 'torrent', title: 'ubuntu-24.04.iso' })
    expect(toTask(job).title).toBe('ubuntu-24.04.iso')
  })

  it('carries an empty title as absent, not as an empty string', () => {
    // The backend leaves the title empty when it resolved nothing. An
    // empty string would be a name, so it is dropped and the fallbacks
    // of the name resolution still apply.
    expect(toTask(createMockJob({ title: '' })).title).toBeUndefined()
  })

  it('leaves the engine file list empty, because the record has none', () => {
    expect(toTask(createMockJob()).files).toEqual([])
  })

  it('makes the card show the record title instead of the placeholder', () => {
    const task = toTask(createMockJob())
    expect(getTaskName(task, { defaultName: 'Getting task name...' })).toBe('payload.bin')
  })
})
