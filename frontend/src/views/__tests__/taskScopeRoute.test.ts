import { describe, it, expect } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { RouterView, createRouter, createMemoryHistory } from 'vue-router'
import { defineComponent, h } from 'vue'

// The view reports the status the router gave it, so that the test sees
// what the application sees.
const StubView = defineComponent({
  props: { status: { type: String, default: 'all' } },
  setup(props) {
    return () => h('div', { 'data-scope': props.status })
  },
})

// The shape of the layout: a router-view whose slot renders the view.
const Layout = defineComponent({
  setup() {
    return () =>
      h(RouterView, null, {
        default: ({ Component }: { Component: unknown }) => h(Component as never, { key: 'task' }),
      })
  },
})

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/task/:status(all|progress|failed|completed)?',
        name: 'task',
        component: StubView,
        props: true,
      },
    ],
  })
}

describe('the status of the route', () => {
  it.each([
    ['/task/all', 'all'],
    ['/task/progress', 'progress'],
    ['/task/failed', 'failed'],
    ['/task/completed', 'completed'],
  ])('reaches the view: %s', async (path, expected) => {
    const router = createTestRouter()
    await router.push(path)
    await router.isReady()

    const wrapper = mount(Layout, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.attributes('data-scope')).toBe(expected)
  })
})
