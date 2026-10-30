/** @fileoverview Revealing the files of a download in the file manager. */
import { dirname, join } from '@tauri-apps/api/path'
import { invoke } from '@tauri-apps/api/core'
import { getFiles } from '@/api/aria2'
import { resolveTaskFilePath } from '@/composables/useTaskPaths'
import { resolveOpenTarget } from '@shared/utils/task'
import { getErrorMessage } from '@shared/utils/errorMessage'
import { logger } from '@shared/logger'
import type { Aria2File, Aria2Task } from '@shared/types'

/**
 * The answer of a reveal: something was shown, or nothing was there.
 */
export type RevealOutcome = 'revealed' | 'missing'

/**
 * The files of a download.
 *
 * The record the backend keeps carries no file list: the files belong
 * to the engine, and they are read from it when they are needed. A
 * download the engine has forgotten has none, and its own record is
 * the answer.
 */
export async function readTaskFiles(task: Aria2Task): Promise<Aria2File[]> {
  if (task.files?.length) return task.files
  try {
    return await getFiles({ gid: task.gid })
  } catch (error) {
    logger.debug('TaskReveal.files', getErrorMessage(error))
    return []
  }
}

/**
 * The task with its files.
 *
 * The record the list shows carries no file list, and the files of the
 * download are read from the engine to fill it.
 */
async function withFiles(task: Aria2Task): Promise<Aria2Task> {
  const files = await readTaskFiles(task)
  return files.length ? { ...task, files } : task
}

/**
 * The folder a download was written to.
 *
 * It is the answer when the file itself is not there any more: the
 * file may have been deleted, and the folder may not have been.
 */
async function resolveTaskFolder(task: Aria2Task): Promise<string> {
  // A torrent writes its content into a folder of its own, named after
  // the torrent.
  const torrentName = task.bittorrent?.info?.name
  if (torrentName) return await join(task.dir, torrentName)

  // Otherwise the folder the file is in, which is the torrent's own
  // folder for a download of several files.
  const file = resolveTaskFilePath(task)
  if (file) return await dirname(file)

  // And when there is no file either, the folder of the download.
  return task.dir
}

/**
 * The path to open for a download.
 *
 * A torrent of several files is opened as the folder that holds them,
 * and a download of one file is opened as the file itself. The files
 * are read from the engine when the record does not carry them.
 */
export async function resolveTaskOpenTarget(task: Aria2Task): Promise<string> {
  return await resolveOpenTarget(await withFiles(task))
}

/**
 * Reveals the files of a download in the file manager.
 *
 * The file is revealed when it is there. When it is not, the folder
 * that holds it is shown instead, because the file may have been
 * deleted and the folder may not have been.
 */
export async function revealTaskInFolder(task: Aria2Task): Promise<RevealOutcome> {
  const withFileList = await withFiles(task)

  const filePath = resolveTaskFilePath(withFileList)
  if (filePath && (await pathExists(filePath))) {
    await invoke('show_item_in_dir', { path: filePath })
    return 'revealed'
  }

  const folder = await resolveTaskFolder(withFileList)
  if (folder && folder !== filePath && (await pathExists(folder))) {
    await invoke('show_item_in_dir', { path: folder })
    return 'revealed'
  }

  return 'missing'
}

/** Answers whether a path is on the disk. */
async function pathExists(path: string): Promise<boolean> {
  return await invoke<boolean>('check_path_exists', { path })
}
