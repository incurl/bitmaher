/**
 * The BitMaher application shell.
 *
 * The layout is the clean, low-friction one of a download manager: a
 * sidebar with the categories, a table of the downloads in the middle
 * and a detail drawer on the right.
 *
 * The debug console sits at the bottom of the window. It is hidden
 * until it is asked for, with Ctrl+Shift+I or with the button in the
 * header, so that it does not take the room of the panels above it.
 *
 * All backend access goes through the Wails bindings; the frontend never
 * talks to PocketBase or to the download engine directly.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Events } from '@wailsio/runtime';

import '../styles.css';

import * as api from '../bindings';
import type { Download, DownloadFile } from '../bindings';
import { applyZoom } from '../ui/zoom';
import { isConsoleShortcut } from '../ui/shortcuts';
import { ConsolePanel, countErrors } from '../features/console/ConsolePanel';
import { SettingsDialog } from '../features/settings/SettingsDialog';
import { AddTaskModal } from '../features/downloads/AddTaskModal';
import { FileSelectionDialog } from '../features/downloads/FileSelectionDialog';
import { Sidebar } from '../features/downloads/Sidebar';
import { TaskDetailDrawer } from '../features/downloads/TaskDetailDrawer';
import { TaskList } from '../features/downloads/TaskList';
import { filterByCategory, formatSpeed, isFinishedStatus } from '../features/downloads/format';
import type { DownloadCategory } from '../features/downloads/format';

interface Toast {
  message: string;
  kind: 'info' | 'error' | 'success';
}

/**
 * The poll interval of the download list.
 *
 * The engine does not push its updates: it answers a question, and the
 * application has to ask. The WebSocket transport accepts a connection
 * but never sends anything on it, so polling is the only way to learn
 * that a download moved.
 *
 * The interval is short enough to feel live and long enough not to
 * hammer the engine.
 */
const SYNC_INTERVAL = 1000;

/**
 * The application root.
 */
