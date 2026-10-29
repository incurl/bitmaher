/**
 * @fileoverview The add-task dialog of the single window.
 *
 * The dialog used to be a window of its own, which BitMaher does not
 * have: the request was forwarded to a window that was never there, and
 * nothing happened at all.
 *
 * These tests pin that the dialog opens in this window, and that the
 * form is what it shows.
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en-US' } }),
}))

vi.mock('naive-ui', () => ({
  NModal: {
    props: ['show'],
    emits: ['update:show'],
    template: '<div v-if="show" class="modal-stub"><slot /></div>',
  },
}))

// The form is the ported component. It is not what this test is about:
// it is stubbed, so that the test is about the dialog around it.
vi.mock('../AddTask.vue', () => ({
  default: {
    name: 'AddTask',
    props: ['show'],
    emits: ['close', 'submitted'],
    template: '<div class="add-task-stub" :data-show="String(show)"></div>',
  },
}))

import AddTaskDialog from '../AddTaskDialog.vue'
import { useAppStore } from '@/stores/app'

beforeEach(() => {
  setActivePinia(createPinia())
})

/** Mounts the dialog. */
function mountDialog() {
  return mount(AddTaskDialog)
}

describe('the add-task dialog', () => {
  it('is closed at first', () => {
    const wrapper = mountDialog()
    expect(wrapper.find('.modal-stub').exists()).toBe(false)
  })

  it('opens when the application asks for it', async () => {
    const store = useAppStore()
    const wrapper = mountDialog()

    store.showAddTaskDialog()
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.modal-stub').exists()).toBe(true)
  })

  it('shows the form', async () => {
    const store = useAppStore()
    const wrapper = mountDialog()

    store.showAddTaskDialog()
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.add-task-stub').exists()).toBe(true)
    expect(wrapper.find('.add-task-stub').attributes('data-show')).toBe('true')
  })

  it('closes when the form is submitted', async () => {
    const store = useAppStore()
    const wrapper = mountDialog()
    store.showAddTaskDialog()
    await wrapper.vm.$nextTick()

    wrapper.findComponent({ name: 'AddTask' }).vm.$emit('submitted')
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.modal-stub').exists()).toBe(false)
  })

  it('closes when the form is dismissed', async () => {
    const store = useAppStore()
    const wrapper = mountDialog()
    store.showAddTaskDialog()
    await wrapper.vm.$nextTick()

    wrapper.findComponent({ name: 'AddTask' }).vm.$emit('close')
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.modal-stub').exists()).toBe(false)
  })

  it('puts the forwarded items in the batch', async () => {
    const store = useAppStore()
    const wrapper = mountDialog()

    store.enqueueBatch([
      {
        id: 'a',
        kind: 'uri',
        source: 'https://example.com/a.bin',
        displayName: 'a.bin',
        payload: 'https://example.com/a.bin',
        status: 'pending',
      },
    ])
    await wrapper.vm.$nextTick()

    expect(store.pendingBatch).toHaveLength(1)
    expect(wrapper.find('.modal-stub').exists()).toBe(true)
  })
})
