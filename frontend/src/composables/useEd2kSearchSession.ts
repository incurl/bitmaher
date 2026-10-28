/** @fileoverview Session-scoped ED2K search state. */
import { computed, ref } from 'vue'
import { cleanupEd2kSearch, ed2kSearch, getEd2kSearchResults } from '@/api/aria2'
import {
  ED2K_SEARCH_POLL_INTERVAL_MS,
  getEd2kSearchToastKey,
  shouldFinishEd2kSearchPoll,
  type Ed2kSearchOutcome,
} from '@/composables/useEd2kPreference'
import { logger } from '@shared/logger'
import type { Ed2kSearchResult } from '@shared/types'
import type { MessageContent } from '@/composables/useAppMessage'

type SearchState = 'idle' | 'searching' | 'cancelling'

interface AppMessage {
  success: (content: MessageContent) => unknown
  error: (content: MessageContent) => unknown
  warning: (content: MessageContent) => unknown
  info: (content: MessageContent) => unknown
}

interface UseEd2kSearchSessionOptions {
  t: (key: string, params?: Record<string, number>) => string
  message: AppMessage
}

const searchKeyword = ref('')
const searchFileType = ref('')
const searchMinSources = ref<number | null>(null)
const searchState = ref<SearchState>('idle')
const currentSearchGid = ref('')
const searchCancelled = ref(false)
const searchCleanupDone = ref(false)
const searchResults = ref<Ed2kSearchResult[]>([])
const searchElapsedMs = ref(0)

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function pollSearchResults(gid: string, maxDurationMs: number): Promise<Ed2kSearchResult[]> {
  let elapsedMs = 0
  let previousResultCount = -1
  let stablePolls = 0
  let latestResults: Ed2kSearchResult[] = []

  while (searchState.value === 'searching') {
    await wait(ED2K_SEARCH_POLL_INTERVAL_MS)
    if (searchState.value !== 'searching') break
    elapsedMs += ED2K_SEARCH_POLL_INTERVAL_MS
    searchElapsedMs.value = Math.min(elapsedMs, maxDurationMs)

    const payload = await getEd2kSearchResults({ gid })
    latestResults = payload.results ?? []
    searchResults.value = latestResults

    const resultCount = latestResults.length
    stablePolls = resultCount === previousResultCount ? stablePolls + 1 : 0

    if (
      shouldFinishEd2kSearchPoll({
        elapsedMs,
        resultCount,
        previousResultCount,
        stablePolls,
        moreResults: typeof payload.moreResults === 'boolean' ? payload.moreResults : undefined,
        maxDurationMs,
      })
    ) {
      break
    }

    previousResultCount = resultCount
  }

  return latestResults
}

async function cancelSearch() {
  const gid = currentSearchGid.value
  if (searchState.value === 'cancelling') return
  searchState.value = 'cancelling'
  searchCancelled.value = true
  if (!gid) return
  try {
    await cleanupEd2kSearch({ gid })
    searchCleanupDone.value = true
  } catch (e) {
    logger.debug('ED2K.searchCancel', e)
  }
}

export function useEd2kSearchSession(options: UseEd2kSearchSessionOptions) {
  const searchActive = computed(() => searchState.value !== 'idle')

  async function runSearch(maxDurationMs: number) {
    const keyword = searchKeyword.value.trim()
    if (searchState.value === 'searching') {
      await cancelSearch()
      return
    }
    if (!keyword) {
      options.message.warning(options.t('preferences.ed2k-search-keyword-required'))
      return
    }
    if (searchState.value !== 'idle') return
    searchState.value = 'searching'
    searchCancelled.value = false
    searchCleanupDone.value = false
    searchResults.value = []
    searchElapsedMs.value = 0
    options.message.info(options.t('preferences.ed2k-search-started'))

    let gid = ''
    let outcome: Ed2kSearchOutcome = 'completed'
    let resultCount = 0
    try {
      gid = await ed2kSearch({
        keyword,
        options: {
          ...(searchFileType.value ? { fileType: searchFileType.value } : {}),
          ...(searchMinSources.value ? { minSourceCount: String(searchMinSources.value) } : {}),
        },
      })
      currentSearchGid.value = gid
      searchResults.value = await pollSearchResults(gid, maxDurationMs)
      resultCount = searchResults.value.length
      if (searchCancelled.value) outcome = 'cancelled'
    } catch (e) {
      logger.debug('ED2K.search', e)
      outcome = searchCancelled.value ? 'cancelled' : 'failed'
    } finally {
      if (gid && currentSearchGid.value === gid && !searchCleanupDone.value) {
        try {
          await cleanupEd2kSearch({ gid })
          searchCleanupDone.value = true
        } catch (e) {
          logger.debug('ED2K.searchCleanup', e)
          options.message.warning(options.t('preferences.ed2k-search-cleanup-failed'))
        }
      }

      const toastKey = getEd2kSearchToastKey(outcome, resultCount)
      if (outcome === 'failed') options.message.error(options.t(toastKey))
      else if (outcome === 'cancelled' && resultCount > 0) {
        options.message.success(options.t(toastKey, { count: resultCount }))
      } else if (outcome === 'cancelled' || resultCount === 0) {
        options.message.warning(options.t(toastKey, { count: resultCount }))
      } else {
        options.message.success(options.t(toastKey, { count: resultCount }))
      }

      searchState.value = 'idle'
      currentSearchGid.value = ''
      searchCancelled.value = false
      searchCleanupDone.value = false
      searchElapsedMs.value = 0
    }
  }

  return {
    searchKeyword,
    searchFileType,
    searchMinSources,
    searchState,
    searchResults,
    searchElapsedMs,
    searchActive,
    runSearch,
  }
}
