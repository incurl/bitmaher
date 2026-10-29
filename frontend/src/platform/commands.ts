/**
 * The command map.
 *
 * Rayburst's components call the Tauri commands by name. This module
 * maps each of them onto the Wails service method that does the same
 * thing, and shapes the arguments into the positional form the Wails
 * bindings take.
 *
 * A command that has no counterpart in BitMaher is mapped onto a method
 * that returns the neutral value. The components that use it are the
 * ones that belong to a feature BitMaher does not have — the tray, the
 * float window, the updater — and the neutral value keeps them inert.
 */

/** A resolved call: the Wails method and its positional arguments. */
export interface ResolvedCommand {
  method: string;
  args: unknown[];
}

/**
 * The prefix of the bound methods.
 *
 * The Wails runtime addresses a bound method by the full name of the
 * service: the import path of the package, a dot, the name of the
 * struct and the name of the method.
 */
const APP_SERVICE = 'github.com/incurl/bitmaher/internal/bindings.AppService';

/**
 * Maps a Tauri command onto a Wails method.
 *
 * The lookup is a plain switch, because the set of commands is fixed
 * and a table would hide the mapping from the reader.
 */
export function resolveCommand(
  command: string,
  args: Record<string, unknown>,
): ResolvedCommand {
  switch (command) {
    // --- the task list -------------------------------------------------
    case 'tasks_refresh':
      return { method: APP_SERVICE + '.SyncDownloads', args: [] };
    case 'aria2_fetch_task_list':
    case 'aria2_fetch_active_task_list':
      return { method: APP_SERVICE + '.ListDownloads', args: [numberOf(args)] };

    // --- one task ------------------------------------------------------
    case 'aria2_fetch_task_item':
    case 'aria2_fetch_task_item_with_peers':
      return { method: APP_SERVICE + '.ListDownloads', args: [numberOf(args)] };

    // --- the engine ----------------------------------------------------
    case 'aria2_get_global_stat':
      return { method: APP_SERVICE + '.DownloadStat', args: [] };

    // --- the actions ----------------------------------------------------
    case 'aria2_pause_task':
      return { method: APP_SERVICE + '.PauseDownload', args: [stringOf(args, 'gid')] };
    case 'aria2_resume_task':
      return { method: APP_SERVICE + '.ResumeDownload', args: [stringOf(args, 'gid')] };
    case 'aria2_force_pause_task':
      return { method: APP_SERVICE + '.PauseDownload', args: [stringOf(args, 'gid')] };
    case 'aria2_force_pause_all':
      return { method: APP_SERVICE + '.PauseDownloadAll', args: [] };
    case 'aria2_remove_task':
      return { method: APP_SERVICE + '.RemoveDownload', args: [stringOf(args, 'gid')] };
    case 'aria2_delete_task_record':
      return { method: APP_SERVICE + '.RemoveDownload', args: [stringOf(args, 'gid')] };
    case 'aria2_purge_task_records':
      return { method: APP_SERVICE + '.PurgeDownloads', args: [] };
    case 'aria2_get_files':
      return { method: APP_SERVICE + '.DownloadFiles', args: [stringOf(args, 'gid')] };
    case 'aria2_select_file':
      return {
        method: APP_SERVICE + '.SelectDownloadFiles',
        args: [stringOf(args, 'gid'), numbersOf(args, 'indexes')],
      };

    // --- adding a task --------------------------------------------------
    case 'aria2_add_uri':
    case 'aria2_add_torrent':
      return { method: APP_SERVICE + '.AddDownload', args: [addRequest(args)] };

    // --- the settings ----------------------------------------------------
    case 'database_initialize':
      return { method: APP_SERVICE + '.GetSettings', args: [] };
    case 'read_settings_backup_file':
    case 'write_settings_backup_file':
      return { method: APP_SERVICE + '.GetSettings', args: [] };

    // --- the lifecycle ----------------------------------------------------
    // The end of the application is the backend's business: it owns the
    // sidecar, and the sidecar must not outlive it.
    case 'app_exit':
      return { method: APP_SERVICE + '.AppExit', args: [exitCodeOf(args)] };
    case 'app_relaunch':
      return { method: APP_SERVICE + '.AppRelaunch', args: [] };
    case 'release_window_close':
      return { method: APP_SERVICE + '.ReleaseWindowClose', args: [] };
    case 'minimize_window_to_tray':
      return { method: APP_SERVICE + '.MinimiseWindowToTray', args: [] };

    // --- the console ------------------------------------------------------
    case 'clear_log_file':
      return { method: APP_SERVICE + '.ClearConsoleEntries', args: [] };

    // --- the preferences --------------------------------------------------
    // The ported frontend keeps its configuration in a key-value store.
    // The backend owns the interface preferences, so the store is a view
    // onto the settings record.
    case 'store_read':
      return { method: APP_SERVICE + '.StoreRead', args: [stringOf(args, 'name')] };
    case 'store_read_value':
      return {
        method: APP_SERVICE + '.StoreReadValue',
        args: [stringOf(args, 'name'), stringOf(args, 'key')],
      };
    case 'store_write':
      return {
        method: APP_SERVICE + '.StoreWrite',
        args: [stringOf(args, 'name'), stringOf(args, 'key'), args.value],
      };
    case 'store_delete':
      return {
        method: APP_SERVICE + '.StoreDelete',
        args: [stringOf(args, 'name'), stringOf(args, 'key')],
      };
    case 'store_clear':
      return { method: APP_SERVICE + '.StoreClear', args: [stringOf(args, 'name')] };
    case 'store_keys':
      return { method: APP_SERVICE + '.StoreKeys', args: [stringOf(args, 'name')] };

    // --- the history ---------------------------------------------------------
    // The downloads that have come to rest. The history outlives the
    // download records, so it is what the interface shows in its own
    // panel.
    case 'history_add_record':
      return {
        method: APP_SERVICE + '.HistoryAddRecord',
        args: [args.record ?? {}],
      };
    case 'history_get_records':
      return {
        method: APP_SERVICE + '.HistoryGetRecords',
        args: [stringOf(args, 'status')],
      };
    case 'history_get_page':
      return {
        method: APP_SERVICE + '.HistoryGetPage',
        args: [args.input ?? {}],
      };
    case 'history_get_record':
      return {
        method: APP_SERVICE + '.HistoryGetRecord',
        args: [stringOf(args, 'gid')],
      };
    case 'history_remove_record':
      return {
        method: APP_SERVICE + '.HistoryRemoveRecord',
        args: [stringOf(args, 'gid')],
      };
    case 'history_clear_records':
      return {
        method: APP_SERVICE + '.HistoryClearRecords',
        args: [stringOf(args, 'status')],
      };
    case 'history_remove_stale':
      return {
        method: APP_SERVICE + '.HistoryRemoveStale',
        args: [stringsOf(args, 'gids')],
      };
    case 'history_record_birth':
      return {
        method: APP_SERVICE + '.HistoryRecordBirth',
        args: [stringOf(args, 'gid'), stringOf(args, 'addedAt')],
      };
    case 'history_load_births':
      return { method: APP_SERVICE + '.HistoryLoadBirths', args: [] };
    case 'history_remove_births':
      return {
        method: APP_SERVICE + '.HistoryRemoveBirths',
        args: [stringsOf(args, 'gids')],
      };

    // --- the database ----------------------------------------------------------
    case 'database_schema_version':
      return { method: APP_SERVICE + '.DatabaseSchemaVersion', args: [] };
    case 'history_check_integrity':
      return { method: APP_SERVICE + '.DatabaseIntegrityCheck', args: [] };

    // --- the window ----------------------------------------------------------
    case 'set_window_decorations':
      return {
        method: APP_SERVICE + '.SetWindowDecorations',
        args: [args.visible === true],
      };

    // --- the preferences the backend owns -----------------------------------
    case 'refresh_runtime_config':
      // The backend applies the stored preferences to the window.
      return { method: APP_SERVICE + '.RefreshRuntimeConfig', args: [] };
    case 'replace_system_config':
      return { method: APP_SERVICE + '.UnsupportedCommand', args: [command] };
    case 'get_engine_conf_path':
      return { method: APP_SERVICE + '.PlatformPaths', args: [] };

    // --- the file system -------------------------------------------------
    case 'check_path_is_dir':
      return { method: APP_SERVICE + '.PathIsDirectory', args: [stringOf(args, 'path')] };
    case 'delete_path':
      return {
        method: APP_SERVICE + '.DeletePath',
        args: [stringOf(args, 'path'), stringOf(args, 'mode')],
      };
    case 'list_dir_files':
      return { method: APP_SERVICE + '.ListDirectoryFiles', args: [stringOf(args, 'path')] };
    case 'rename_completed_file':
      return {
        method: APP_SERVICE + '.RenameCompletedFile',
        args: [stringOf(args, 'gid'), stringOf(args, 'name')],
      };

    // --- the categories ---------------------------------------------------
    case 'resolve_file_category':
      return {
        method: APP_SERVICE + '.ResolveFileCategory',
        args: [args.candidates ?? [], args.categories ?? [], stringOf(args, 'baseDir')],
      };
    case 'validate_file_categories':
      return {
        method: APP_SERVICE + '.ValidateFileCategories',
        args: [args.categories ?? [], stringOf(args, 'baseDir')],
      };

    // --- the preview ------------------------------------------------------
    case 'preview_download':
      return { method: APP_SERVICE + '.PreviewDownload', args: [args.request ?? {}] };
    case 'cancel_download_preview':
      return { method: APP_SERVICE + '.CancelDownloadPreview', args: [stringOf(args, 'id')] };

    // --- the state of the files -------------------------------------------
    case 'task_file_states':
      return { method: APP_SERVICE + '.TaskFileStates', args: [stringsOf(args, 'gids')] };

    // --- the engine --------------------------------------------------------
    case 'engine_recover_runtime_state':
      return { method: APP_SERVICE + '.EngineRecoverRuntimeState', args: [] };
    case 'engine_cancel':
      return { method: APP_SERVICE + '.EngineCancel', args: [] };

    // --- the notifications --------------------------------------------------
    case 'send_app_system_notification':
      return {
        method: APP_SERVICE + '.SendAppSystemNotification',
        args: [stringOf(args, 'title'), stringOf(args, 'body')],
      };
    case 'notification_access_state':
      return { method: APP_SERVICE + '.NotificationAccessState', args: [] };
    case 'open_notification_settings':
      return { method: APP_SERVICE + '.OpenNotificationSettings', args: [] };
    case 'open_default_apps_settings':
      return { method: APP_SERVICE + '.OpenDefaultAppsSettings', args: [] };

    // --- the application ----------------------------------------------------
    case 'set_app_log_level':
      return { method: APP_SERVICE + '.SetAppLogLevel', args: [stringOf(args, 'level')] };
    case 'export_diagnostic_logs':
      return { method: APP_SERVICE + '.ExportDiagnosticLogs', args: [stringOf(args, 'target')] };
    case 'factory_reset':
      return { method: APP_SERVICE + '.FactoryReset', args: [] };
    case 'database_reset':
      return { method: APP_SERVICE + '.DatabaseReset', args: [] };

    // --- the paths and the files ---------------------------------------------
    case 'platform_paths':
      return { method: APP_SERVICE + '.PlatformPaths', args: [] };
    case 'read_local_file':
      return { method: APP_SERVICE + '.ReadLocalFile', args: [stringOf(args, 'path')] };
    case 'write_local_file':
      return {
        method: APP_SERVICE + '.WriteLocalFile',
        args: [stringOf(args, 'path'), stringOf(args, 'contents')],
      };
    case 'open_external_url':
      return { method: APP_SERVICE + '.OpenExternalURL', args: [stringOf(args, 'url')] };

    // --- the identity of the application ---------------------------------------
    case 'app_name':
      return { method: APP_SERVICE + '.AppName', args: [] };
    case 'app_version':
      return { method: APP_SERVICE + '.AppVersion', args: [] };

    // --- the peer blocklist --------------------------------------------------
    case 'get_bt_peer_blocklist_status':
      return { method: APP_SERVICE + '.GetBtPeerBlocklistStatus', args: [] };
    case 'sync_bt_peer_blocklist':
      return { method: APP_SERVICE + '.SyncBtPeerBlocklist', args: [] };
    case 'reconcile_bt_peer_blocklist':
      return { method: APP_SERVICE + '.ReconcileBtPeerBlocklist', args: [] };

    // --- the remote files ------------------------------------------------------
    case 'fetch_remote_bytes':
      return {
        method: APP_SERVICE + '.FetchRemoteBytes',
        args: [
          stringOf(args, 'url'),
          nullableString(args, 'proxy'),
          nullableString(args, 'referer'),
          nullableString(args, 'cookie'),
          nullableString(args, 'userAgent'),
          args.requestHeaders ?? [],
        ],
      };
    case 'preview_site_rules':
      return { method: APP_SERVICE + '.PreviewSiteRules', args: [stringsOf(args, 'uris')] };

    // --- the features BitMaher does not have --------------------------------
    // These belong to the tray, the float window, the updater, the
    // protocol handler and the browser extension. They answer with a
    // neutral value, so the components that call them stay inert.
    case 'tasks_set_detailed':
      return { method: APP_SERVICE + '.SetDetailedTask', args: [nullableString(args, 'gid')] };
    case 'engine_supervisor_state':
      return { method: APP_SERVICE + '.EngineState', args: [] };
    case 'show_item_in_dir':
    case 'open_path_normalized':
      return { method: APP_SERVICE + '.ShowInFolder', args: [stringOf(args, 'path')] };
    case 'check_path_exists':
      return { method: APP_SERVICE + '.PathExists', args: [stringOf(args, 'path')] };
    case 'disk_free_space':
      return { method: APP_SERVICE + '.DiskFreeSpace', args: [stringOf(args, 'path')] };
    case 'system_shutdown':
      return { method: APP_SERVICE + '.Shutdown', args: [] };

    default:
      // An unknown command is a sign that a feature was ported without
      // its backend. It is reported rather than silently ignored, so
      // that the gap is visible in the console.
      return { method: APP_SERVICE + '.UnsupportedCommand', args: [command] };
  }
}

