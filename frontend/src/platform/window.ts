/**
 * The Tauri-compatible window API.
 *
 * The window of the application is the Wails runtime's own window: the
 * runtime addresses the window the webview runs in, and its methods
 * are the ones the operating system answers to.
 *
 * BitMaher runs in a single window, so the parts of the API that belong
 * to the multi-window features of Rayburst — the float window, the add
 * window — answer with the neutral value.
 */
import { Window } from '@wailsio/runtime';
import { listen as listenEvent, type Event, type UnlistenFn } from './event';
import { invoke } from './core';

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
    // The runtime has no destroy: closing the window is the end of it.
    await Window.Close();
  },

  async hide() {
    await Window.Hide();
  },

  async show() {
    await Window.Show();
  },

  async minimize() {
    await Window.Minimise();
  },

  async maximize() {
    await Window.Maximise();
  },

  async unmaximize() {
    await Window.UnMaximise();
  },

  async unminimize() {
    await Window.UnMinimise();
  },

  async toggleMaximize() {
    await Window.ToggleMaximise();
  },

  async close() {
    await Window.Close();
  },

  async isVisible() {
    // The runtime does not carry the question to the frontend, so the
    // backend answers it. The answer has to be the truth: the frontend
    // hides the window when it is asked to, and a window that was
    // already hidden does not need hiding again.
    return invoke<boolean>('window_is_visible');
  },

  async isMaximized() {
    return Window.IsMaximised();
  },

  async startDragging() {
    // The runtime has no drag of its own. The regions of the window
    // that drag are the ones the stylesheet marks, and the webview
    // drags them itself.
    return;
  },

  async setDecorations(visible: boolean) {
    // The frame is the backend's business: the window is created
    // there, and the backend is the one that turns the frame on and
    // off.
    await invoke('set_window_decorations', { visible });
  },

  async scaleFactor() {
    return window.devicePixelRatio;
  },

  async onScaleChanged(handler) {
    // The Wails event carries no payload: it says that the scale of the
    // screen changed, and not by how much. The factor is read here, so
    // that the caller gets the same shape as under Tauri.
    return listenEvent<number>('common:WindowDPIChanged', () =>
      handler({ payload: window.devicePixelRatio }),
    );
  },

  async onResized(handler) {
    // The Wails event carries no payload either: it says that the
    // window changed its size. The size is read here, so that the
    // caller gets the same shape as under Tauri.
    return listenEvent<{ width: number; height: number }>('common:WindowDidResize', () =>
      handler({ payload: { width: window.innerWidth, height: window.innerHeight } }),
    );
  },

  async onCloseRequested(handler) {
    return listenEvent<undefined>('common:WindowClosing', () =>
      handler({ preventDefault: () => undefined }),
    );
  },

  async setFocus() {
    await Window.Focus();
  },

  async innerSize() {
    return { width: window.innerWidth, height: window.innerHeight };
  },

  async onFocusChanged(handler) {
    return listenEvent<boolean>('common:WindowFocus', (event) => handler({ payload: event.payload }));
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
