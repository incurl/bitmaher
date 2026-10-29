<script setup lang="ts">
/**
 * @fileoverview The add-task dialog of the single window.
 *
 * Rayburst shows the add-task form in a window of its own. BitMaher
 * runs in a single window, so the form is shown in a modal instead.
 *
 * The form itself is the ported component, unchanged: it is the same
 * `AddTask.vue` that Rayburst mounts in its add window. This component
 * only gives it a place to live.
 *
 * The store holds the batch that the form shows, so this component
 * only has to say whether the form is on the screen.
 */
import { NModal } from 'naive-ui'
import AddTask from '@/components/task/AddTask.vue'
import { useAppStore } from '@/stores/app'
import { logger } from '@shared/logger'

const appStore = useAppStore()

/**
 * Closes the dialog.
 *
 * The store is told, so that it does not open the dialog again on the
 * next tick with the same items.
 */
function close(): void {
  appStore.hideAddTaskDialog()
}

/**
 * The form was submitted.
 *
 * The downloads are in the list now, and the dialog has done its work.
 */
function submitted(): void {
  close()
}

// The forwarder: what the application calls when it wants the dialog.
//
// In Rayburst the answer was a window of its own. Here it is the modal
// below, and the store's pending batch is what the form shows.
appStore.setAddTaskForwarder((forward) => {
  try {
    if (forward.items?.length) {
      appStore.enqueueLocalBatch(forward.items)
    } else {
      appStore.showLocalAddTaskDialog()
    }
  } catch (error) {
    logger.warn('AddTaskDialog.open', error)
  }
})
</script>

<template>
  <NModal
    :show="appStore.addTaskVisible"
    preset="card"
    style="width: min(900px, 92vw)"
    transform-origin="center"
    :mask-closable="false"
    @update:show="(v: boolean) => { if (!v) close() }"
  >
    <AddTask :show="appStore.addTaskVisible" @close="close" @submitted="submitted" />
  </NModal>
</template>
