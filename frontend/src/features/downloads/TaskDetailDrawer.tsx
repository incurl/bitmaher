/**
 * The detail drawer.
 *
 * The panel that slides in from the right when a download is selected.
 * It shows everything the list does not have room for: the source, the
 * files and the error.
 */

import type { Download, DownloadFile } from '../../bindings';
import { formatBytes, formatSpeed, formatTimestamp } from './format';

interface TaskDetailDrawerProps {
  /** The download to show, or null when the drawer is closed. */
  download: Download | null;
  /** The files of the download. */
  files: DownloadFile[];
  /** Closes the drawer. */
  onClose: () => void;
  /** Pauses the download. */
  onPause: (id: string) => void;
  /** Resumes the download. */
  onResume: (id: string) => void;
  /** Removes the download. */
  onRemove: (id: string) => void;
}

/**
 * The detail drawer.
 */
export function TaskDetailDrawer({
  download,
  files,
  onClose,
  onPause,
  onResume,
  onRemove,
}: TaskDetailDrawerProps) {
  if (!download) {
    return null;
  }

  const isTorrent = download.kind === 'torrent' || download.kind === 'magnet';

  return (
    <aside className="drawer" aria-label="Download details">
      <header className="drawer-header">
        <span className="drawer-title" title={download.title}>
          {download.title}
        </span>
        <button type="button" className="btn btn-sm" onClick={onClose} aria-label="Close">
          ×
        </button>
      </header>

      <div className="drawer-body">
        <div className="field">
          <span className="field-label">Source</span>
          <span className="drawer-source" title={download.uri}>
            {download.uri || '—'}
          </span>
        </div>

        <div className="field">
          <span className="field-label">Location</span>
          <span className="drawer-source" title={download.dir}>
            {download.dir || '—'}
          </span>
        </div>

        <div className="field">
          <span className="field-label">Added</span>
          <span>{formatTimestamp(download.created)}</span>
        </div>

        <div className="field">
          <span className="field-label">Size</span>
          <span>
            {formatBytes(download.completed_length)} of {formatBytes(download.total_length)}
          </span>
        </div>

        <div className="field">
          <span className="field-label">Speed</span>
          <span>
            {formatSpeed(download.download_speed)} down · {formatSpeed(download.upload_speed)} up
          </span>
        </div>

        {isTorrent && (
          <div className="field">
            <span className="field-label">Peers</span>
            <span>
              {download.seeders} seeders · {download.peers} peers
            </span>
          </div>
        )}

        {download.info_hash && (
          <div className="field">
            <span className="field-label">Info hash</span>
            <span className="drawer-hash">{download.info_hash}</span>
          </div>
        )}

        {download.error_message && (
          <div className="field">
            <span className="field-label">Error</span>
            <span className="drawer-error">{download.error_message}</span>
          </div>
        )}

        {files.length > 0 && (
          <div className="field">
            <span className="field-label">Files</span>
            <ul className="drawer-files">
              {files.map((file) => (
                <li key={file.index} className="drawer-file">
                  <span className="drawer-file-name" title={file.path}>
                    {file.path.split('/').pop()}
                  </span>
                  <span className="drawer-file-size">{formatBytes(file.length)}</span>
                  {!file.selected && <span className="drawer-file-skipped">skipped</span>}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>

      <footer className="drawer-footer">
        {(download.status === 'active' || download.status === 'waiting') && (
          <button type="button" className="btn" onClick={() => onPause(download.id)}>
            Pause
          </button>
        )}
        {download.status === 'paused' && (
          <button type="button" className="btn" onClick={() => onResume(download.id)}>
            Resume
          </button>
        )}
        <button type="button" className="btn btn-danger" onClick={() => onRemove(download.id)}>
          Remove
        </button>
      </footer>
    </aside>
  );
}
