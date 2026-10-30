/** @fileoverview Revealing the files of a download in the file manager. */
import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockInvoke = vi.fn()
vi.mock('@tauri-apps/api/core', () => ({
  invoke: (command: string, args?: Record<string, unknown>) => mockInvoke(command, args),
}))

vi.mock('@tauri-apps/api/path', () => ({
  join: (...parts: string[]) => Promise.resolve(parts.join('/')),
  dirname: (path: string) => Promise.resolve(path.split('/').slice(0, -1).join('/')),
}))

const mockGetFiles = vi.fn()
vi.mock('@/api/aria2', () => ({
  getFiles: (params: { gid: string }) => mockGetFiles(params),
}))

vi.mock('@shared/logger', () => ({
  logger: { debug: vi.fn(), warn: vi.fn(), error: vi.fn(), info: vi.fn() },
}))

const { revealTaskInFolder, resolveTaskOpenTarget, readTaskFiles } = await import('../useTaskReveal')
import type { Aria2File, Aria2Task } from '@shared/types'

function makeTask(overrides: Partial<Aria2Task> = {}): Aria2Task {
  return {
    gid: 'g1',
    status: 'complete',
    totalLength: '1000',
    completedLength: '1000',
    uploadLength: '0',
    downloadSpeed: '0',
    uploadSpeed: '0',
    connections: '0',
    dir: '/downloads',
    files: [],
    ...overrides,
  }
}

function makeFile(overrides: Partial<Aria2File> = {}): Aria2File {
  return {
    index: '1',
    path: '/downloads/movie.mkv',
    length: '1000',
    completedLength: '1000',
    selected: 'true',
    uris: [],
    ...overrides,
  }
}

/** Answers `check_path_exists` with the paths that are on the disk. */
function withExistingPaths(...paths: string[]) {
  mockInvoke.mockImplementation((command: string, args?: Record<string, unknown>) => {
    if (command === 'check_path_exists') return Promise.resolve(paths.includes(args?.path as string))
    if (command === 'show_item_in_dir') return Promise.resolve(undefined)
    return Promise.resolve(undefined)
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  mockGetFiles.mockResolvedValue([])
})

describe('readTaskFiles', () => {
  it('reads the files from the engine when the record carries none', async () => {
    mockGetFiles.mockResolvedValue([makeFile()])

    const files = await readTaskFiles(makeTask())

    expect(mockGetFiles).toHaveBeenCalledWith({ gid: 'g1' })
    expect(files).toHaveLength(1)
  })

  it('does not read the engine when the record carries the files', async () => {
    const files = await readTaskFiles(makeTask({ files: [makeFile()] }))

    expect(mockGetFiles).not.toHaveBeenCalled()
    expect(files).toHaveLength(1)
  })

  it('survives a download the engine has forgotten', async () => {
    mockGetFiles.mockRejectedValue(new Error('unknown gid'))

    await expect(readTaskFiles(makeTask())).resolves.toEqual([])
  })
})

describe('revealTaskInFolder', () => {
  it('reveals the file of a download whose record carries no files', async () => {
    // The bug: the record carries no file list, so the action returned
    // before it revealed anything.
    mockGetFiles.mockResolvedValue([makeFile()])
    withExistingPaths('/downloads/movie.mkv')

    const outcome = await revealTaskInFolder(makeTask())

    expect(outcome).toBe('revealed')
    expect(mockInvoke).toHaveBeenCalledWith('show_item_in_dir', { path: '/downloads/movie.mkv' })
  })

  it('reveals the selected file rather than the first one', async () => {
    mockGetFiles.mockResolvedValue([
      makeFile({ index: '1', path: '/downloads/unwanted.txt', selected: 'false' }),
      makeFile({ index: '2', path: '/downloads/wanted.mkv', selected: 'true' }),
    ])
    withExistingPaths('/downloads/wanted.mkv')

    await revealTaskInFolder(makeTask())

    expect(mockInvoke).toHaveBeenCalledWith('show_item_in_dir', { path: '/downloads/wanted.mkv' })
  })

  it('shows the folder of a torrent when the file is gone', async () => {
    mockGetFiles.mockResolvedValue([makeFile()])
    withExistingPaths('/downloads/Big Buck Bunny')

    const outcome = await revealTaskInFolder(
      makeTask({ bittorrent: { info: { name: 'Big Buck Bunny' } } }),
    )

    expect(outcome).toBe('revealed')
    expect(mockInvoke).toHaveBeenCalledWith('show_item_in_dir', { path: '/downloads/Big Buck Bunny' })
  })

  it('shows the folder that holds the file when the file is gone', async () => {
    mockGetFiles.mockResolvedValue([makeFile()])
    withExistingPaths('/downloads')

    const outcome = await revealTaskInFolder(makeTask())

    expect(outcome).toBe('revealed')
    expect(mockInvoke).toHaveBeenCalledWith('show_item_in_dir', { path: '/downloads' })
  })

  it('says that nothing is there when neither the file nor the folder is', async () => {
    mockGetFiles.mockResolvedValue([makeFile()])
    withExistingPaths()

    const outcome = await revealTaskInFolder(makeTask())

    expect(outcome).toBe('missing')
    expect(mockInvoke).not.toHaveBeenCalledWith('show_item_in_dir', expect.anything())
  })

  it('says that nothing is there when the engine has forgotten the download', async () => {
    mockGetFiles.mockRejectedValue(new Error('unknown gid'))
    withExistingPaths()

    const outcome = await revealTaskInFolder(makeTask())

    expect(outcome).toBe('missing')
  })
})

describe('resolveTaskOpenTarget', () => {
  it('opens the file of a download whose record carries no files', async () => {
    mockGetFiles.mockResolvedValue([makeFile()])

    expect(await resolveTaskOpenTarget(makeTask())).toBe('/downloads/movie.mkv')
  })

  it('opens the folder of a torrent of several files', async () => {
    mockGetFiles.mockResolvedValue([
      makeFile({ index: '1', path: '/downloads/Big Buck Bunny/one.mkv' }),
      makeFile({ index: '2', path: '/downloads/Big Buck Bunny/two.mkv' }),
    ])

    expect(await resolveTaskOpenTarget(makeTask({ bittorrent: { info: { name: 'Big Buck Bunny' } } }))).toBe(
      '/downloads/Big Buck Bunny',
    )
  })

  it('falls back to the folder of the download when there is no file', async () => {
    mockGetFiles.mockResolvedValue([])

    expect(await resolveTaskOpenTarget(makeTask())).toBe('/downloads')
  })
})
