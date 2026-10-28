/**
 * The Tauri-compatible store API.
 *
 * Rayburst keeps the preferences in a JSON file through the Tauri
 * store plugin. BitMaher keeps them in PocketBase, so the store is a
 * thin wrapper over the settings of the backend. The API is the one
 * the plugin offers, so that the ported code needs no change.
 */
import { invoke } from './core';

/** A key and value store. */
export interface Store {
  /** Reads a value. */
  get<T>(key: string): Promise<T | null>;
  /** Writes a value. */
  set(key: string, value: unknown): Promise<void>;
  /** Removes a value. */
  delete(key: string): Promise<boolean>;
  /** Removes every value. */
  clear(): Promise<void>;
  /** The keys that are in the store. */
  keys(): Promise<string[]>;
  /** Saves the store. The backend saves as it goes, so this does nothing. */
  save(): Promise<void>;
}

/**
 * Opens a store.
 *
 * The name is kept for the caller's sake: every store of Rayburst maps
 * onto the same settings record of BitMaher, and the values are keyed
 * by their name.
 */
export async function load(name = 'settings'): Promise<Store> {
  const values = new Map<string, unknown>();

  try {
    const stored = await invoke<Record<string, unknown>>('store_read', { name });
    for (const [key, value] of Object.entries(stored ?? {})) {
      values.set(key, value);
    }
  } catch {
    // A store that cannot be read is an empty one. The settings the
    // backend holds are the fallback.
  }

  return {
    async get<T>(key: string): Promise<T | null> {
      if (values.has(key)) return values.get(key) as T;
      return (await invoke<T | null>('store_read_value', { name, key })) ?? null;
    },

    async set(key: string, value: unknown): Promise<void> {
      values.set(key, value);
      await invoke('store_write', { name, key, value });
    },

    async delete(key: string): Promise<boolean> {
      const had = values.delete(key);
      await invoke('store_delete', { name, key });
      return had;
    },

    async clear(): Promise<void> {
      values.clear();
      await invoke('store_clear', { name });
    },

    async keys(): Promise<string[]> {
      const keys = await invoke<string[]>('store_keys', { name });
      return keys.length > 0 ? keys : [...values.keys()];
    },

    async save(): Promise<void> {
      // The backend writes every value as it is set.
    },
  };
}
