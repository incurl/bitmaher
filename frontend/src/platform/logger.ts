/**
 * The Tauri-compatible log API.
 *
 * The frontend writes to the console of the backend, which keeps the
 * entries in the debug console of the application.
 */
import { emit } from './event';

/** The levels of the log. */
export type LogLevel = 'trace' | 'debug' | 'info' | 'warn' | 'error';

/** The extra fields of an entry. */
export interface LogOptions {
  /** The fields of the entry, as the Tauri log plugin takes them. */
  keyValues?: Record<string, string | number | boolean | undefined>;
}

/**
 * Writes an entry to the log.
 *
 * The entry goes to the backend as an event, so that it lands in the
 * debug console of the application next to the entries of the backend
 * itself. The fields are written with the message.
 */
export async function log(level: LogLevel, message: string, options: LogOptions = {}): Promise<void> {
  const fields = options.keyValues ?? {};
  const rendered = Object.entries(fields)
    .filter(([, value]) => value !== undefined)
    .map(([key, value]) => `${key}=${String(value)}`)
    .join(' ');

  await emit('frontend:log', {
    id: 0,
    time: new Date().toISOString(),
    level,
    source: 'frontend',
    message: rendered ? `${message} ${rendered}` : message,
    detail: '',
  });
}

/** Writes a trace entry. */
export async function trace(message: string, options?: LogOptions): Promise<void> {
  await log('trace', message, options);
}

/** Writes a debug entry. */
export async function debug(message: string, options?: LogOptions): Promise<void> {
  await log('debug', message, options);
}

/** Writes an info entry. */
export async function info(message: string, options?: LogOptions): Promise<void> {
  await log('info', message, options);
}

/** Writes a warning. */
export async function warn(message: string, options?: LogOptions): Promise<void> {
  await log('warn', message, options);
}

/** Writes an error. */
export async function error(message: string, options?: LogOptions): Promise<void> {
  await log('error', message, options);
}
