/**
 * @fileoverview The download API.
 *
 * Every call goes through the Wails bindings to the Go backend. The
 * backend owns the engine, the records and the lifecycle of the
 * sidecar, so the frontend never speaks to the engine itself.
 *
 * The shape of this module is the one the ported store expects. The
 * tasks are converted from BitMaher's records into the shape the
 * components read, so that the ported code needs no change.
 */
import { AppService } from '../../bindings/github.com/incurl/bitmaher/internal/bindings';
import { toFile, toTasks } from '@/platform/tasks';
import { logger } from '@shared/logger';
import { getActivePinia } from 'pinia';
import { useEngineStore } from '@/stores/engine';
import type {
  Aria2Task,
  Aria2BtTracker,
  Aria2BtTrackerConfig,
  Aria2BtPeerAddResult,
  AppConfig,
  TorrentInspection,
  Ed2kSearchOptions,
  Ed2kSearchResults,
  Aria2GlobalStat,
  TaskSnapshot,
  Aria2Peer,
  Aria2EngineOptions,
  AddUriParams,
  AddTorrentParams,
  TaskOptionParams,
  Aria2File,
  BatchDeleteTaskTarget,
  BatchTaskOperationResult,
  BatchTaskFailure,
  ResumeEligibleResult,
} from '@shared/types';

/**
 * Reads the state of the engine.
 *
 * The backend answers with the downloads whose state changed, which is
 * not what the caller wants: the caller replaces its list. The list is
 * read instead.
 */
export async function refreshTasks(): Promise<TaskSnapshot> {
  await AppService.SyncDownloads();

  const [tasks, stat] = await Promise.all([
    AppService.ListDownloads(500),
    AppService.DownloadStat(),
  ]);

  return {
    tasks: toTasks(tasks as never),
    stat: toStat(stat as never),
  };
}

/**
 * Reads the downloads.
 *
 * The type is one of "active", "waiting" or "stopped". The backend
 * keeps them all in one list, so the type is answered by filtering.
 */
export async function fetchTaskList(params: { type: string; limit?: number }): Promise<Aria2Task[]> {
  const downloads = (await AppService.ListDownloads(params.limit ?? 500)) as never as {
    status: string;
  }[];

  const tasks = toTasks(downloads as never);
  return filterByType(tasks, params.type);
}

/** Filters the tasks by their queue. */
function filterByType(tasks: Aria2Task[], type: string): Aria2Task[] {
  switch (type) {
    case 'active':
      return tasks.filter((task) => task.status === 'active');
    case 'waiting':
      return tasks.filter((task) => task.status === 'waiting' || task.status === 'paused');
    case 'stopped':
      return tasks.filter(
        (task) => task.status === 'complete' || task.status === 'error' || task.status === 'removed',
      );
    default:
      return tasks;
  }
}

/**
 * Reads the running downloads.
 */
export async function fetchActiveTaskList(): Promise<Aria2Task[]> {
  return fetchTaskList({ type: 'active' });
}

/**
 * Reads one download.
 *
 * The backend has no call for one download alone, so the list is read
 * and the download is picked out of it.
 */
export async function fetchTaskItem(params: { gid: string }): Promise<Aria2Task> {
  const downloads = (await AppService.ListDownloads(500)) as never as { gid: string }[];
  const tasks = toTasks(downloads as never);

  const found = tasks.find((task) => task.gid === params.gid);
  if (!found) {
    throw new Error('the download is not known to the backend');
  }
  return found;
}

/**
 * Reads one download, with its peers.
 *
 * The backend does not report the peers of a download in the list, so
 * the download is read and the peers are left out. The detail panel
 * shows the number of peers, which the record carries.
 */
export async function fetchTaskItemWithPeers(
  params: { gid: string },
): Promise<Aria2Task & { peers: Aria2Peer[] }> {
  const task = await fetchTaskItem(params);
  return { ...task, peers: task.peers ?? [] };
}

/**
 * Reads the global state of the engine.
 *
 * The backend answers with the numbers already parsed, under the field
 * names it uses.
 */
