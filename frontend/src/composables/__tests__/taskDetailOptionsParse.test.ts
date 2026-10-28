/**
 * @fileoverview Tests for pure header utility functions exported from
 * useTaskDetailOptions.  These are framework-free functions that can be
 * verified without Vue reactivity.
 */
import { describe, it, expect } from 'vitest'
import { parseHeaders, buildHeaders } from '@/composables/useTaskDetailOptions'

// ── parseHeaders ────────────────────────────────────────────────────

describe('parseHeaders', () => {
  it('extracts Cookie from header string', () => {
    const result = parseHeaders('Cookie: session=abc')
    expect(result.cookie).toBe('session=abc')
    expect(result.authorization).toBe('')
  })

  it('extracts Authorization from header string', () => {
    const result = parseHeaders('Authorization: Bearer token123')
    expect(result.authorization).toBe('Bearer token123')
  })

  it('handles empty string', () => {
    const result = parseHeaders('')
    expect(result.cookie).toBe('')
    expect(result.authorization).toBe('')
  })

  it('handles undefined input', () => {
    const result = parseHeaders(undefined as unknown as string)
    expect(result.cookie).toBe('')
    expect(result.authorization).toBe('')
  })

  it('extracts both from multi-line header', () => {
    const result = parseHeaders('Cookie: abc\nAuthorization: Bearer xyz')
    expect(result.cookie).toBe('abc')
    expect(result.authorization).toBe('Bearer xyz')
  })

  it('handles header array', () => {
    const result = parseHeaders(['Cookie: session=abc', 'Authorization: Bearer xyz'])
    expect(result.cookie).toBe('session=abc')
    expect(result.authorization).toBe('Bearer xyz')
  })

  it('ignores non-Cookie/Authorization headers', () => {
    const result = parseHeaders(['X-Custom: value', 'Cookie: abc'])
    expect(result.cookie).toBe('abc')
    expect(result.authorization).toBe('')
  })

  it('handles header with whitespace after colon', () => {
    const result = parseHeaders('Cookie:   abc')
    expect(result.cookie).toBe('abc')
  })
})

// ── buildHeaders ────────────────────────────────────────────────────

describe('buildHeaders', () => {
  it('builds header array with both values', () => {
    expect(buildHeaders('session=abc', 'Bearer xyz')).toEqual(['Cookie: session=abc', 'Authorization: Bearer xyz'])
  })

  it('sanitizes Cookie and Authorization values before building headers', () => {
    expect(buildHeaders('session=abc\r\nX-Evil: 1', 'Bearer xyz\nInjected: bad')).toEqual([
      'Cookie: session=abcX-Evil: 1',
      'Authorization: Bearer xyzInjected: bad',
    ])
  })

  it('builds array with only Cookie', () => {
    expect(buildHeaders('session=abc', '')).toEqual(['Cookie: session=abc'])
  })

  it('builds array with only Authorization', () => {
    expect(buildHeaders('', 'Bearer xyz')).toEqual(['Authorization: Bearer xyz'])
  })

  it('returns empty array when both are empty', () => {
    expect(buildHeaders('', '')).toEqual([])
  })
})
