/**
 * The formatters of the download list.
 *
 * The pure logic lives here rather than in the component, so that it can
 * be unit tested on its own.
 */

/**
 * The categories of the sidebar.
 *
 * They are the four the user thinks in: everything, what is running,
 * what is finished and what went wrong.
 */
export type DownloadCategory = 'all' | 'active' | 'completed' | 'failed';

/**
 * The categories, in the order they are shown.
 */
export const DOWNLOAD_CATEGORIES: { value: DownloadCategory; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'active', label: 'Active' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
];

/**
 * Reports whether a status is one of the running ones.
 *
 * A paused download counts as active: it is not finished, and the user
 * still has something to do about it.
 */
export function isActiveStatus(status: string): boolean {
  return status === 'active' || status === 'waiting' || status === 'paused';
}

/**
 * Reports whether a status is a finished one.
 */
export function isFinishedStatus(status: string): boolean {
  return status === 'complete' || status === 'error' || status === 'removed';
}

/**
 * Reports whether a download failed.
 */
export function isFailedStatus(status: string): boolean {
  return status === 'error' || status === 'removed';
}

/**
 * Keeps the downloads of one category.
 *
 * The order is kept, so the newest stays at the top.
 */
export function filterByCategory<T extends { status: string }>(
  downloads: T[],
  category: DownloadCategory,
): T[] {
  switch (category) {
    case 'active':
      return downloads.filter((item) => isActiveStatus(item.status));
    case 'completed':
      return downloads.filter((item) => item.status === 'complete');
    case 'failed':
      return downloads.filter((item) => isFailedStatus(item.status));
    default:
      return downloads;
  }
}

/**
 * Counts the downloads of every category.
 *
 * The sidebar shows the counts, so that the user sees what is in each
 * one without clicking through them.
 */
export function countByCategory<T extends { status: string }>(
  downloads: T[],
): Record<DownloadCategory, number> {
  const counts: Record<DownloadCategory, number> = {
    all: downloads.length,
    active: 0,
    completed: 0,
    failed: 0,
  };

  for (const item of downloads) {
    if (isActiveStatus(item.status)) {
      counts.active += 1;
    }
    if (item.status === 'complete') {
      counts.completed += 1;
    }
    if (isFailedStatus(item.status)) {
      counts.failed += 1;
    }
  }

  return counts;
}

/**
 * Formats a number of bytes for display.
 *
 * The units are the binary ones, because that is what the file sizes
 * of an operating system are shown in.
 */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0 B';
  }

  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
  let value = bytes;
  let unit = 0;

  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }

  // The bytes are whole, the rest has one decimal.
  const rounded = unit === 0 ? Math.round(value) : Math.round(value * 10) / 10;
  return `${rounded} ${units[unit]}`;
}

/**
 * Formats a speed for display.
 */
export function formatSpeed(bytesPerSecond: number): string {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) {
    return '—';
  }
  return `${formatBytes(bytesPerSecond)}/s`;
}

/**
 * Formats the completion ratio as a percentage.
 */
export function formatPercent(progress: number): string {
  if (!Number.isFinite(progress) || progress <= 0) {
    return '0%';
  }
  const percent = Math.round(Math.min(1, progress) * 100);
  return `${percent}%`;
}

/**
 * Formats the remaining time of a download.
 *
 * The estimate is made from the speed the download is going at now, so
 * it moves as the transfer speeds up and slows down.
 */
export function formatEta(
  completed: number,
  total: number,
  bytesPerSecond: number,
): string {
  if (!Number.isFinite(total) || total <= 0 || total <= completed) {
    return '—';
  }
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) {
    return '—';
  }

  const seconds = Math.round((total - completed) / bytesPerSecond);
  if (seconds < 60) {
    return `${seconds}s`;
  }

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m ${seconds % 60}s`;
  }

  const hours = Math.floor(minutes / 60);
  return `${hours}h ${minutes % 60}m`;
}

/**
 * Formats a timestamp as a short date and time.
 *
 * The list shows when a download was added, so the format is short
 * enough to sit in a column.
 */
export function formatTimestamp(timestamp: string): string {
  if (!timestamp) {
    return '';
  }

  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) {
    return '';
  }

  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Reports whether a download can be paused.
 */
export function canPause(status: string): boolean {
  return status === 'active' || status === 'waiting';
}

/**
 * Reports whether a download can be resumed.
 */
export function canResume(status: string): boolean {
  return status === 'paused';
}

/**
 * Reports whether a download can be removed.
 *
 * A running download has to be paused first, so the button is offered
 * for the downloads that have come to rest.
 */
export function canRemove(status: string): boolean {
  return isFinishedStatus(status);
}
