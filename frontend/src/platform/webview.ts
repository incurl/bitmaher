/**
 * The Tauri-compatible webview API.
 *
 * The webview is the window the application runs in. It is separate
 * from the window module, because the two are mocked apart in the
 * tests.
 */
import { Events } from '@wailsio/runtime';
import { listen as listenEvent, type UnlistenFn } from './event';
import type { DragDropEvent } from './window';

/** The webview the application runs in. */
export function getCurrentWebview() {
  return {
    /** The drag and drop of the operating system. */
    onDragDropEvent(handler: (event: DragDropEvent) => void): Promise<UnlistenFn> {
      return listenEvent<DragDropEvent>('webview:drag-drop', (event) => handler(event.payload));
    },
    /** Sets the zoom of the webview. */
    async setZoom(factor: number): Promise<void> {
      await Events.Emit('webview:set-zoom' as never, factor as never);
    },
    /** The zoom of the webview. */
    async zoomFactor(): Promise<number> {
      return window.devicePixelRatio;
    },
  };
}
