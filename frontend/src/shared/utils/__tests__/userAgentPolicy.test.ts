import { describe, expect, it } from 'vitest'
import {
  buildUserAgentSelectionItems,
  findMatchingUserAgentRule,
  normalizeUserAgentProfiles,
  normalizeUserAgentRules,
  resolveUserAgent,
} from '../userAgentPolicy'
import type { UserAgentProfile, UserAgentRule } from '@shared/types'

const profiles: UserAgentProfile[] = [
  {
    id: 'quark',
    name: 'Quark Drive',
    value: 'QuarkUA/1.0',
    createdAt: 1,
    updatedAt: 1,
  },
  {
    id: 'baidu',
    name: 'Baidu Netdisk',
    value: 'BaiduUA/1.0',
    createdAt: 2,
    updatedAt: 2,
  },
  {
    id: 'github',
    name: 'GitHub Assets',
    value: 'GitHubUA/1.0',
    createdAt: 3,
    updatedAt: 3,
  },
]

const rules: UserAgentRule[] = [
  {
    id: 'quark-rule',
    enabled: true,
    hostPattern: '*.quark.cn',
    profileId: 'quark',
    overridePlugin: false,
    createdAt: 1,
    updatedAt: 1,
  },
  {
    id: 'baidu-rule',
    enabled: true,
    hostPattern: 'pan.baidu.com',
    profileId: 'baidu',
    overridePlugin: true,
    createdAt: 2,
    updatedAt: 2,
  },
]

describe('userAgentPolicy', () => {
  it('keeps only valid saved profiles and dedupes ids', () => {
    const result = normalizeUserAgentProfiles([
      profiles[0],
      { id: 'quark', name: 'Duplicate', value: 'DuplicateUA/1.0', createdAt: 3, updatedAt: 3 },
      { id: 'empty-value', name: 'Empty', value: '', createdAt: 4, updatedAt: 4 },
      { id: 'bad-name', name: '', value: 'BadUA/1.0', createdAt: 5, updatedAt: 5 },
    ])

    expect(result).toEqual([profiles[0]])
  })

  it('keeps only rules that reference a saved profile', () => {
    const result = normalizeUserAgentRules(
      [
        rules[0],
        { ...rules[1], profileId: 'missing' },
        { ...rules[1], id: 'bad-host', hostPattern: 'https://pan.baidu.com/path' },
      ],
      profiles,
    )

    expect(result).toEqual([rules[0]])
  })

  it('accepts wildcard host patterns and rejects URL-shaped patterns', () => {
    expect(normalizeUserAgentRules([{ ...rules[0], hostPattern: '*.Example.com' }], profiles)).toEqual([
      { ...rules[0], hostPattern: '*.example.com' },
    ])

    expect(normalizeUserAgentRules([{ ...rules[0], hostPattern: 'https://example.com/file.zip' }], profiles)).toEqual(
      [],
    )
  })

  it('matches host rules against final url, source url, and referer', () => {
    expect(
      findMatchingUserAgentRule({
        url: 'https://download.example.com/file.zip',
        finalUrl: 'https://cdn.quark.cn/file.zip',
        referer: '',
        rules,
        profiles,
      })?.profile.id,
    ).toBe('quark')

    expect(
      findMatchingUserAgentRule({
        url: 'https://download.example.com/file.zip',
        referer: 'https://pan.baidu.com/s/abc',
        rules,
        profiles,
      })?.profile.id,
    ).toBe('baidu')
  })

  it('preserves plugin user agent unless a matching rule is configured to override it', () => {
    expect(
      resolveUserAgent({
        manualUserAgent: '',
        pluginUserAgent: 'BrowserUA/1.0',
        defaultUserAgent: 'DefaultUA/1.0',
        url: 'https://cdn.quark.cn/file.zip',
        profiles,
        rules,
      }).userAgent,
    ).toBe('BrowserUA/1.0')

    expect(
      resolveUserAgent({
        manualUserAgent: '',
        pluginUserAgent: 'BrowserUA/1.0',
        defaultUserAgent: 'DefaultUA/1.0',
        url: 'https://pan.baidu.com/file.zip',
        profiles,
        rules,
      }).userAgent,
    ).toBe('BaiduUA/1.0')
  })

  it('uses normal matching rules only when manual and plugin user agents are empty', () => {
    const result = resolveUserAgent({
      manualUserAgent: '',
      pluginUserAgent: '',
      defaultUserAgent: 'DefaultUA/1.0',
      url: 'https://cdn.quark.cn/file.zip',
      profiles,
      rules,
    })

    expect(result).toMatchObject({
      userAgent: 'QuarkUA/1.0',
      source: 'rule',
      profileId: 'quark',
      ruleId: 'quark-rule',
    })
  })

  it('builds matched, recent, and remaining saved profile picker items without raw UA strings', () => {
    const result = buildUserAgentSelectionItems({
      url: 'https://cdn.quark.cn/file.zip',
      profiles,
      rules,
      recentProfileIds: ['baidu', 'github', 'quark', 'missing'],
      maxRecent: 2,
    })

    expect(result).toEqual([
      { section: 'matched', profile: profiles[0], rule: rules[0] },
      { section: 'recent', profile: profiles[1] },
      { section: 'recent', profile: profiles[2] },
    ])
  })

  it('lists saved profiles alphabetically when no rule or recent profile matches', () => {
    const result = buildUserAgentSelectionItems({
      url: 'https://example.com/file.zip',
      profiles: [profiles[0], profiles[2], profiles[1]],
      rules,
      recentProfileIds: [],
    })

    expect(result).toEqual([
      { section: 'saved', profile: profiles[1] },
      { section: 'saved', profile: profiles[2] },
      { section: 'saved', profile: profiles[0] },
    ])
  })
})