/** Reads the exit code the application ends with. */
function exitCodeOf(args: Record<string, unknown>): number {
  const value = args.code;
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

/** Reads the number of tasks to list. */
function numberOf(args: Record<string, unknown>): number {
  const value = args.limit;
  return typeof value === 'number' && value > 0 ? value : 500;
}

/** Reads a string argument. */
function stringOf(args: Record<string, unknown>, key: string): string {
  const value = args[key];
  return typeof value === 'string' ? value : '';
}

/** Reads a nullable string argument. */
function nullableString(args: Record<string, unknown>, key: string): string | null {
  const value = args[key];
  return typeof value === 'string' ? value : null;
}

/** Reads an array of strings. */
function stringsOf(args: Record<string, unknown>, key: string): string[] {
  const value = args[key];
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is string => typeof item === 'string');
}

/** Reads an array of numbers. */
function numbersOf(args: Record<string, unknown>, key: string): number[] {
  const value = args[key];
  return Array.isArray(value) ? value.filter((item): item is number => typeof item === 'number') : [];
}

/**
 * Builds the request of a new download.
 *
 * Rayburst sends the uris and the options separately. BitMaher takes
 * them together, in the shape the backend declares.
 */
function addRequest(args: Record<string, unknown>): Record<string, unknown> {
  const uris = Array.isArray(args.uris) ? (args.uris as string[]) : [];
  const options = (args.options ?? {}) as Record<string, string>;
  const torrent = args.torrent;

  return {
    uris,
    torrent: typeof torrent === 'string' ? torrent : '',
    options,
    selected_files: [],
  };
}
