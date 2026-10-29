/**
 * @fileoverview Application entry point: mounts Vue, initializes i18n, aria2 engine, and IPC listeners.
 * The add-task and floating windows load the same entry and mount only their
 * own content.
 */
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import { i18n, loadLocale } from '@/composables/useLocale'
import { isSupportedLocale, SUPPORTED_LOCALES, type SupportedLocale } from '@shared/localeCatalog'
import { setI18nLocale } from '@shared/utils/i18n'
import { usePreferenceStore } from './stores/preference'
import { useTaskStore } from './stores/task'
import { useAppStore } from './stores/app'
import { useHistoryStore } from './stores/history'
import { useDatabaseStore } from './stores/database'
import { useEngineStore } from './stores/engine'
import aria2Api from './api/aria2'
import { DEFAULT_TRACKER_SOURCE, PROXY_SCOPES } from '@shared/constants'
import { convertTrackerDataToLine, convertTrackerDataToComma } from '@shared/utils/tracker'
import { logger } from '@shared/logger'
import { getErrorMessage } from '@shared/utils/errorMessage'
import { resolveAppProxyUrl } from '@shared/utils/proxy'
import { checkSyncDue } from '@shared/utils/syncSchedule'
import type { AppConfig, TauriUpdate } from '@shared/types'
import App from './App.vue'
import './styles/tokens.css'
import './styles/base.css'
import './styles/transitions.css'
import './styles/preferences.css'
import './styles/naive-overrides.css'
import './styles/reduced-motion.css'

import { getLocale } from 'tauri-plugin-locale-api'
import { getCurrentWindow } from '@tauri-apps/api/window'
import { resolveSystemLocale } from '@shared/utils/locale'

// BitMaher runs in a single window: the add-task dialog and the task
// list share it, and there is no float window.
const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(i18n)

// ── Global error boundary — catch all uncaught exceptions to log file ──
// Register before preference hydration so startup failures do not disappear
// before Vue mounts and the app-level handler becomes active.
window.addEventListener('error', (e) => {
  logger.error('GlobalError', e.error ?? e.message)
})
window.addEventListener('unhandledrejection', (e) => {
  logger.error('UnhandledRejection', e.reason)
})
app.config.errorHandler = (err) => {
  logger.error('VueError', err)
}

// ── Production guard: suppress browser default context menu ─────────
// In dev mode, keep the context menu for DevTools / Inspect Element.
// Industry standard for Tauri/Electron desktop apps (Discord, Slack, VS Code).
if (import.meta.env.PROD) {
  document.addEventListener('contextmenu', (e) => e.preventDefault())
}

/** Resolves the persisted or system locale and loads its messages. */
async function applyStartupLocale(): Promise<void> {
  const preferenceStore = usePreferenceStore()
  const storedLocale = preferenceStore.locale
  let resolvedLocale: SupportedLocale

  if (!storedLocale || storedLocale === 'auto') {
    // First install (empty/auto) or explicit Follow System mode:
    // detect the OS locale and resolve to the closest available match.
    try {
      const raw = (await getLocale()) || 'en-US'
      resolvedLocale = resolveSystemLocale(raw, SUPPORTED_LOCALES)
    } catch (e) {
      logger.debug('main.locale', e)
      resolvedLocale = 'en-US'
    }

    if (!storedLocale) {
      // Legacy first-install path (locale was ''): persist 'auto' so
      // subsequent launches continue to follow the system language.
      preferenceStore.updatePreference({ locale: 'auto' })
      preferenceStore.savePreference()
    }
    // When storedLocale is already 'auto', we intentionally do NOT
    // overwrite it — the config stays 'auto' across restarts.
  } else {
    // Explicit locale chosen by the user (e.g. 'zh-CN', 'ja').
    resolvedLocale = isSupportedLocale(storedLocale) ? storedLocale : 'en-US'
  }

  // Apply resolved locale to vue-i18n and expose it on the store
  // so downstream consumers (direction, General.vue) can read it.
  // Locale messages are lazily loaded — only en-US ships in the main bundle.
  await loadLocale(resolvedLocale)
  setI18nLocale(i18n, resolvedLocale)
}