function App() {
  const queryClient = useQueryClient();

  const [settingsOpen, setSettingsOpen] = useState(false);
  const [consoleOpen, setConsoleOpen] = useState(false);
  const [toast, setToast] = useState<Toast | null>(null);

  const [category, setCategory] = useState<DownloadCategory>('all');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [fileSelectFor, setFileSelectFor] = useState<Download | null>(null);
  const [files, setFiles] = useState<DownloadFile[]>([]);

  // The toast timer is kept so that a second toast replaces the first
  // instead of being dismissed by the first one's timer.
  const toastTimer = useRef<number | null>(null);

  const notify = useCallback((message: string, kind: Toast['kind'] = 'info') => {
    setToast({ message, kind });

    if (toastTimer.current !== null) {
      window.clearTimeout(toastTimer.current);
    }
    toastTimer.current = window.setTimeout(() => {
      toastTimer.current = null;
      setToast(null);
    }, 4000);
  }, []);

  // A pending timer must not outlive the component.
  useEffect(() => {
    return () => {
      if (toastTimer.current !== null) {
        window.clearTimeout(toastTimer.current);
      }
    };
  }, []);

  // --- Data ---------------------------------------------------------

  const settingsQuery = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.getSettings(),
  });

  // The download list. The poll is the live update: the engine pushes
  // nothing, so this is what keeps the list in step with it.
  const downloadsQuery = useQuery({
    queryKey: ['downloads'],
    queryFn: async () => {
      // The sync reads the engine and updates the records, so the
      // list is the stored state and the engine is the truth.
      await api.syncDownloads();
      return api.listDownloads(200);
    },
    refetchInterval: SYNC_INTERVAL,
  });

  const statQuery = useQuery({
    queryKey: ['download-stat'],
    queryFn: () => api.downloadStat(),
    refetchInterval: SYNC_INTERVAL * 2,
  });

  const consoleQuery = useQuery({
    queryKey: ['console'],
    queryFn: () => api.listConsoleEntries(),
    refetchInterval: 5000,
  });

  // --- Live updates -------------------------------------------------

  useEffect(() => {
    const unsubscribe = Events.On(api.EVENT_CONSOLE_ENTRY, () => {
      void queryClient.invalidateQueries({ queryKey: ['console'] });
    });
    return () => {
      unsubscribe();
    };
  }, [queryClient]);

  // The debug console is shown and hidden with Ctrl+Shift+I. The
  // shortcut is handled in the page, because the webview has no menu
  // bar of its own.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!isConsoleShortcut(event)) {
        return;
      }
      // The webview would otherwise open its own inspector.
      event.preventDefault();
      setConsoleOpen((value) => !value);
    };

    window.addEventListener('keydown', onKeyDown);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
    };
  }, []);

  // --- Actions ------------------------------------------------------

  const addMutation = useMutation({
    mutationFn: (request: api.AddDownloadRequest) => api.addDownload(request),
    onSuccess: (download: Download) => {
      void queryClient.invalidateQueries({ queryKey: ['downloads'] });
      setAddOpen(false);
      notify(`"${download.title}" was started.`, 'success');
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const pauseMutation = useMutation({
    mutationFn: (id: string) => api.pauseDownload(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['downloads'] });
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const resumeMutation = useMutation({
    mutationFn: (id: string) => api.resumeDownload(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['downloads'] });
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const removeMutation = useMutation({
    mutationFn: (id: string) => api.removeDownload(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['downloads'] });
      setSelectedId(null);
      notify('The download was removed.', 'success');
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const purgeMutation = useMutation({
    mutationFn: () => api.purgeDownloads(),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['downloads'] });
      notify('The finished downloads were cleared.', 'success');
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const selectFilesMutation = useMutation({
    mutationFn: ({ id, indexes }: { id: string; indexes: number[] }) =>
      api.selectDownloadFiles(id, indexes),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['downloads'] });
      setFileSelectFor(null);
      notify('The files were chosen.', 'success');
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const clearConsoleMutation = useMutation({
    mutationFn: () => api.clearConsoleEntries(),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['console'] });
    },
  });

  // The credentials and the interface preferences are saved through
  // mutations, so that a failure is reported instead of becoming an
  // unhandled rejection.
  const saveCredentialsMutation = useMutation({
    mutationFn: ({ apiKey, baseUrl }: { apiKey: string; baseUrl: string }) =>
      api.saveCredentials(apiKey, baseUrl),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['settings'] });
      notify('The settings were saved.', 'success');
      setSettingsOpen(false);
    },
    onError: (error: Error) => {
      notify(`The settings could not be saved: ${error.message}`, 'error');
    },
  });

  const saveUIMutation = useMutation({
    mutationFn: (ui: { zoom: number; theme: string }) => api.saveUI(ui),
    onSuccess: (_data, ui) => {
      void queryClient.invalidateQueries({ queryKey: ['settings'] });
      applyZoom(ui.zoom);
    },
    onError: (error: Error) => {
      notify(`The zoom could not be saved: ${error.message}`, 'error');
    },
  });

  const saveAria2Mutation = useMutation({
    mutationFn: (settings: api.Aria2Settings) => api.saveAria2(settings),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['settings'] });
      notify('The engine settings were saved. Restart the application to apply them.', 'success');
    },
    onError: (error: Error) => {
      notify(`The engine settings could not be saved: ${error.message}`, 'error');
    },
  });

  // The zoom is restored as soon as the settings are known.
  useEffect(() => {
    if (settingsQuery.data) {
      applyZoom(settingsQuery.data.ui.zoom);
    }
  }, [settingsQuery.data]);

  const downloads = downloadsQuery.data ?? [];
  const entries = consoleQuery.data ?? [];
  const consoleErrors = countErrors(entries);

  // The list shows the downloads of the chosen category.
  const visible = useMemo(() => filterByCategory(downloads, category), [downloads, category]);

  // The drawer shows the download that is selected.
  const selected = useMemo(
    () => downloads.find((item) => item.id === selectedId) ?? null,
    [downloads, selectedId],
  );

  // The files of the selected download are fetched when it changes.
  useEffect(() => {
    if (!selected) {
      setFiles([]);
      return;
    }

    let cancelled = false;
    void api
      .downloadFiles(selected.id)
      .then((list) => {
        if (!cancelled) {
          setFiles(list);
        }
      })
      .catch(() => {
        // A download that the engine no longer knows about has no
        // files to show. That is not worth a message.
        if (!cancelled) {
          setFiles([]);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [selected]);

  return (
    <div className="app app-downloads">
      <header className="app-header">
        <span className="app-brand">
          BitMaher
          <small>Wails v3 · PocketBase · aria2-next</small>
        </span>

        <span className="app-header-spacer" />

        <span className="app-speed" title="The speed of all the downloads together">
          {formatSpeed(statQuery.data?.download_speed ?? 0)} down ·{' '}
          {formatSpeed(statQuery.data?.upload_speed ?? 0)} up
        </span>

        <button
          type="button"
          className={`btn btn-sm${consoleOpen ? ' btn-active' : ''}`}
          onClick={() => setConsoleOpen((value) => !value)}
          title="Show or hide the debug console (Ctrl+Shift+I)"
          aria-pressed={consoleOpen}
        >
          Console
          {consoleErrors > 0 && (
            <span className="console-error-count"> · {consoleErrors}</span>
          )}
        </button>

        <button type="button" className="btn btn-sm" onClick={() => setSettingsOpen(true)}>
          Settings
        </button>
      </header>

      <Sidebar
        category={category}
        onCategoryChange={setCategory}
        downloads={downloads}
        onAdd={() => setAddOpen(true)}
        onPurge={() => purgeMutation.mutate()}
        canPurge={downloads.some((item) => isFinishedStatus(item.status))}
      />

      <main className="app-canvas">
        <TaskList
          downloads={visible}
          selectedId={selectedId}
          onSelect={(id) => setSelectedId((current) => (current === id ? null : id))}
          onPause={(id) => pauseMutation.mutate(id)}
          onResume={(id) => resumeMutation.mutate(id)}
          onRemove={(id) => removeMutation.mutate(id)}
          emptyMessage={
            category === 'all'
              ? 'No downloads yet. Add one to get started.'
              : 'Nothing in this category.'
          }
        />
      </main>

      {selected && (
        <TaskDetailDrawer
          download={selected}
          files={files}
          onClose={() => setSelectedId(null)}
          onPause={(id) => pauseMutation.mutate(id)}
          onResume={(id) => resumeMutation.mutate(id)}
          onRemove={(id) => removeMutation.mutate(id)}
        />
      )}

      {consoleOpen && (
        <ConsolePanel
          entries={entries}
          onClear={() => clearConsoleMutation.mutate()}
        />
      )}

      <AddTaskModal
        open={addOpen}
        defaultDir={settingsQuery.data?.aria2.download_dir ?? ''}
        onClose={() => setAddOpen(false)}
        onSubmit={(request) => addMutation.mutate(request)}
        busy={addMutation.isPending}
      />

      <FileSelectionDialog
        open={fileSelectFor !== null}
        files={files}
        title={fileSelectFor?.title ?? ''}
        onClose={() => setFileSelectFor(null)}
        onApply={(indexes) => {
          if (fileSelectFor) {
            selectFilesMutation.mutate({ id: fileSelectFor.id, indexes });
          }
        }}
        busy={selectFilesMutation.isPending}
      />

      {settingsOpen && (
        <SettingsDialog
          open={settingsOpen}
          settings={settingsQuery.data ?? null}
          onClose={() => setSettingsOpen(false)}
          onSave={(apiKey, baseUrl) => {
            saveCredentialsMutation.mutate({ apiKey, baseUrl });
          }}
          onSaveUI={(ui) => {
            saveUIMutation.mutate(ui);
          }}
          onSaveAria2={(engineSettings) => {
            saveAria2Mutation.mutate(engineSettings);
          }}
        />
      )}

      {toast && (
        <div className={`toast toast-${toast.kind}`} role="status">
          {toast.message}
        </div>
      )}
    </div>
  );
}

export default App;

