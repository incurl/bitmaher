/**
 * The file selection dialog.
 *
 * It shows the files of a torrent and lets the user choose the ones to
 * download.
 *
 * The engine only accepts the choice while the download is paused, so
 * the backend pauses it, changes the selection and resumes it again.
 */

import { useEffect, useState } from 'react';

import type { DownloadFile } from '../../bindings';
import { formatBytes } from './format';

interface FileSelectionDialogProps {
  /** Whether the dialog is shown. */
  open: boolean;
  /** The files of the torrent. */
  files: DownloadFile[];
  /** The name of the download. */
  title: string;
  /** Closes the dialog. */
  onClose: () => void;
  /** Applies the choice. */
  onApply: (indexes: number[]) => void;
  /** Whether the backend is working. */
  busy: boolean;
}

/**
 * The file selection dialog.
 */
export function FileSelectionDialog({
  open,
  files,
  title,
  onClose,
  onApply,
  busy,
}: FileSelectionDialogProps) {
  const [selected, setSelected] = useState<Set<number>>(new Set());

  // The selection starts with everything the engine has chosen.
  useEffect(() => {
    if (!open) {
      return;
    }
    const chosen = new Set<number>();
    for (const file of files) {
      if (file.selected) {
        chosen.add(file.index);
      }
    }
    setSelected(chosen);
  }, [open, files]);

  if (!open) {
    return null;
  }

  const toggle = (index: number) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(index)) {
        next.delete(index);
      } else {
        next.add(index);
      }
      return next;
    });
  };

  const all = files.map((file) => file.index);
  const total = files
    .filter((file) => selected.has(file.index))
    .reduce((sum, file) => sum + file.length, 0);

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-label="Choose files">
      <div className="modal modal-wide">
        <div className="modal-header">
          <span>Choose the files of {title}</span>
          <button type="button" className="btn btn-sm" onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>

        <div className="modal-body">
          {files.length === 0 ? (
            <p className="text-dim">The files of this download are not known yet.</p>
          ) : (
            <>
              <div className="file-select-actions">
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => setSelected(new Set(all))}
                >
                  Select all
                </button>
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => setSelected(new Set())}
                >
                  Select none
                </button>
                <span className="file-select-size">
                  {selected.size} of {files.length} files · {formatBytes(total)}
                </span>
              </div>

              <ul className="file-select-list">
                {files.map((file) => (
                  <li key={file.index}>
                    <label className="file-select-item">
                      <input
                        type="checkbox"
                        checked={selected.has(file.index)}
                        onChange={() => toggle(file.index)}
                      />
                      <span className="file-select-name" title={file.path}>
                        {file.path.split('/').pop()}
                      </span>
                      <span className="file-select-size">{formatBytes(file.length)}</span>
                    </label>
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>

        <div className="modal-footer">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => onApply(Array.from(selected).sort((a, b) => a - b))}
            disabled={busy || selected.size === 0}
          >
            {busy ? 'Applying…' : 'Apply'}
          </button>
        </div>
      </div>
    </div>
  );
}
