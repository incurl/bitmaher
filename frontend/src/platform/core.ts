/**
 * The Tauri-compatible `invoke`.
 *
 * The ported components are written against the Tauri API. This module
 * keeps that shape and routes the calls to the Go backend through the
 * Wails runtime, so that the components need no changes.
 *
 * The command names are the ones Rayburst uses. They are mapped onto the
 * Wails services in `commands.ts`.
 */
import { Call } from '@wailsio/runtime';
import { resolveCommand } from './commands';

/**
 * Calls a backend command.
 *
 * The Tauri signature is preserved: the command name and an optional
 * payload. The Wails services take positional arguments, so the payload
 * is spread into them.
 */
export function invoke<T>(command: string, args?: Record<string, unknown> | null): Promise<T> {
  const mapped = resolveCommand(command, args ?? {});
  return Call.ByName(mapped.method, ...mapped.args) as unknown as Promise<T>;
}

/**
 * The Tauri `join` helper, which concatenates the parts of a path.
 *
 * The Wails runtime has no path helpers, so the separator is used
 * directly. The frontend only ever joins relative parts.
 */
export function join(...parts: string[]): string {
  return parts
    .filter((part) => part.length > 0)
    .join('/')
    .replace(/\/{2,}/g, '/');
}
