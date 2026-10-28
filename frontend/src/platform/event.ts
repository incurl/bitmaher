/**
 * The Tauri-compatible event API.
 *
 * Rayburst listens for `tasks:snapshot` and a few window events. The
 * Wails runtime emits and receives events with the same shape, so the
 * listeners are forwarded unchanged.
 */
import { Events } from '@wailsio/runtime';

/** The function that cancels a listener. */
export type UnlistenFn = () => void;

/** The callback of a listener. */
export type EventCallback<T> = (event: Event<T>) => void;

/** The payload of an event. */
export interface Event<T> {
  event: string;
  id: number;
  payload: T;
}

/**
 * Listens for an event.
 *
 * The Wails listener receives the payload directly, while the Tauri one
 * receives an envelope. The envelope is rebuilt here, so that the
 * callers do not have to change.
 */
export function listen<T>(
  eventName: string,
  handler: (event: Event<T>) => void,
): Promise<UnlistenFn> {
  return Promise.resolve(
    Events.On(eventName as never, (event: { data: T }) => {
      handler({ event: eventName, id: 0, payload: event.data });
    }),
  );
}

/** Listens for an event once. */
export function once<T>(eventName: string, handler: (event: Event<T>) => void): Promise<UnlistenFn> {
  return Promise.resolve(
    Events.Once(eventName as never, (event: { data: T }) => {
      handler({ event: eventName, id: 0, payload: event.data });
    }),
  );
}

/** Stops every listener of an event. */
export function unlisten(eventName: string): void {
  Events.Off(eventName);
}

/** Emits an event to the backend. */
export function emit(eventName: string, payload?: unknown): Promise<boolean> {
  return Events.Emit(eventName as never, payload as never);
}