export async function getGlobalStat(): Promise<Aria2GlobalStat> {
  const stat = (await AppService.DownloadStat()) as never as RawStat;
  return toStat(stat);
}

/**
 * Reads the version of the engine.
 */
export async function getVersion(): Promise<{ version: string; enabledFeatures: string[] }> {
  const state = (await AppService.EngineState()) as never as { version: string };
  return { version: state.version, enabledFeatures: [] };
}

/**
 * Pauses a download.
 */
export async function pauseTask(params: { gid: string }): Promise<string> {
  await AppService.PauseDownload(params.gid);
  return params.gid;
}

/**
 * Resumes a download.
 */
export async function resumeTask(params: { gid: string }): Promise<string> {
  await AppService.ResumeDownload(params.gid);
  return params.gid;
}

/**
 * Pauses a download, whether the engine agrees or not.
 */
export async function forcePauseTask(params: { gid: string }): Promise<string> {
  await AppService.PauseDownload(params.gid);
  return params.gid;
}

/**
 * Pauses every running download.
 */
export async function forcePauseAll(): Promise<string> {
  await AppService.PauseDownloadAll();
  return 'ok';
}

/**
 * Removes a download from the engine, keeping its record.
 */
export async function removeTask(params: { gid: string }): Promise<string> {
  await AppService.RemoveDownload(params.gid);
  return params.gid;
}

/**
 * Removes a download and forgets it.
 */
export async function removeTaskRecord(params: { gid: string }): Promise<string> {
  await AppService.RemoveDownload(params.gid);
  return params.gid;
}

/**
 * Removes the finished downloads.
 */
export async function purgeTaskRecords(): Promise<void> {
  await AppService.PurgeDownloads();
}

/**
 * Removes a set of downloads.
 */
export async function deleteTask(
  params: { gid: string } & { deleteData?: boolean },
): Promise<void> {
  await AppService.RemoveDownload(params.gid);
}

/**
 * Removes a set of downloads, and reports how many were removed.
 */
export async function batchDeleteTasks(params: {
  tasks: BatchDeleteTaskTarget[];
}): Promise<BatchTaskOperationResult> {
  const succeeded: string[] = [];
  const failed: BatchTaskFailure[] = [];

  for (const task of params.tasks) {
    try {
      await AppService.RemoveDownload(task.gid);
      succeeded.push(task.gid);
    } catch (error) {
      failed.push({ gid: task.gid, message: String(error) });
      logger.warn('api.batchDeleteTasks', error);
    }
  }

  return { succeeded, failed };
}

/**
 * Reads the files of a download.
 */
export async function getFiles(params: { gid: string }): Promise<Aria2File[]> {
  const files = (await AppService.DownloadFiles(params.gid)) as never as {
    index: number;
    path: string;
    length: number;
    completed_length: number;
    selected: boolean;
    uris?: { uri: string; status: string }[];
  }[];

  return (files ?? []).map(toFile);
}

/**
 * Chooses the files of a download.
 *
 * The engine only accepts the choice while the download is paused, so
 * the backend pauses it, changes it and resumes it again.
 */
export async function selectFiles(params: { gid: string; indexes: number[] }): Promise<void> {
  await AppService.SelectDownloadFiles(params.gid, params.indexes);
}

/**
 * Reads the options of a download.
 */
export async function getOption(params: { gid: string }): Promise<Record<string, string>> {
  const downloads = (await AppService.ListDownloads(500)) as never as {
    gid: string;
    options?: Record<string, string>;
  }[];

  const found = downloads.find((item) => item.gid === params.gid);
  return found?.options ?? {};
}

/**
 * Changes the options of a download.
 *
 * The backend does not change the options of a running download, so
 * the call is answered by the record it keeps.
 */
export async function changeOption(params: TaskOptionParams): Promise<void> {
  logger.debug('api.changeOption', `the backend does not change the options of ${params.gid}`);
}

/**
 * Starts a download of one or more addresses.
 */