// ── Window initialization ───────────────────────────────────────────

{
  const preferenceStore = usePreferenceStore()
  const taskStore = useTaskStore()
  const appStore = useAppStore()
  const engineStore = useEngineStore()
  const historyStore = useHistoryStore()

  /**
   * Looks for a new version.
   *
   * BitMaher has no updater: the release page is opened by hand. The
   * function keeps the preference panel working, which shows the
   * version of the application.
   */
  async function autoCheckForUpdate() {
    const config = preferenceStore.config
    if (config.autoCheckUpdate === false) return

    try {
      const { invoke } = await import('@tauri-apps/api/core')
      const update = await invoke<TauriUpdate | null>('check_for_update', {})
      if (update) {
        appStore.pendingUpdate = update;
      }
    } catch (e) {
      logger.debug('Updater', 'update_check_failed', { reason: getErrorMessage(e) })
    }
  }

  async function syncBtTrackersIfDue(startup: boolean) {
    const config = preferenceStore.config
    if (
      !checkSyncDue({
        enabled: !!config.btTrackerAutoSync,
        intervalHours: Number(config.btTrackerSyncIntervalHours),
        lastSyncTime: Number(config.lastSyncTrackerTime),
        now: Date.now(),
        startup,
      })
    ) {
      return
    }

    const sources = config.trackerSource?.length ? config.trackerSource : DEFAULT_TRACKER_SOURCE
    try {
      const result = await preferenceStore.fetchBtTracker(sources)
      const text = convertTrackerDataToLine(result.data)
      if (!text) return

      const comma = convertTrackerDataToComma(result.data)
      await preferenceStore.updateAndSave({
        btTracker: comma,
        lastSyncTrackerTime: Date.now(),
      })

      const { invoke } = await import('@tauri-apps/api/core')
      const { buildSystemConfigFromAppConfig } = await import('@shared/utils/systemConfig')
      await invoke('replace_system_config', {
        config: buildSystemConfigFromAppConfig(preferenceStore.config, preferenceStore.config.dir),
      })
      if (engineStore.isReady) {
        await aria2Api.changeGlobalOption({ 'bt-tracker': comma } as Partial<AppConfig>)
      }
      logger.info('Tracker', `Auto-synced: ${result.data.length}/${sources.length} source(s) succeeded`)
    } catch (e) {
      logger.debug('Tracker', 'auto-sync failed: ' + (e as Error).message)
    }
  }

  async function syncEd2kBootstrapIfDue(startup: boolean) {
    const config = preferenceStore.config
    try {
      const { invoke } = await import('@tauri-apps/api/core')
      const status = await invoke<{ serverMetModified: number | null; nodesDatModified: number | null }>(
        'get_ed2k_bootstrap_status',
      )
      const lastSync = Math.max(status.serverMetModified ?? 0, status.nodesDatModified ?? 0)
      if (
        !checkSyncDue({
          enabled: !!config.ed2kBootstrapAutoSync,
          intervalHours: Number(config.ed2kBootstrapSyncIntervalHours),
          lastSyncTime: lastSync,
          now: Date.now(),
          startup,
        })
      ) {
        return
      }

      await invoke('sync_ed2k_bootstrap_files', {
        serverMetUrl: config.ed2kServerMetUrl,
        nodesDatUrl: config.ed2kNodesDatUrl,
        proxy: resolveAppProxyUrl(config.proxy, PROXY_SCOPES.UPDATE_TRACKERS) ?? undefined,
      })
      logger.info('ED2K', 'Bootstrap files auto-synced')
    } catch (e) {
      logger.debug('ED2K.bootstrapAutoSync', e)
    }
  }

  function syncNetworkSourcesIfDue(startup: boolean) {
    void syncBtTrackersIfDue(startup)
    void syncEd2kBootstrapIfDue(startup)
  }

  // ---------------------------------------------------------------------------
  // Startup orchestration
  //
  // The chain is split into phases that run as parallel as possible so the
  // window appears almost instantly while the engine boots in the background.
  //
  //  Phase 1 (critical path)   – loadPreference → locale → window.show()
  //  Phase 2 (engine, async)   – rpcSecret → save config → EngineSupervisor
  //  Phase 3 (non-critical)    – autostart, protocol sync (parallel)
  //  Phase 4 (deferred)        – update check, tracker sync, FS warmup,
  //                              clipboard monitor
  // ---------------------------------------------------------------------------

  /** Persists the runtime configuration and delegates startup to EngineSupervisor. */
  /**
   * Reads the state of the engine.
   *
   * BitMaher's backend owns the sidecar: it starts it, keeps it alive
   * and reads its state. The frontend only learns whether it is up.
   */
  async function initEngine(): Promise<boolean> {
    try {
      const snapshot = await engineStore.ensureRunning('startup')
      logger.info('Engine', `the backend reported the engine as ${snapshot.phase}`)
      return snapshot.phase === 'running'
    } catch (e) {
      logger.error('Engine', getErrorMessage(e))
      return false
    }
  }

  async function bootstrapMainWindow(): Promise<void> {
    // ── Phase 1: critical path → window visible ASAP ──────────────────────
    await preferenceStore.loadPreference()

    await applyStartupLocale()

    // Mount only after preference + locale hydration so root-level theme,
    // color-scheme, locale, and layout watchers see stable persisted values
    // on their first run. The native window is still hidden until
    // MainLayout.onMounted explicitly shows it.
    app.mount('#app')
    // The add-task window saves directory history and remembered locations,
    // and the floating window's mode changes from its menu or the tray.
    void import('@tauri-apps/api/event').then(({ listen }) => {
      void listen('add-window:closed', () => preferenceStore.reloadPreferenceFromDisk())
      void listen('float-window:mode', () => preferenceStore.reloadPreferenceFromDisk())
    })
    // The add-task dialog lives in this window, so the forwarder is not
    // set here: the dialog installs it when it mounts.
    // The UI remains usable if database inspection fails.
    const database = useDatabaseStore()
    await database.init().catch(() => undefined)
    if (database.phase === 'resetting') return
    await engineStore.initialize()

    const config = preferenceStore.config

    // ── Phase 2: engine startup (non-blocking) ────────────────────────────
    // The backend owns the sidecar, so there is no port and no secret
    // here: the backend starts the engine and publishes its state.
    taskStore.setApi(aria2Api)

    // Engine initialization stays asynchronous while the supervisor publishes state.
    const enginePromise = initEngine()

    // ── Phase 2 completion: engine ready ───────────────────────────────────
    try {
      const ok = await enginePromise

      // Global option sync and speed scheduler are now handled by Rust:
      // - on_engine_ready() syncs system.json options to aria2 via changeGlobalOption
      // - spawn_speed_scheduler() runs a 60s timer in tokio (no WebView needed)
      if (ok) {
        await preferenceStore.reloadPreferenceFromDisk()
        logger.debug('Engine', 'runtime_services_ready')
      }
    } catch (e) {
      logger.error('Engine', `unexpected startup error: ${getErrorMessage(e)}`)
    }

    // Resume all paused/waiting tasks on launch if configured
    if (config.resumeAllWhenAppLaunched) {
      taskStore.resumeAllTask().catch((e) => logger.debug('main.resumeAll', e))
    }

    // ── Phase 4: deferred non-critical tasks ───────────────────────────────
    autoCheckForUpdate()
    syncNetworkSourcesIfDue(true)

    // Initialize download history database, then schedule lightweight cleanup.
    historyStore
      .init()
      .then(() => {
        const runCleanup = async () => {
          try {
            const { runHistoryMaintenance } = await import('./composables/useStaleCleanup')
            const { extractHistoryFilePaths } = await import('./composables/useTaskLifecycle')
            await runHistoryMaintenance({
              autoDeleteStaleRecords: !!preferenceStore.config?.autoDeleteStaleRecords,
              completedRecordRetentionDays: Number(preferenceStore.config?.completedRecordRetentionDays ?? 0),
              getRecords: historyStore.getRecords,
              removeStaleRecords: historyStore.removeStaleRecords,
              removeHistoryRecords: historyStore.removeStaleRecords,
              removeTaskRecord: aria2Api.removeTaskRecord,
              extractFilePaths: extractHistoryFilePaths,
            })
            await taskStore.fetchList()
          } catch (e) {
            logger.debug('HistoryMaintenance', e)
          }
        }
        // First scan 30s after startup — not urgent.
        setTimeout(runCleanup, 30_000)
        // Re-scan every 30 minutes for long-running sessions.
        setInterval(runCleanup, 1_800_000)
      })
      .catch((e) => logger.warn('HistoryDB', 'init failed: ' + e))

    setInterval(() => syncNetworkSourcesIfDue(false), 3_600_000)

    // Warm up Tauri FS plugin IPC channel to eliminate cold-start delay on first
    // file operation (e.g. task deletion).
    setTimeout(() => {
      import('@tauri-apps/plugin-fs').then(({ exists }) => exists('/')).catch(() => {})
    }, 3000)

    // ── Lightweight mode: destroy WebView after autostart init ─────────
    //
    // When autostart + autoHideWindow + lightweightMode are all enabled,
    // destroy the WebView to free ~300MB RAM.  This MUST run after all
    // critical invoke() calls complete (engine start, option sync,
    // resume-all, history init) because invoke() requires a live WebView.
    //
    // Delegates to handle_minimize_to_tray() in Rust which handles:
    //   - end_cold_start()  → prevents re-hide on window recreation
    //   - lightweightMode   → window.destroy() vs window.hide()
    //   - macOS Dock hiding → hideDockOnMinimize (cfg-gated in Rust)
    //
    // Cross-platform: macOS (WKWebView), Windows (WebView2), Linux (WebKitGTK)
    // all release the renderer process on destroy().  ExitRequested handler
    // in handle_run_event() calls prevent_exit() to keep the process alive.
    if (config.lightweightMode && config.autoHideWindow) {
      try {
        const { invoke } = await import('@tauri-apps/api/core')
        const isAutostart = await invoke<boolean>('is_autostart_launch')
        if (isAutostart) {
          logger.info('main', 'autostart + lightweight: destroying WebView via minimize_to_tray')
          await invoke('minimize_to_tray')
          // WebView destroyed — JS execution stops here.
          // All background services (stat, monitor, speed scheduler) continue in Rust.
          return
        }
      } catch (e) {
        // Non-fatal: WebView stays alive (standard autostart-hide behavior).
        // Graceful degradation — the user just doesn't get the RAM savings.
        logger.debug('main.lightweightAutostart', e)
      }
    }

    // The clipboard monitor offers a copied link to the user.
    //
    // The clipboard is read when the window gets the focus: that is the
    // moment the user has just copied something.
    getCurrentWindow().onFocusChanged(async ({ payload: focused }) => {
      if (!focused) return
      const clipboardConfig = preferenceStore.config.clipboard
      if (!clipboardConfig?.enable) return
      try {
        const { readText } = await import('@tauri-apps/plugin-clipboard-manager')
        const text = ((await readText()) || '').trim()
        if (!text) return
        const { detectResource, shouldIgnoreClipboardTextForAutoDetect } = await import('@shared/utils')
        if (shouldIgnoreClipboardTextForAutoDetect(text) || !detectResource(text, clipboardConfig)) return
        appStore.showAddTaskDialog()
      } catch (e) {
        logger.debug('Main.clipboardMonitor', e)
      }
    })
  }

  void bootstrapMainWindow().catch(async (e) => {
    logger.error('main.bootstrap', e)
    // A failed hydration must not leave a permanently hidden desktop process.
    const { message } = await import('@tauri-apps/plugin-dialog')
    await message(getErrorMessage(e), { title: 'BitMaher', kind: 'error' })
  })
} // end: main window initialization
