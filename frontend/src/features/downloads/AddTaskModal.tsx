/**
 * The add-task dialog.
 *
 * Three tabs, one per kind of download: a plain link, a magnet link and
 * a .torrent file. The advanced section carries the engine's own
 * options.
 *
 * The torrent file is read in the browser and sent as base64, because
 * the Wails bindings carry JSON. The backend stores it in a PocketBase
 * file field.
 */

import { useEffect, useRef, useState } from 'react';

import type { AddDownloadRequest } from '../../bindings';

type Tab = 'link' | 'magnet' | 'torrent';

interface AddTaskModalProps {
  /** Whether the dialog is shown. */
  open: boolean;
  /** The directory the downloads are written to. */
  defaultDir: string;
  /** Closes the dialog. */
  onClose: () => void;
  /** Starts the download. */
  onSubmit: (request: AddDownloadRequest) => void;
  /** Whether the dialog is waiting for the backend. */
  busy: boolean;
}

/**
 * The advanced options the dialog offers.
 *
 * They are the engine's own options, under the names the engine uses.
 * The ones that are not in this list are not offered in phase one:
 * ED2K, the HLS decryption and the automatic muxing are all left out.
 */
const ADVANCED_OPTIONS: { name: string; label: string; hint: string }[] = [
  {
    name: 'max-connection-per-server',
    label: 'Connections per server',
    hint: 'How many connections one server may serve. The default is 6.',
  },
  {
    name: 'split',
    label: 'Splits',
    hint: 'How many connections one download may use. The default is 6.',
  },
  {
    name: 'min-split-size',
    label: 'Minimum split size',
    hint: 'The smallest piece a file is downloaded in, such as 20M.',
  },
  {
    name: 'max-tries',
    label: 'Retries',
    hint: 'How many times a failed transfer is tried again. The default is 5.',
  },
  {
    name: 'timeout',
    label: 'Timeout',
    hint: 'How long a stalled transfer is waited out, in seconds.',
  },
  {
    name: 'user-agent',
    label: 'User agent',
    hint: 'The name the server is told.',
  },
];

/**
 * The add-task dialog.
 */