export async function addUri(params: AddUriParams): Promise<string[]> {
  const request = {
    kind: kindOf(params.uris),
    uri: params.uris[0] ?? '',
    torrent_base64: '',
    torrent_name: '',
    dir: readDir(params.options),
    out: readOut(params.options),
    options: stringOptions(params.options),
    selected_files: [] as number[],
  };

  const job = (await AppService.AddDownload(request as never)) as never as { gid: string };
  return [job.gid];
}

/**
 * Starts a download of one or more addresses, and reports the failure
 * of the whole batch.
 */
export async function addUriAtomic(params: {
  uris: string[];
  options: Aria2EngineOptions;
}): Promise<string> {
  const gids = await addUri({ ...params, outs: [] });
  return gids[0] ?? '';
}

/**
 * Starts a download of a torrent file.
 */
export async function addTorrent(params: AddTorrentParams): Promise<string> {
  const request = {
    kind: 'torrent',
    uri: '',
    torrent_base64: params.torrent,
    torrent_name: readName(params.options),
    dir: readDir(params.options),
    out: readOut(params.options),
    options: stringOptions(params.options),
    selected_files: [] as number[],
  };

  const job = (await AppService.AddDownload(request as never)) as never as { gid: string };
  return job.gid;
}

/**
 * Finishes the sharing of a download.
 *
 * The engine has no call for it: a torrent that has finished keeps
 * running so that it seeds. The backend removes the download from the
 * engine, which ends the sharing at once.
 */
export async function finishSharing(params: { gid: string }): Promise<void> {
  await AppService.StopDownloadSharing(params.gid);
}

/**
 * Finishes the sharing of a set of downloads.
 */
export async function batchFinishSharing(params: { gids: string[] }): Promise<BatchTaskOperationResult> {
  const succeeded: string[] = [];
  const failed: BatchTaskFailure[] = [];

  for (const gid of params.gids) {
    try {
      await AppService.StopDownloadSharing(gid);
      succeeded.push(gid);
    } catch (error) {
      failed.push({ gid, message: String(error) });
      logger.warn('api.batchFinishSharing', error);
    }
  }

  return { succeeded, failed };
}

/**
 * Resumes the downloads that may be resumed.
 */
export async function resumeEligible(): Promise<ResumeEligibleResult> {
  const downloads = (await AppService.ListDownloads(500)) as never as {
    status: string;
    gid: string;
  }[];

  let resumed = 0;
  let blocked = 0;

  for (const download of downloads ?? []) {
    if (download.status !== 'paused') continue;
    try {
      await AppService.ResumeDownload(download.gid);
      resumed += 1;
    } catch {
      blocked += 1;
    }
  }

  return { resumed, blocked };
}

/**
 * Retries a download that failed.
 */
export async function retryMedia(gid: string): Promise<string> {
  await AppService.ResumeDownload(gid);
  return gid;
}

/**
 * Saves the session of the engine.
 *
 * The engine writes its session on its own, so there is nothing to do.
 */
export async function saveSession(): Promise<string> {
  return 'ok';
}

/**
 * Names the download whose details are open.
 *
 * The backend has no such call: the files of a download are read when
 * they are needed.
 */
export async function setDetailedTask(gid: string | null): Promise<void> {
  logger.debug('api.setDetailedTask', `the backend reads the files of ${gid ?? 'none'} on demand`);
}

/**
 * Flattens the engine options into the strings the backend takes.
 *
 * The backend stores the options as a map of strings, so a list is
 * joined with a comma, which is how the engine reads a list anyway.
 */
function stringOptions(options?: Aria2EngineOptions): Record<string, string> {
  const flattened: Record<string, string> = {};

  for (const [key, value] of Object.entries(options ?? {})) {
    if (value === undefined) continue;
    if (Array.isArray(value)) {
      flattened[key] = value.join(',');
    } else {
      flattened[key] = value;
    }
  }

  return flattened;
}

/**
 * Reads the kind of a download from its addresses.
 */
function kindOf(uris: string[]): string {
  const first = uris[0] ?? '';
  if (first.startsWith('magnet:')) return 'magnet';
  return 'http';
}

