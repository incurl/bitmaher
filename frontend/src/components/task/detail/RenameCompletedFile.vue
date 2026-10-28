<script setup lang="ts">
/** @fileoverview Rename a completed file through the native history owner. */
import { computed, ref } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { NButton, NIcon, NInput, NPopover } from 'naive-ui'
import { CreateOutline } from '@vicons/ionicons5'
import { useI18n } from 'vue-i18n'
import type { Aria2Task } from '@shared/types'
import { getErrorMessage } from '@shared/utils/errorMessage'

const props = defineProps<{ task: Aria2Task }>()
const emit = defineEmits<{ renamed: [] }>()
const { t } = useI18n()
const visible = ref(false)
const name = ref('')
const busy = ref(false)
const error = ref('')
const available = computed(
  () => props.task.status === 'complete' && props.task.seeder !== 'true' && props.task.files?.length === 1,
)
function open(show: boolean) {
  if (busy.value) return
  visible.value = show
  if (show) {
    name.value = props.task.files[0].path.split(/[/\\]/).pop() ?? ''
    error.value = ''
  }
}
async function save() {
  if (busy.value || !name.value.trim()) return
  busy.value = true
  error.value = ''
  try {
    await invoke('rename_completed_file', { gid: props.task.gid, name: name.value })
    visible.value = false
    emit('renamed')
  } catch (cause) {
    error.value = getErrorMessage(cause)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <NPopover v-if="available" trigger="click" placement="bottom-start" :show="visible" @update:show="open">
    <template #trigger>
      <NButton quaternary size="tiny" :aria-label="t('task.task-out')"
        ><NIcon :size="14"><CreateOutline /></NIcon
      ></NButton>
    </template>
    <form class="rename-file" @submit.prevent="save">
      <label>{{ t('task.file-name') }}<NInput v-model:value="name" autofocus :disabled="busy" /></label>
      <span v-if="error" role="alert">{{ error }}</span>
      <NButton attr-type="submit" size="small" type="primary" :loading="busy" :disabled="!name.trim()">{{
        t('app.confirm')
      }}</NButton>
    </form>
  </NPopover>
</template>

<style scoped>
.rename-file {
  display: grid;
  gap: 12px;
  width: min(320px, 70vw);
  overflow-wrap: anywhere;
}
.rename-file label {
  display: grid;
  gap: 6px;
}
.rename-file [role='alert'] {
  color: var(--color-error);
}
</style>
