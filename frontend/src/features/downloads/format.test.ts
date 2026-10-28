import { describe, expect, it } from 'vitest';

import {
  canPause,
  canRemove,
  canResume,
  countByCategory,
  filterByCategory,
  formatBytes,
  formatEta,
  formatPercent,
  formatSpeed,
  isActiveStatus,
  isFailedStatus,
  isFinishedStatus,
} from './format';

interface Item {
  id: string;
  status: string;
}

const ITEMS: Item[] = [
  { id: '1', status: 'active' },
  { id: '2', status: 'waiting' },
  { id: '3', status: 'paused' },
  { id: '4', status: 'complete' },
  { id: '5', status: 'error' },
  { id: '6', status: 'removed' },
];

describe('the status helpers', () => {
  it('knows the running statuses', () => {
    expect(isActiveStatus('active')).toBe(true);
    expect(isActiveStatus('waiting')).toBe(true);
    expect(isActiveStatus('paused')).toBe(true);
    expect(isActiveStatus('complete')).toBe(false);
    expect(isActiveStatus('error')).toBe(false);
  });

  it('knows the finished statuses', () => {
    expect(isFinishedStatus('complete')).toBe(true);
    expect(isFinishedStatus('error')).toBe(true);
    expect(isFinishedStatus('removed')).toBe(true);
    expect(isFinishedStatus('active')).toBe(false);
  });

  it('knows the failed statuses', () => {
    expect(isFailedStatus('error')).toBe(true);
    expect(isFailedStatus('removed')).toBe(true);
    expect(isFailedStatus('complete')).toBe(false);
  });
});

describe('the categories', () => {
  it('keeps everything in the all category', () => {
    expect(filterByCategory(ITEMS, 'all')).toHaveLength(ITEMS.length);
  });

  it('keeps the running ones in the active category', () => {
    const filtered = filterByCategory(ITEMS, 'active');
    expect(filtered.map((item) => item.id)).toEqual(['1', '2', '3']);
  });

  it('keeps the finished ones in the completed category', () => {
    const filtered = filterByCategory(ITEMS, 'completed');
    expect(filtered.map((item) => item.id)).toEqual(['4']);
  });

  it('keeps the failed ones in the failed category', () => {
    const filtered = filterByCategory(ITEMS, 'failed');
    expect(filtered.map((item) => item.id)).toEqual(['5', '6']);
  });

  it('counts every category', () => {
    const counts = countByCategory(ITEMS);
    expect(counts.all).toBe(6);
    expect(counts.active).toBe(3);
    expect(counts.completed).toBe(1);
    expect(counts.failed).toBe(2);
  });
});

describe('the formatters', () => {
  it('formats the byte sizes', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(512)).toBe('512 B');
    expect(formatBytes(1024)).toBe('1 KiB');
    expect(formatBytes(1536)).toBe('1.5 KiB');
    expect(formatBytes(1024 * 1024)).toBe('1 MiB');
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1 GiB');
  });

  it('formats a speed', () => {
    expect(formatSpeed(0)).toBe('—');
    expect(formatSpeed(1024)).toBe('1 KiB/s');
    expect(formatSpeed(1024 * 1024)).toBe('1 MiB/s');
  });

  it('formats the progress', () => {
    expect(formatPercent(0)).toBe('0%');
    expect(formatPercent(0.5)).toBe('50%');
    expect(formatPercent(1)).toBe('100%');
    expect(formatPercent(2)).toBe('100%');
  });

  it('estimates the remaining time', () => {
    expect(formatEta(0, 1000, 100)).toBe('10s');
    expect(formatEta(0, 6000, 100)).toBe('1m 0s');
    expect(formatEta(0, 360000, 100)).toBe('1h 0m');
  });

  it('has no estimate without a size or a speed', () => {
    expect(formatEta(0, 0, 100)).toBe('—');
    expect(formatEta(0, 1000, 0)).toBe('—');
    expect(formatEta(1000, 1000, 100)).toBe('—');
  });
});

describe('the action guards', () => {
  it('pauses the running downloads', () => {
    expect(canPause('active')).toBe(true);
    expect(canPause('waiting')).toBe(true);
    expect(canPause('paused')).toBe(false);
  });

  it('resumes the paused downloads', () => {
    expect(canResume('paused')).toBe(true);
    expect(canResume('active')).toBe(false);
  });

  it('removes the downloads that have come to rest', () => {
    expect(canRemove('complete')).toBe(true);
    expect(canRemove('error')).toBe(true);
    expect(canRemove('active')).toBe(false);
    expect(canRemove('paused')).toBe(false);
  });
});