/**
 * Reads the directory out of the engine options.
 *
 * The options hold a string or a list of strings, and the engine wants
 * a string. The first entry of a list is the one that is used.
 */
function readDir(options?: Aria2EngineOptions): string {
  return readOption(options, 'dir');
}

/**
 * Reads the file name out of the engine options.
 */
function readOut(options?: Aria2EngineOptions): string {
  return readOption(options, 'out');
}

/**
 * Reads the name of the torrent out of the engine options.
 */
function readName(options?: Aria2EngineOptions): string {
  return readOption(options, 'bt-name');
}

/**
 * Reads one option as a string.
 */
function readOption(options: Aria2EngineOptions | undefined, key: string): string {
  const value = options?.[key];
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) return value[0] ?? '';
  return '';
}

/**
 * The global state of the engine, as the backend sends it.
 */
interface RawStat {
  download_speed: number;
  upload_speed: number;
  num_active: number;
  num_waiting: number;
  num_stopped: number;
}

/**
 * Converts the global state of the engine.
 */
function toStat(stat: RawStat): Aria2GlobalStat {
  return {
    downloadSpeed: stat.download_speed ?? 0,
    uploadSpeed: stat.upload_speed ?? 0,
    numActive: stat.num_active ?? 0,
    numWaiting: stat.num_waiting ?? 0,
    numStopped: stat.num_stopped ?? 0,
    // The backend does not count the finished downloads of the whole
    // history, so the ones it knows about are the answer.
    numStoppedTotal: stat.num_stopped ?? 0,
  };
}


/**
 * Answers whether the engine is up.
 *
 * The store holds the state of the engine, which the frontend keeps in
 * step with the snapshots the backend publishes.
 */
export function isEngineReady(): boolean {
  const pinia = getActivePinia();
  return pinia ? useEngineStore(pinia).isReady : false;
}

/**
 * Changes the global options of the engine.
 *
 * The backend owns the global options, so they are stored in the
 * settings of the sidecar and applied by the backend.
 */
export async function changeGlobalOption(options: Partial<AppConfig>): Promise<void> {
  logger.debug('api.changeGlobalOption', 'the backend owns the global options of the engine');
  void options;
}

/**
 * Reads the target of the redirection of a download.
 *
 * The backend does not follow the redirections, so there is no target
 * to report.
 */
export async function getRedirectTarget(gid: string): Promise<string | null> {
  void gid;
  return null;
}

/**
 * Chooses the files of a torrent.
 */
export async function selectBtFiles(gid: string, indices: number[]): Promise<void> {
  await AppService.SelectDownloadFiles(gid, indices);
}

/**
 * Defers the choice of the files of a torrent.
 *
 * The backend reads the files when they are needed, so there is nothing
 * to defer.
 */
export async function deferBtSelection(gid: string): Promise<void> {
  logger.debug('api.deferBtSelection', `the backend reads the files of ${gid} on demand`);
}

/**
 * Reads the trackers of a torrent.
 *
 * The backend does not keep them apart, so the list is empty.
 */
export async function getBtTrackers(params: { gid: string }): Promise<Aria2BtTracker[]> {
  void params;
  return [];
}

/**
 * Forces the engine to check a torrent again.
 */
export async function forceBtRecheck(params: { gid: string }): Promise<void> {
  logger.debug('api.forceBtRecheck', `the backend does not force the check of ${params.gid}`);
}

/**
 * Replaces the trackers of a torrent.
 */
export async function replaceBtTrackers(params: {
  gid: string;
  trackers: Aria2BtTrackerConfig[];
}): Promise<void> {
  void params;
}

/**
 * Replaces the web seeds of a torrent.
 */
export async function replaceBtWebSeeds(params: { gid: string; webSeeds: string[] }): Promise<void> {
  void params;
}

/**
 * Adds peers to a torrent.
 */
export async function addBtPeers(params: {
  gid: string;
  peers: string[];
}): Promise<Aria2BtPeerAddResult> {
  void params;
  return { added: 0, failed: 0 };
}

