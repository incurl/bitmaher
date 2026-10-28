/**
 * The interface size.
 *
 * The setting is a factor, and it is applied as the zoom of the
 * webview: the whole interface follows it, text included.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

const { setZoomMock, getZoomMock } = vi.hoisted(() => ({
  setZoomMock: vi.fn().mockResolvedValue(undefined),
  getZoomMock: vi.fn().mockResolvedValue(1),
}));

vi.mock('@tauri-apps/api/webview', () => ({
  getCurrentWebview: () => ({
    setZoom: setZoomMock,
    zoomFactor: getZoomMock,
  }),
}));

import { usePreferenceStore } from '@/stores/preference';
import { UI_SCALE_STEPS, shortcutScale, stepUiScale } from '../useInterfaceScale';

describe('the interface size', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('offers the steps the settings declare', () => {
    expect(UI_SCALE_STEPS).toContain(1);
    expect(UI_SCALE_STEPS.length).toBeGreaterThan(2);
  });

  it('steps up and down through the steps', () => {
    const index = UI_SCALE_STEPS.indexOf(1);
    expect(stepUiScale(1, 1)).toBe(UI_SCALE_STEPS[index + 1]);
    expect(stepUiScale(1, -1)).toBe(UI_SCALE_STEPS[index - 1]);
  });

  it('stays at the ends', () => {
    expect(stepUiScale(UI_SCALE_STEPS[0], -1)).toBe(UI_SCALE_STEPS[0]);
    expect(stepUiScale(UI_SCALE_STEPS[UI_SCALE_STEPS.length - 1], 1)).toBe(
      UI_SCALE_STEPS[UI_SCALE_STEPS.length - 1],
    );
  });

  it('maps the shortcuts onto the steps', () => {
    const plus = new KeyboardEvent('keydown', { key: '+', ctrlKey: true });
    const minus = new KeyboardEvent('keydown', { key: '-', ctrlKey: true });
    const reset = new KeyboardEvent('keydown', { key: '0', ctrlKey: true });
    const other = new KeyboardEvent('keydown', { key: 'a', ctrlKey: true });

    expect(shortcutScale(plus, 1)).not.toBeNull();
    expect(shortcutScale(minus, 1)).not.toBeNull();
    expect(shortcutScale(reset, 1)).toBe(1);
    expect(shortcutScale(other, 1)).toBeNull();
  });

  it('ignores a shortcut without the modifier', () => {
    const plain = new KeyboardEvent('keydown', { key: '+' });
    expect(shortcutScale(plain, 1)).toBeNull();
  });

  it('keeps the setting and the zoom in step', async () => {
    const store = usePreferenceStore();
    store.config.uiScale = 1.25;

    // The composable watches the setting and applies it.
    const { useInterfaceScale } = await import('../useInterfaceScale');
    useInterfaceScale();

    // The watcher runs on the next tick.
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(setZoomMock).toHaveBeenCalledWith(1.25);
  });
});
