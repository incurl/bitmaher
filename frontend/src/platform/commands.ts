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

    // --- the preferences the backend owns -----------------------------------
    case 'refresh_runtime_config':
      // The backend applies the stored preferences to the window.
      return { method: APP_SERVICE + '.RefreshRuntimeConfig', args: [] };
    case 'replace_system_config':
      return { method: APP_SERVICE + '.UnsupportedCommand', args: [command] };
    case 'get_engine_conf_path':
      return { method: APP_SERVICE + '.PlatformPaths', args: [] };

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