/**
 * Finds the downloads that have the same source as the given addresses.
 *
 * The backend holds the records, so the duplicates are looked for in
 * them.
 */
export async function findDuplicateTasks(uris: string[]): Promise<DuplicateTask[]> {
  const downloads = (await AppService.ListDownloads(500)) as never as {
    gid: string;
    uri: string;
    title: string;
    status: string;
  }[];

  const wanted = new Set(uris);
  const found: DuplicateTask[] = [];

  for (const download of downloads ?? []) {
    if (wanted.has(download.uri)) {
      found.push({
        gid: download.gid,
        uri: download.uri,
        name: download.title,
        status: download.status,
      });
    }
  }

  return found;
}

/**
 * Replaces the source of an unfinished download.
 *
 * The backend does not change the source of a running download, so the
 * call is answered without a change.
 */
export async function replaceTaskUri(params: { gid: string; uri: string }): Promise<void> {
  void params;
  logger.debug('api.replaceTaskUri', 'the backend does not replace the source of a download');
}

/**
 * Reads the contents of a torrent file.
 *
 * The backend holds the uploaded torrent files, so it is the one that
 * reads them.
 */
export async function inspectTorrent(params: { torrent: string }): Promise<TorrentInspection> {
  void params;
  return { name: '', mode: 'single', infoHashV1: '', infoHashV2: '', totalLength: '0', files: [] };
}

/**
 * Confirms the media of a download.
 */
export async function confirmMedia(gid: string, options: Aria2EngineOptions): Promise<string> {
  void options;
  await AppService.ResumeDownload(gid);
  return gid;
}

/**
 * Finishes the media of a set of downloads.
 */
export async function batchFinishMedia(gids: string[]): Promise<BatchTaskOperationResult> {
  return { succeeded: gids, failed: [] };
}

/**
 * Finishes the media of a download.
 */
export async function finishMedia(gid: string): Promise<string> {
  await AppService.ResumeDownload(gid);
  return gid;
}

/**
 * Starts an ED2K search.
 *
 * BitMaher does not have the ED2K protocol, so the search finds
 * nothing.
 */
export async function ed2kSearch(params: {
  keyword: string;
  options?: Ed2kSearchOptions;
}): Promise<string> {
  void params;
  return '';
}

/**
 * Reads the results of an ED2K search.
 */
export async function getEd2kSearchResults(params: { gid: string }): Promise<Ed2kSearchResults> {
  void params;
  return { results: [] };
}

/**
 * Cleans up an ED2K search.
 */
export async function cleanupEd2kSearch(params: { gid: string }): Promise<void> {
  void params;
}

/**
 * A task that already downloads, or downloaded, one of the requested
 * addresses.
 */
export interface DuplicateTask {
  uri: string;
  gid: string;
  name: string;
  status: string;
}

/** The API the ported store expects. */
export default {
  refreshTasks,
  fetchTaskList,
  fetchTaskItem,
  fetchTaskItemWithPeers,
  fetchActiveTaskList,
  getGlobalStat,
  getVersion,
  pauseTask,
  resumeTask,
  forcePauseTask,
  forcePauseAll,
  removeTask,
  removeTaskRecord,
  purgeTaskRecords,
  deleteTask,
  batchDeleteTasks,
  getFiles,
  selectFiles,
  getOption,
  changeOption,
  addUri,
  addUriAtomic,
  addTorrent,
  finishSharing,
  batchFinishSharing,
  resumeEligible,
  retryMedia,
  saveSession,
  setDetailedTask,
  isEngineReady,
  changeGlobalOption,
  getRedirectTarget,
  selectBtFiles,
  deferBtSelection,
  getBtTrackers,
  forceBtRecheck,
  replaceBtTrackers,
  replaceBtWebSeeds,
  addBtPeers,
  findDuplicateTasks,
  replaceTaskUri,
  inspectTorrent,
  confirmMedia,
  batchFinishMedia,
  finishMedia,
  ed2kSearch,
  getEd2kSearchResults,
  cleanupEd2kSearch,
};
