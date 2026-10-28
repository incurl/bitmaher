/**
 * The Vitest configuration.
 *
 * It reuses the Vite aliases, so that the ported components resolve
 * their imports in the tests as well.
 */
import { defineConfig, mergeConfig } from 'vitest/config';
import viteConfig from './vite.config';

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      server: {
        deps: {
          // The material colour utilities ship without an extension on
          // their imports, so the bundler has to resolve them.
          inline: ['@material/material-color-utilities'],
        },
      },
      environment: 'happy-dom',
      testTimeout: 10000,
      include: ['src/**/*.{test,spec}.ts'],
      coverage: {
        provider: 'v8',
        reporter: ['text', 'text-summary', 'lcov'],
        include: ['src/shared/**/*.ts', 'src/stores/**/*.ts', 'src/api/**/*.ts'],
        exclude: ['src/**/*.{test,spec}.ts', 'src/shared/locales/**'],
      },
    },
  }),
);
