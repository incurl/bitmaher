/**
 * The sidecar settings.
 *
 * The download directory, the global speed limits, the choice between
 * the bundled engine and a system-installed one, and the watchdog.
 */

import { useEffect, useState } from 'react';

import type { Aria2Settings } from '../../bindings';

interface Aria2SettingsPanelProps {
  /** The settings to show. */
  settings: Aria2Settings | null;
  /** Stores the settings. */
  onSave: (settings: Aria2Settings) => void;
}

/**
 * The log levels the engine understands.
 */
const LOG_LEVELS = ['debug', 'info', 'notice', 'warn', 'error'];

/**
 * Converts a speed in bytes per second into the engine's own unit.
 *
 * The engine takes a number with a unit suffix, such as "1M" for a
 * megabyte per second. Zero means no limit.
 */
export function formatSpeedOption(bytesPerSecond: number): string {
  if (!bytesPerSecond || bytesPerSecond <= 0) {
    return '0';
  }
  if (bytesPerSecond % (1024 * 1024) === 0) {
    return `${bytesPerSecond / (1024 * 1024)}M`;
  }
  if (bytesPerSecond % 1024 === 0) {
    return `${bytesPerSecond / 1024}K`;
  }
  return String(bytesPerSecond);
}

/**
 * Reads a speed out of the engine's own unit.
 */
export function parseSpeedOption(value: string): number {
  const cleaned = value.trim().toUpperCase();
  if (!cleaned) {
    return 0;
  }

  const match = /^(\d+(?:\.\d+)?)\s*([KM])?$/.exec(cleaned);
  if (!match) {
    return 0;
  }

  const amount = Number(match[1]);
  switch (match[2]) {
    case 'M':
      return Math.round(amount * 1024 * 1024);
    case 'K':
      return Math.round(amount * 1024);
    default:
      return Math.round(amount);
  }
}

/**
 * The sidecar settings panel.
 */
export function Aria2SettingsPanel({ settings, onSave }: Aria2SettingsPanelProps) {
  const [downloadDir, setDownloadDir] = useState('');
  const [downloadSpeed, setDownloadSpeed] = useState('0');
  const [uploadSpeed, setUploadSpeed] = useState('0');
  const [useExternal, setUseExternal] = useState(false);
  const [externalPath, setExternalPath] = useState('');
  const [autoRestart, setAutoRestart] = useState(true);
  const [logLevel, setLogLevel] = useState('warn');

  // The form follows the stored settings.
  useEffect(() => {
    if (!settings) {
      return;
    }
    setDownloadDir(settings.download_dir ?? '');
    setDownloadSpeed(formatSpeedOption(settings.max_download_speed));
    setUploadSpeed(formatSpeedOption(settings.max_upload_speed));
    setUseExternal(settings.use_external_binary);
    setExternalPath(settings.external_binary_path ?? '');
    setAutoRestart(settings.auto_restart);
    setLogLevel(settings.log_level || 'warn');
  }, [settings]);

  const save = () => {
    onSave({
      download_dir: downloadDir.trim(),
      max_download_speed: parseSpeedOption(downloadSpeed),
      max_upload_speed: parseSpeedOption(uploadSpeed),
      use_external_binary: useExternal,
      external_binary_path: externalPath.trim(),
      auto_restart: autoRestart,
      log_level: logLevel,
    });
  };

  return (
    <div className="settings-body">
      <div className="field">
        <label className="field-label" htmlFor="settings-dir">
          Download folder
        </label>
        <input
          id="settings-dir"
          type="text"
          value={downloadDir}
          onChange={(event) => setDownloadDir(event.target.value)}
        />
        <span className="field-hint">Where the downloads are written to.</span>
      </div>

      <div className="field">
        <label className="field-label" htmlFor="settings-download-limit">
          Global download limit
        </label>
        <input
          id="settings-download-limit"
          type="text"
          placeholder="0 for no limit"
          value={downloadSpeed}
          onChange={(event) => setDownloadSpeed(event.target.value)}
        />
        <span className="field-hint">
          A number with a unit, such as 2M for two megabytes per second. Zero means no limit.
        </span>
      </div>

      <div className="field">
        <label className="field-label" htmlFor="settings-upload-limit">
          Global upload limit
        </label>
        <input
          id="settings-upload-limit"
          type="text"
          placeholder="0 for no limit"
          value={uploadSpeed}
          onChange={(event) => setUploadSpeed(event.target.value)}
        />
        <span className="field-hint">The limit of the BitTorrent uploads. Zero means no limit.</span>
      </div>

      <div className="field">
        <label className="field-label" htmlFor="settings-external">
          <input
            id="settings-external"
            type="checkbox"
            checked={useExternal}
            onChange={(event) => setUseExternal(event.target.checked)}
          />
          Use a system-installed aria2-next
        </label>
        {useExternal && (
          <>
            <input
              type="text"
              placeholder="/usr/bin/aria2-next"
              value={externalPath}
              onChange={(event) => setExternalPath(event.target.value)}
            />
            <span className="field-hint">The bundled engine is used when this is empty.</span>
          </>
        )}
      </div>

      <div className="field">
        <label className="field-label" htmlFor="settings-restart">
          <input
            id="settings-restart"
            type="checkbox"
            checked={autoRestart}
            onChange={(event) => setAutoRestart(event.target.checked)}
          />
          Start the engine again if it stops unexpectedly
        </label>
      </div>

      <div className="field">
        <label className="field-label" htmlFor="settings-log-level">
          Log level
        </label>
        <select
          id="settings-log-level"
          value={logLevel}
          onChange={(event) => setLogLevel(event.target.value)}
        >
          {LOG_LEVELS.map((level) => (
            <option key={level} value={level}>
              {level}
            </option>
          ))}
        </select>
      </div>

      <div className="settings-actions">
        <button type="button" className="btn btn-primary" onClick={save}>
          Save
        </button>
      </div>
    </div>
  );
}
