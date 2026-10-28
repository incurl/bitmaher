import { appDataDir, downloadDir, homeDir, join } from '@tauri-apps/api/path'
import { invoke } from '@tauri-apps/api/core'

export type UserVisibleDirectorySource = 'configured' | 'system-downloads' | 'home-downloads' | 'home' | 'app-data'

export interface ResolvedUserVisibleDirectory {
  path: string
  source: UserVisibleDirectorySource
  usedFallback: boolean
}

export interface UserVisibleDirectoryDeps {
  downloadDir: () => Promise<string>
  homeDir: () => Promise<string>
  appDataDir: () => Promise<string>
  pathExists: (path: string) => Promise<boolean>
  join: (...paths: string[]) => Promise<string>
}

export interface ResolveUserVisibleDownloadDirOptions {
  configuredDir?: string
  deps?: UserVisibleDirectoryDeps
}

const defaultDeps: UserVisibleDirectoryDeps = {
  downloadDir,
  homeDir,
  appDataDir,
  pathExists: (path: string) => invoke<boolean>('check_path_exists', { path }),
  join: (...parts: string[]) => Promise.resolve(join(...parts)),
}

function cleanPath(path: string | undefined): string {
  return (path ?? '').trim()
}

function normalizeForCompare(path: string): string {
  const normalized = path.replace(/\\/g, '/').replace(/\/+$/, '')
  return normalized || '/'
}

async function pathExists(deps: UserVisibleDirectoryDeps, path: string): Promise<boolean> {
  try {
    return await deps.pathExists(path)
  } catch {
    return false
  }
}

export function shouldPersistResolvedDownloadDir(resolved: ResolvedUserVisibleDirectory): boolean {
  return resolved.source === 'system-downloads' || resolved.source === 'home-downloads'
}

export async function resolveUserVisibleDownloadDir(
  options: ResolveUserVisibleDownloadDirOptions = {},
): Promise<ResolvedUserVisibleDirectory> {
  const deps = options.deps ?? defaultDeps
  const configuredDir = cleanPath(options.configuredDir)
  if (configuredDir) {
    return { path: configuredDir, source: 'configured', usedFallback: false }
  }

  let home = ''
  let homeDownloads = ''
  try {
    home = cleanPath(await deps.homeDir())
    if (home) homeDownloads = await deps.join(home, 'Downloads')
  } catch {
    home = ''
    homeDownloads = ''
  }

  let systemDownloads = ''
  try {
    systemDownloads = cleanPath(await deps.downloadDir())
  } catch {
    systemDownloads = ''
  }

  const systemIsHome = !!systemDownloads && !!home && normalizeForCompare(systemDownloads) === normalizeForCompare(home)

  if (systemDownloads && !systemIsHome) {
    return { path: systemDownloads, source: 'system-downloads', usedFallback: false }
  }

  const homeDownloadsExists = homeDownloads ? await pathExists(deps, homeDownloads) : false
  if (homeDownloads && homeDownloadsExists) {
    return { path: homeDownloads, source: 'home-downloads', usedFallback: true }
  }

  if (systemDownloads) {
    return {
      path: systemIsHome && home ? home : systemDownloads,
      source: systemIsHome ? 'home' : 'system-downloads',
      usedFallback: systemIsHome,
    }
  }

  if (home) {
    return { path: home, source: 'home', usedFallback: true }
  }

  return { path: await deps.appDataDir(), source: 'app-data', usedFallback: true }
}
