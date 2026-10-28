<script setup lang="ts">
/**
 * @fileoverview The BitMaher header.
 *
 * The header names the application, shows the speed of all the downloads
 * together, and holds the two controls that are not about the downloads:
 * the settings and the about panel.
 *
 * The icon is the application's own, so that the window reads as a
 * native one.
 */
import { computed } from 'vue';
import { useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { NIcon } from 'naive-ui';
import { InformationCircleOutline, SettingsOutline } from '@vicons/ionicons5';
import { useAppStore } from '@/stores/app';
import { bytesToSize } from '@/shared/utils/format';

const emit = defineEmits<{ 'show-about': [] }>();

const { t } = useI18n();
const router = useRouter();
const appStore = useAppStore();

/** The speed of all the downloads together. */
const speed = computed(() => ({
  download: appStore.stat?.downloadSpeed ?? 0,
  upload: appStore.stat?.uploadSpeed ?? 0,
}));

/** How many downloads are running. */
const activeCount = computed(() => appStore.stat?.numActive ?? 0);
</script>

<template>
  <header class="app-header" data-tauri-drag-region>
    <img class="app-logo" src="/appicon.png" alt="" aria-hidden="true" />
    <span class="app-brand">
      BitMaher
      <small>Wails v3 · PocketBase · aria2-next</small>
    </span>

    <span class="app-header-spacer" />

    <span class="app-speed" :title="t('app.total-speed')">
      {{ bytesToSize(speed.download) }}/s down · {{ bytesToSize(speed.upload) }}/s up
      <span v-if="activeCount > 0" class="app-speed-active">· {{ activeCount }} active</span>
    </span>

    <button
      type="button"
      class="btn btn-sm"
      :title="t('navigation.about')"
      @click="emit('show-about')"
    >
      <NIcon :component="InformationCircleOutline" />
    </button>

    <button
      type="button"
      class="btn btn-sm"
      :title="t('navigation.settings')"
      @click="router.push('/preference')"
    >
      <NIcon :component="SettingsOutline" />
    </button>
  </header>
</template>

<style scoped>
/*
 * The header is the application's own: a band across the top, with the
 * icon and the name on the left and the controls on the right.
 *
 * Every size is in rem, so that the header follows the zoom factor,
 * which scales the root font size.
 */
.app-header {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 1.2308rem;
  padding: 0 1.2308rem;
  background: var(--sidebar-bg);
  border-bottom: 0.0769rem solid var(--panel-border);
}

.app-logo {
  /* The icon is square and it is never stretched: the header is a flex
   * row, so it would otherwise grow with the row. */
  flex: none;
  width: 1.6923rem;
  height: 1.6923rem;
  /* The artwork has a transparent background, so it floats on the
   * header without a chip around it. */
  object-fit: contain;
  /* The icon belongs to the title, so it sits closer to it than the
   * header's own spacing would put it. */
  margin-right: -0.5385rem;
}

.app-brand {
  font-weight: 600;
  display: flex;
  flex-direction: column;
  line-height: 1.2;
  user-select: none;
}

.app-brand small {
  font-weight: 400;
  font-size: var(--font-size-xs);
  color: var(--text-muted);
}

.app-header-spacer {
  flex: 1;
}

.app-speed {
  font-variant-numeric: tabular-nums;
  color: var(--text-muted);
  font-size: var(--font-size-sm);
  white-space: nowrap;
}

.app-speed-active {
  color: var(--accent-color, #6366f1);
}
</style>
