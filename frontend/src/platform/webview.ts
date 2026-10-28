/**
 * The Tauri-compatible webview API.
 *
 * The webview is the window the application runs in. It is separate
 * from the window module, because the two are mocked apart in the
 * tests.
 */
import { Window } from '@wailsio/runtime';
import { listen as listenEvent, type UnlistenFn } from './event';
import type { DragDropEvent } from './window';

/** The webview the application runs in. */
export function getCurrentWebview() {
  return {
    /** The drag and drop of the operating system. */
    onDragDropEvent(handler: (event: DragDropEvent) => void): Promise<UnlistenFn> {
      return listenEvent<DragDropEvent>('webview:drag-drop', (event) => handler(event.payload));
    },
    /**
     * Sets the zoom of the webview.
     *
     * The zoom is the magnification of the page: the interface is laid
     * out in `rem`, so scaling the root font size would do the same.
     * The webview's own zoom is used instead, because it scales the
     * text with it, and it is what the platform expects.
     */
    async setZoom(factor: number): Promise<void> {
      await Window.SetZoom(factor);
    },
    /** The zoom of the webview. */
    async zoomFactor(): Promise<number> {
      return Window.GetZoom();
    },
  };
}
