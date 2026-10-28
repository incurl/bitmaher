<script lang="ts">
/**
 * @fileoverview MTooltip — project tooltip wrapper with a fixed delay.
 *
 * Wraps Naive UI's `NTooltip` to enforce a consistent show delay across the
 * entire application.  Every tooltip waits 500 ms before appearing, with no
 * cooldown or instant-reshow behaviour.
 *
 * Usage is identical to NTooltip — all props, slots, and events are passed
 * through transparently.
 *
 * @example
 * ```vue
 * <MTooltip placement="right">
 *   <template #trigger>
 *     <button>hover me</button>
 *   </template>
 *   Brief tooltip text
 * </MTooltip>
 * ```
 */

/** Default configuration constants, exported for testing. */
export const TOOLTIP_DEFAULTS = {
  /** Delay before tooltip appears (ms). */
  delay: 500,
} as const
</script>

<script setup lang="ts">
/**
 * MTooltip component — wraps NTooltip with a fixed delay.
 *
 * All NTooltip props are accepted.  If `delay` is not explicitly passed,
 * the default (500 ms) is applied.
 */
import { NTooltip } from 'naive-ui'

const props = withDefaults(
  defineProps<{
    /** Show delay in ms.  Defaults to TOOLTIP_DEFAULTS.delay (500ms). */
    delay?: number
    /** Tooltip placement.  Passed through to NTooltip. */
    placement?:
      | 'top'
      | 'bottom'
      | 'left'
      | 'right'
      | 'top-start'
      | 'top-end'
      | 'bottom-start'
      | 'bottom-end'
      | 'left-start'
      | 'left-end'
      | 'right-start'
      | 'right-end'
    /** Whether the tooltip is disabled.  Passed through to NTooltip. */
    disabled?: boolean
    /** Duration the tooltip remains visible in ms.  Passed through. */
    duration?: number
  }>(),
  {
    delay: TOOLTIP_DEFAULTS.delay,
    placement: undefined,
    disabled: undefined,
    duration: undefined,
  },
)
</script>

<template>
  <NTooltip :delay="props.delay" :placement="placement" :disabled="disabled" :duration="duration">
    <template #trigger>
      <slot name="trigger" />
    </template>
    <slot />
  </NTooltip>
</template>
