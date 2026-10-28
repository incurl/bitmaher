/**
 * The Vite configuration.
 *
 * The path aliases mirror the ones Rayburst uses, so that the ported
 * components resolve their imports unchanged.
 *
 * The `@tauri-apps/*` aliases point at the platform shim, which
 * implements the Tauri API on top of the Wails runtime. They are
 * listed one by one rather than as a prefix, because Vite matches an
 * alias exactly unless it ends with a slash.
 */
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import wails from '@wailsio/runtime/plugins/vite';
import { fileURLToPath, URL } from 'node:url';

const platform = (file: string) => fileURLToPath(new URL(`./src/platform/${file}`, import.meta.url));

export default defineConfig({
  plugins: [vue(), wails('./bindings')],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@shared': fileURLToPath(new URL('./src/shared', import.meta.url)),

      '@tauri-apps/api/core': platform('core.ts'),
      '@tauri-apps/api/event': platform('event.ts'),
      '@tauri-apps/api/window': platform('window.ts'),
      '@tauri-apps/api/webviewWindow': platform('window.ts'),
      '@tauri-apps/api/webview': platform('webview.ts'),
      '@tauri-apps/api/path': platform('path.ts'),
      '@tauri-apps/api/app': platform('app.ts'),
      '@tauri-apps/plugin-dialog': platform('dialog.ts'),
      '@tauri-apps/plugin-opener': platform('opener.ts'),
      '@tauri-apps/plugin-process': platform('process.ts'),
      '@tauri-apps/plugin-store': platform('store.ts'),
      '@tauri-apps/plugin-os': platform('os.ts'),
      '@tauri-apps/plugin-fs': platform('fs.ts'),
      '@tauri-apps/plugin-clipboard-manager': platform('clipboard.ts'),
      '@tauri-apps/plugin-log': platform('logger.ts'),
      'tauri-plugin-locale-api': platform('locale.ts'),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'naive-ui': ['naive-ui'],
          'vue-vendor': ['vue', 'vue-router', 'pinia', 'vue-i18n'],
        },
      },
    },
  },
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
});