export function AddTaskModal({
  open,
  defaultDir,
  onClose,
  onSubmit,
  busy,
}: AddTaskModalProps) {
  const [tab, setTab] = useState<Tab>('link');
  const [uri, setUri] = useState('');
  const [dir, setDir] = useState(defaultDir);
  const [out, setOut] = useState('');
  const [torrentName, setTorrentName] = useState('');
  const [torrentData, setTorrentData] = useState('');
  const [options, setOptions] = useState<Record<string, string>>({});
  const [error, setError] = useState('');
  const fileInput = useRef<HTMLInputElement>(null);

  // The dialog starts from the default directory every time it opens.
  useEffect(() => {
    if (open) {
      setDir(defaultDir);
      setError('');
    }
  }, [open, defaultDir]);

  if (!open) {
    return null;
  }

  const readTorrent = async (file: File) => {
    setError('');
    try {
      const buffer = await file.arrayBuffer();
      const bytes = new Uint8Array(buffer);
      let binary = '';
      for (let index = 0; index < bytes.length; index += 1) {
        binary += String.fromCharCode(bytes[index]);
      }
      setTorrentData(btoa(binary));
      setTorrentName(file.name);
    } catch {
      setError('The torrent file could not be read.');
    }
  };

  const submit = () => {
    setError('');

    if (tab === 'torrent') {
      if (!torrentData) {
        setError('Choose a .torrent file first.');
        return;
      }
      onSubmit({
        kind: 'torrent',
        uri: '',
        torrent_name: torrentName,
        torrent_data: torrentData,
        dir,
        out: '',
        selected_files: [],
        options,
        paused: false,
      });
      return;
    }

    if (!uri.trim()) {
      setError('Enter a link.');
      return;
    }

    onSubmit({
      kind: tab === 'magnet' ? 'magnet' : 'http',
      uri: uri.trim(),
      torrent_name: '',
      torrent_data: '',
      dir,
      out: out.trim(),
      selected_files: [],
      options,
      paused: false,
    });
  };

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-label="Add download">
      <div className="modal modal-wide">
        <div className="modal-header">
          <span>Add download</span>
          <button type="button" className="btn btn-sm" onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>

        <div className="modal-tabs" role="tablist">
          {(
            [
              { value: 'link', label: 'Link' },
              { value: 'magnet', label: 'Magnet' },
              { value: 'torrent', label: 'Torrent' },
            ] as { value: Tab; label: string }[]
          ).map((item) => (
            <button
              key={item.value}
              type="button"
              role="tab"
              aria-selected={tab === item.value}
              className={`modal-tab${tab === item.value ? ' modal-tab-active' : ''}`}
              onClick={() => setTab(item.value)}
            >
              {item.label}
            </button>
          ))}
        </div>

        <div className="modal-body">
          {tab === 'link' && (
            <div className="field">
              <label className="field-label" htmlFor="add-uri">
                Link
              </label>
              <input
                id="add-uri"
                type="text"
                placeholder="https://example.com/file.zip"
                value={uri}
                onChange={(event) => setUri(event.target.value)}
              />
            </div>
          )}

          {tab === 'magnet' && (
            <div className="field">
              <label className="field-label" htmlFor="add-magnet">
                Magnet link
              </label>
              <input
                id="add-magnet"
                type="text"
                placeholder="magnet:?xt=urn:btih:..."
                value={uri}
                onChange={(event) => setUri(event.target.value)}
              />
              <span className="field-hint">
                The files of a magnet link are only known once the metadata has been fetched, so
                they cannot be chosen before it starts.
              </span>
            </div>
          )}

          {tab === 'torrent' && (
            <div className="field">
              <span className="field-label">Torrent file</span>
              <input
                ref={fileInput}
                type="file"
                accept=".torrent,application/x-bittorrent"
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  if (file) {
                    void readTorrent(file);
                  }
                }}
              />
              <span className="field-hint">
                {torrentName
                  ? `${torrentName} is ready. The files can be chosen once it has started.`
                  : 'Choose a .torrent file. The files inside it can be chosen once it has started.'}
              </span>
            </div>
          )}

          <div className="field">
            <label className="field-label" htmlFor="add-dir">
              Save to
            </label>
            <input
              id="add-dir"
              type="text"
              value={dir}
              onChange={(event) => setDir(event.target.value)}
            />
          </div>

          {tab === 'link' && (
            <div className="field">
              <label className="field-label" htmlFor="add-out">
                File name
              </label>
              <input
                id="add-out"
                type="text"
                placeholder="Leave empty to use the name of the link"
                value={out}
                onChange={(event) => setOut(event.target.value)}
              />
            </div>
          )}

          <details className="advanced">
            <summary>Advanced</summary>
            <div className="advanced-body">
              {ADVANCED_OPTIONS.map((option) => (
                <div className="field" key={option.name}>
                  <label className="field-label" htmlFor={`opt-${option.name}`}>
                    {option.label}
                  </label>
                  <input
                    id={`opt-${option.name}`}
                    type="text"
                    placeholder={option.hint}
                    value={options[option.name] ?? ''}
                    onChange={(event) =>
                      setOptions((current) => {
                        const next = { ...current };
                        if (event.target.value === '') {
                          delete next[option.name];
                        } else {
                          next[option.name] = event.target.value;
                        }
                        return next;
                      })
                    }
                  />
                </div>
              ))}
            </div>
          </details>

          {error && <p className="modal-error">{error}</p>}
        </div>

        <div className="modal-footer">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button type="button" className="btn btn-primary" onClick={submit} disabled={busy}>
            {busy ? 'Starting…' : 'Start'}
          </button>
        </div>
      </div>
    </div>
  );
}
