/**
 * The history of the downloads that have come to rest.
 *
 * The history outlives the download records: a download that is removed
 * from the list stays in the history, and the history is what the
 * interface shows in its own panel.
 *
 * The backend keeps it in PocketBase, so the commands below are the
 * ones of the backend, reached through the command map.
 */
import { defineStore } from 'pinia'
import { invoke } from '@tauri-apps/api/core'
import { useDatabaseStore } from '@/stores/database'
import type { HistoryRecord } from '@shared/types'

export type HistoryRecordSortOrder = 'ascend' | 'descend' | false

export interface HistoryRecordsPageInput {
  status?: string
  page: number
  pageSize: number
  sortField?: string
  sortOrder?: HistoryRecordSortOrder
}

export interface HistoryRecordsPage {
  records: HistoryRecord[]
  total: number
}

export const useHistoryStore = defineStore('history', () => {
  const database = useDatabaseStore()
  async function call<T>(command: string, args?: Record<string, unknown>): Promise<T> {
    await database.init()
    return invoke<T>(command, args)
  }
  return {
    init: database.init,
    addRecord: (record: HistoryRecord) => call<HistoryRecord>('history_add_record', { record }),
    getRecords: (status?: string) => call<HistoryRecord[]>('history_get_records', { status }),
    getRecordsPage: (input: HistoryRecordsPageInput) =>
      call<HistoryRecordsPage>('history_get_page', {
        input: { ...input, sortOrder: input.sortOrder || null },
      }),
    getRecordByGid: (gid: string) => call<HistoryRecord | null>('history_get_record', { gid }),
    removeRecord: (gid: string) => call<void>('history_remove_record', { gid }),
    removeBirthRecords: (gids: string[]) => call<void>('history_remove_births', { gids }),
    clearRecords: (status?: string) => call<void>('history_clear_records', { status }),
    removeStaleRecords: (gids: string[]) => call<void>('history_remove_stale', { gids }),
    checkIntegrity: () => call<string>('history_check_integrity'),
    recordTaskBirth: (gid: string, addedAt = new Date().toISOString()) =>
      call<void>('history_record_birth', { gid, addedAt }),
    async loadBirthRecords(): Promise<Array<{ gid: string; added_at: string }>> {
      const records = await call<Array<{ gid: string; added_at: string }>>('history_load_births')
      return records.map((record) => ({ gid: record.gid, added_at: record.added_at }))
    },
    getSchemaVersion: () => call<number>('database_schema_version'),
  }
})
