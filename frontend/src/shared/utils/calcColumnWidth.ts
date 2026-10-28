/**
 * @fileoverview Data-driven column width calculator for NDataTable.
 *
 * Computes the minimum fixed `width` a column needs so that **neither** its
 * header label **nor** any of its cell values are truncated or wrapped.
 *
 * ## Algorithm
 *
 * ```
 * width = max(
 *   measureText(title, headerFont) + sortArrowWidth,
 *   max(...values.map(v => measureText(v, bodyFont)))
 * ) + cellPadding + safetyMargin
 * ```
 *
 * Uses the Canvas 2D `measureText` API for sub-pixel accuracy without
 * triggering DOM layout/reflow.  Falls back to a character-width heuristic
 * when `CanvasRenderingContext2D` is unavailable (e.g. jsdom in Vitest).
 *
 * ## Usage
 *
 * ```ts
 * const cols = computed(() => [
 *   {
 *     title: t('task.file-size'),
 *     key: 'length',
 *     width: calcColumnWidth({
 *       title: t('task.file-size'),
 *       values: rows.value.map(r => bytesToSize(r.length)),
 *       sortable: true,
 *     }),
 *   },
 *   // "flex" column — absorbs remaining space
 *   { title: t('task.file-name'), key: 'name' },
 * ])
 * ```
 */

// ── Constants ────────────────────────────────────────────────────────

/** NDataTable header: medium-weight 14px system font. */
const HEADER_FONT = '500 14px system-ui, sans-serif'

/** NDataTable body: normal-weight 14px system font. */
const BODY_FONT = '400 14px system-ui, sans-serif'

/** Width of the sort-arrow icon NDataTable renders for sortable columns. */
const SORT_ARROW_WIDTH = 18

/** Default horizontal cell padding (left + right) for `size="small"`. */
const DEFAULT_PADDING = 24

/** Extra pixels added after rounding to avoid sub-pixel clipping. */
const SAFETY_MARGIN = 2

// ── Canvas singleton ─────────────────────────────────────────────────

/** Lazily-initialized offscreen canvas for text measurement. */
let canvas: HTMLCanvasElement | null = null

/**
 * Returns a 2D rendering context, or `null` in environments that do
 * not support `<canvas>` (e.g. jsdom).
 */
function getContext(): CanvasRenderingContext2D | null {
  if (!canvas) {
    canvas = document.createElement('canvas')
  }
  return canvas.getContext('2d')
}

// ── Measurement helpers ──────────────────────────────────────────────

/** Regex matching CJK Unified Ideographs and common CJK ranges. */
const CJK_RANGE = /[\u2E80-\u9FFF\uF900-\uFAFF]/

/**
 * Measures the rendered pixel width of `text` using either Canvas 2D
 * (preferred) or a character-width heuristic (fallback).
 */
function measureText(text: string, font: string, ctx: CanvasRenderingContext2D | null): number {
  if (ctx) {
    ctx.font = font
    return ctx.measureText(text).width
  }
  // Heuristic: CJK ≈ 14px, Latin/ASCII ≈ 8px at 14px font size.
  return [...text].reduce((w, ch) => w + (CJK_RANGE.test(ch) ? 14 : 8), 0)
}

// ── Public API ───────────────────────────────────────────────────────

export interface CalcColumnWidthOptions {
  /** Column header label (already translated via `t()`). */
  title: string

  /**
   * Display values of every row for this column **after** formatting.
   *
   * For columns with a `render` function (e.g. `bytesToSize`), pass the
   * formatted strings, not the raw numbers.
   *
   * When empty or omitted, the width is based on the title alone.
   */
  values?: string[]

  /** Whether the column has a `sorter` (adds a sort-arrow icon). */
  sortable?: boolean

  /**
   * Extra pixel width to accommodate non-text content inside cells,
   * such as NTag internal padding or icon gutters.
   *
   * Example: NTag `size="small"` adds ~20px (10px left + 10px right).
   */
  extraWidth?: number

  /** Override header font.  @default '500 14px system-ui, sans-serif' */
  headerFont?: string

  /** Override body font.  @default '400 14px system-ui, sans-serif' */
  bodyFont?: string

  /** Override cell padding (left + right).  @default 24 */
  padding?: number
}

/**
 * Computes the optimal fixed `width` for an NDataTable column.
 *
 * The result is the smallest integer pixel value that can display both
 * the header label and the widest cell value without wrapping or
 * truncation.
 *
 * **Design contract**: columns returned with a computed `width` are
 * "fixed" — they do not participate in NDataTable's remaining-space
 * distribution.  This leaves the maximum available width for "flex"
 * columns (file names, URLs) that omit `width` and `minWidth`.
 */
export function calcColumnWidth(options: CalcColumnWidthOptions): number {
  const {
    title,
    values = [],
    sortable = false,
    extraWidth = 0,
    headerFont = HEADER_FONT,
    bodyFont = BODY_FONT,
    padding = DEFAULT_PADDING,
  } = options

  const ctx = getContext()
  const arrowWidth = sortable ? SORT_ARROW_WIDTH : 0

  // Header contribution: text width + sort arrow.
  const headerWidth = measureText(title, headerFont, ctx) + arrowWidth

  // Body contribution: widest formatted value.
  let maxBodyWidth = 0
  for (const value of values) {
    const w = measureText(value, bodyFont, ctx)
    if (w > maxBodyWidth) maxBodyWidth = w
  }

  // The column must accommodate whichever is wider.
  const contentWidth = Math.max(headerWidth, maxBodyWidth + extraWidth)

  return Math.ceil(contentWidth + padding + SAFETY_MARGIN)
}
