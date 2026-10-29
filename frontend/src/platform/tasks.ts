/**
 * The adapter between BitMaher's records and the ported tasks.
 *
 * Rayburst's components work with `Aria2Task`: the shape the engine
 * reports, with the numbers as strings. BitMaher's backend normalises
 * the state of the engine and hands it over as a record, with the
 * numbers as numbers.
 *
 * This module translates between the two, so that the ported components
 * need no change. It is the counterpart of the engine's own
 * normalisation on the other side of the boundary.
 */
import type { Job } from '../../bindings/github.com/incurl/bitmaher/internal/downloads/models';
import type { Aria2File, Aria2Task, TaskSnapshot, TaskStatus } from '@shared/types';

/**
 * The statuses of a task.
 *
 * They are the ones the ported components expect. The backend
 * normalises the engine's own vocabulary into the same set, so the
 * translation is a lookup and not a mapping of several names.
 */
const STATUSES: readonly TaskStatus[] = [
  'active',
  'waiting',
  'paused',
  'error',
  'complete',
  'removed',
];

/**
 * Converts a record into a task.
 *
 * The numbers become strings, because that is how the engine reports
 * them and how the ported formatters read them.
 */
export function toTask(job: Job): Aria2Task {
  const status = normaliseStatus(job.status);

  const task: Aria2Task = {
    gid: job.gid,
    // The record carries the name the backend resolved, and the engine
    // has not reported a file list yet: without it the card has nothing
    // to show but the placeholder. An empty title means "unknown", so it
    // is carried as absent rather than as an empty string.
    title: job.title || undefined,
    status,
    totalLength: String(job.total_length ?? 0),
    completedLength: String(job.completed_length ?? 0),
    uploadLength: String(job.upload_length ?? 0),
    downloadSpeed: String(job.download_speed ?? 0),
    uploadSpeed: String(job.upload_speed ?? 0),
    connections: String(job.connections ?? 0),
    dir: job.dir ?? '',
    // The record carries no file list: the files belong to the engine,
    // and are read on demand -- by the detail panel when it opens, and
    // by the add-task dialog before it shows a torrent.
    files: [],
  };

  // A torrent shows its peers and its seeders, and the detail panel
  // shows the information hash. The other kinds have none of them.
  if (job.kind === 'torrent' || job.kind === 'magnet') {
    task.infoHash = job.info_hash || undefined;
    task.numSeeders = String(job.seeders ?? 0);
  }

  if (job.error_message) {
    task.errorCode = String(job.error_code ?? 0);
    task.errorMessage = job.error_message;
  }

  return task;
}

/**
 * Converts a list of records into tasks.
 *
 * The empty list is returned as an empty array, so that the components
 * that check its length do not have to guard against null.
 */
export function toTasks(jobs: Job[] | null | undefined): Aria2Task[] {
  if (!jobs || jobs.length === 0) return [];
  return jobs.map(toTask);
}

/**
 * Reads the status of a record.
 *
 * An unknown status becomes `waiting`, which is the state that keeps
 * the task in the list without claiming that it is running.
 */
function normaliseStatus(status: string): TaskStatus {
  const found = STATUSES.find((candidate) => candidate === status);
  return found ?? 'waiting';
}

/**
 * Converts a file of a download into the shape the ported components
 * expect.
 *
 * The engine numbers its files from one, and so does the ported code.
 */
export function toFile(file: {
  index: number;
  path: string;
  length: number;
  completed_length: number;
  selected: boolean;
  uris?: { uri: string; status: string }[];
}): Aria2File {
  return {
    index: String(file.index),
    path: file.path,
    length: String(file.length ?? 0),
    completedLength: String(file.completed_length ?? 0),
    selected: file.selected ? 'true' : 'false',
    // The engine reports which source it is reading and which ones it
    // has not tried yet; a torrent's file has no source at all.
    uris: (file.uris ?? []).map((entry) => ({ uri: entry.uri, status: entry.status || 'used' })),
  };
}

/**
 * Converts a list of files into the shape the ported components expect.
 */
export function toFiles(
  files:
    | {
        index: number;
        path: string;
        length: number;
        completed_length: number;
        selected: boolean;
        uris?: { uri: string; status: string }[];
      }[]
    | null
    | undefined,
): Aria2File[] {
  if (!files || files.length === 0) return [];
  return files.map(toFile);
}

/**
 * The snapshot the backend publishes.
 *
 * It is the Go model, with the field names the backend uses.
 */
export interface RawSnapshot {
  stat: {
    download_speed: number;
    upload_speed: number;
    num_active: number;
    num_waiting: number;
    num_stopped: number;
  };
  tasks: Job[] | null;
}

/**
 * Converts a snapshot of the backend into the one the store expects.
 *
 * The backend publishes its own field names, and the store reads the
 * ones the engine reports. The tasks are converted the same way as the
 * ones of a list.
 */
export function toSnapshot(raw: RawSnapshot): TaskSnapshot {
  return {
    stat: {
      downloadSpeed: raw.stat.download_speed ?? 0,
      uploadSpeed: raw.stat.upload_speed ?? 0,
      numActive: raw.stat.num_active ?? 0,
      numWaiting: raw.stat.num_waiting ?? 0,
      numStopped: raw.stat.num_stopped ?? 0,
      // The backend does not count the finished downloads of the whole
      // history, so the ones it knows about are the answer.
      numStoppedTotal: raw.stat.num_stopped ?? 0,
    },
    tasks: toTasks(raw.tasks),
  };
}
