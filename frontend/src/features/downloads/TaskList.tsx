/**
 * The download list.
 *
 * The main table of the window: one row per download, with its
 * progress, its speed and the actions that apply to it.
 *
 * The list is the heart of the interface, so it is deliberately plain:
 * a table of rows, and the detail of the selected one in the drawer.
 */

import type { Download } from '../../bindings';
import {
  canPause,
  canRemove,
  canResume,
  formatBytes,
  formatEta,
  formatPercent,
  formatSpeed,
} from './format';

interface TaskListProps {
  /** The downloads to show. */
  downloads: Download[];
  /** The download whose detail is open, if any. */
  selectedId: string | null;
  /** Opens the detail of a download. */
  onSelect: (id: string) => void;
  /** Pauses a download. */
  onPause: (id: string) => void;
  /** Resumes a download. */
  onResume: (id: string) => void;
  /** Removes a download. */
  onRemove: (id: string) => void;
  /** The message shown when there is nothing to show. */
  emptyMessage: string;
}

/**
 * The status of a download, as the dot shows it.
 */
function statusLabel(status: string): string {
  switch (status) {
    case 'active':
      return 'Downloading';
    case 'waiting':
      return 'Queued';
    case 'paused':
      return 'Paused';
    case 'complete':
      return 'Finished';
    case 'error':
      return 'Failed';
    case 'removed':
      return 'Removed';
    default:
      return status;
  }
}

/**
 * The download list.
 */
export function TaskList({
  downloads,
  selectedId,
  onSelect,
  onPause,
  onResume,
  onRemove,
  emptyMessage,
}: TaskListProps) {
  if (downloads.length === 0) {
    return <div className="task-empty">{emptyMessage}</div>;
  }

  return (
    <div className="task-table" role="table" aria-label="Downloads">
      <div className="task-row task-head" role="row">
        <span role="columnheader">Name</span>
        <span role="columnheader">Size</span>
        <span role="columnheader">Progress</span>
        <span role="columnheader">Speed</span>
        <span role="columnheader">Peers</span>
        <span role="columnheader" className="task-actions-head">
          Actions
        </span>
      </div>

      {downloads.map((download) => {
        const selected = download.id === selectedId;
        const isTorrent = download.kind === 'torrent' || download.kind === 'magnet';

        return (
          <div
            key={download.id}
            role="row"
            className={`task-row task-status-${download.status}${selected ? ' task-selected' : ''}`}
            onClick={() => onSelect(download.id)}
          >
            <span className="task-name" role="cell" title={download.title}>
              <span className={`task-dot task-dot-${download.kind}`} aria-hidden="true" />
              <span className="task-title">{download.title}</span>
              <span className="task-status">{statusLabel(download.status)}</span>
            </span>

            <span className="task-size" role="cell">
              {download.total_length > 0
                ? `${formatBytes(download.completed_length)} / ${formatBytes(download.total_length)}`
                : formatBytes(download.completed_length)}
            </span>

            <span className="task-progress" role="cell">
              <span className="task-progress-bar">
                <span
                  className="task-progress-fill"
                  style={{ width: `${Math.round(download.progress * 100)}%` }}
                />
              </span>
              <span className="task-progress-text">
                {formatPercent(download.progress)}
                {download.status === 'active' && (
                  <span className="task-eta">
                    {' · '}
                    {formatEta(
                      download.completed_length,
                      download.total_length,
                      download.download_speed,
                    )}
                  </span>
                )}
              </span>
            </span>

            <span className="task-speed" role="cell">
              {download.status === 'active' ? formatSpeed(download.download_speed) : '—'}
            </span>

            <span className="task-peers" role="cell">
              {isTorrent ? `${download.seeders} / ${download.peers}` : download.connections.toString()}
            </span>

            <span className="task-actions" role="cell">
              {canPause(download.status) && (
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={(event) => {
                    event.stopPropagation();
                    onPause(download.id);
                  }}
                  title="Pause this download"
                >
                  Pause
                </button>
              )}
              {canResume(download.status) && (
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={(event) => {
                    event.stopPropagation();
                    onResume(download.id);
                  }}
                  title="Resume this download"
                >
                  Resume
                </button>
              )}
              {canRemove(download.status) && (
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={(event) => {
                    event.stopPropagation();
                    onRemove(download.id);
                  }}
                  title="Remove this download"
                >
                  ×
                </button>
              )}
            </span>
          </div>
        );
      })}
    </div>
  );
}
