/**
 * The Tauri-compatible window API.
 *
 * BitMaher runs in a single window, so the parts of the API that belong
 * to the multi-window features of Rayburst — the float window, the add
 * window — answer with the neutral value.
 */
import { Events } from '@wailsio/runtime';
import { listen as listenEvent, type Event, type UnlistenFn } from './event';

/** A window handle. */
export interface WindowHandle {
  /** The label of the window. BitMaher has one, and it is the main one. */
  label: string;
  /** Closes the window. */
  destroy(): Promise<void>;
  /** Hides the window. */
  hide(): Promise<void>;
  /** Shows the window. */
  show(): Promise<void>;
  /** Minimises the window. */
  minimize(): Promise<void>;
  /** Maximises the window. */
  maximize(): Promise<void>;
  /** Unmaximises the window. */
  unmaximize(): Promise<void>;
  /** Toggles between the maximised and the normal state. */
  toggleMaximize(): Promise<void>;
  /** Closes the window. */
  close(): Promise<void>;
  /** Answers whether the window is visible. */
  isVisible(): Promise<boolean>;
  /** Answers whether the window is maximised. */
  isMaximized(): Promise<boolean>;
  /** Starts a drag of the window. */
  startDragging(): Promise<void>;
  /** Sets the decorations of the window. */
  setDecorations(visible: boolean): Promise<void>;
  /** The scale factor of the window. */
  scaleFactor(): Promise<number>;
  /** Listens for a change of the scale factor. */
  onScaleChanged(handler: (event: { payload: number }) => void): Promise<UnlistenFn>;
  /** Listens for a resize of the window. */
  onResized(handler: (event: { payload: { width: number; height: number } }) => void): Promise<UnlistenFn>;
  /** Listens for a request to close the window. */
  onCloseRequested(handler: (event: { preventDefault(): void }) => void): Promise<UnlistenFn>;
  /** Restores the window from the minimised state. */
  unminimize(): Promise<void>;
  /** Sets the focus of the window. */
  setFocus(): Promise<void>;
  /** The size of the window. */
  innerSize(): Promise<{ width: number; height: number }>;
  /** Listens for a window event. */
  onFocusChanged(handler: (event: { payload: boolean }) => void): Promise<UnlistenFn>;
  /** Listens for an arbitrary event. */
  listen<T>(eventName: string, handler: (event: Event<T>) => void): Promise<UnlistenFn>;
}

/**
 * The window BitMaher runs in.
 *
 * The single window of the application has the label `main`. The
 * secondary windows of Rayburst are not ported: their commands answer
 * with the neutral value.
 */
const mainWindow: WindowHandle = {
  label: 'main',

  async destroy() {
    await Events.Emit('window:destroy' as never);
  },

  async hide() {
    await Events.Emit('window:hide' as never);
  },

  async show() {
    await Events.Emit('window:show' as never);
  },

  async minimize() {
    await Events.Emit('window:minimize' as never);
  },

  async maximize() {
    await Events.Emit('window:maximize' as never);
  },

  async unmaximize() {
    await Events.Emit('window:unmaximize' as never);
  },

  async unminimize() {
    await Events.Emit('window:unminimize' as never);
  },

  async toggleMaximize() {
    await Events.Emit('window:toggle-maximize' as never);
  },

  async close() {
    await Events.Emit('window:close' as never);
  },

  async isVisible() {
    return true;
  },

  async isMaximized() {
    return false;
  },

  async startDragging() {
    await Events.Emit('window:start-dragging' as never);
  },

  async setDecorations(visible: boolean) {
    await Events.Emit('window:set-decorations' as never, visible as never);
  },

  async scaleFactor() {
    return window.devicePixelRatio;
  },

  async onScaleChanged(handler) {
    return listenEvent<number>('window:scale-changed', (event) => handler({ payload: event.payload }));
  },

  async onResized(handler) {
    return listenEvent<{ width: number; height: number }>('window:resized', (event) =>
      handler({ payload: event.payload }),
    );
  },

  async onCloseRequested(handler) {
    return listenEvent<undefined>('window:close-requested', () =>
      handler({ preventDefault: () => undefined }),
    );
  },

  async setFocus() {
    await Events.Emit('window:focus' as never);
  },

  async innerSize() {
    return { width: window.innerWidth, height: window.innerHeight };
  },

  async onFocusChanged(handler) {
    return listenEvent<boolean>('window:focus-changed', (event) => handler({ payload: event.payload }));
  },

  async listen<T>(eventName: string, handler: (event: Event<T>) => void) {
    return listenEvent<T>(eventName, handler);
  },
};

/** Returns the window the application runs in. */
export function getCurrentWindow(): WindowHandle {
  return mainWindow;
}

/** Returns the webview window of the application. */
export function getCurrentWebviewWindow(): WindowHandle {
  return mainWindow;
}

/** The events of a drag and drop. */
export type DragDropEvent = {
  /** The kind of the event. */
  type: 'enter' | 'over' | 'drop' | 'leave';
  /** The payload of the event: the files that were dragged, and where. */
  payload: {
    /** The kind of the event, as the Tauri plugin reports it. */
    type: 'enter' | 'over' | 'drop' | 'leave';
    /** The files that were dragged. */
    paths: string[];
    /** Where the pointer is. */
    position: { x: number; y: number };
  };
};
