import { describe, expect, it } from 'vitest';
import { formatDate, formatDateTime } from './datetime';

describe('datetime formatting', () => {
  it('converts UTC instants to the requested time zone', () => {
    expect(formatDateTime('2026-10-20T02:56:30Z', { timeZone: 'Asia/Shanghai' })).toBe(
      '2026-10-20 10:56',
    );
  });

  it('carries over to the next local day when the UTC instant crosses midnight', () => {
    expect(formatDateTime('2026-10-20T20:30:00Z', { timeZone: 'Asia/Shanghai' })).toBe(
      '2026-10-21 04:30',
    );
  });

  it('includes seconds only when requested', () => {
    expect(
      formatDateTime('2026-10-20T02:56:30Z', {
        timeZone: 'Asia/Shanghai',
        seconds: true,
      }),
    ).toBe('2026-10-20 10:56:30');
  });

  it('uses the browser local time zone by default', () => {
    const localTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    expect(formatDateTime('2026-10-20T02:56:30Z')).toBe(
      formatDateTime('2026-10-20T02:56:30Z', { timeZone: localTimeZone }),
    );
  });

  it('falls back for empty values and keeps invalid strings as-is', () => {
    expect(formatDateTime('')).toBe('—');
    expect(formatDateTime(null)).toBe('—');
    expect(formatDateTime(undefined)).toBe('—');
    expect(formatDateTime('', { fallback: '未填' })).toBe('未填');
    expect(formatDateTime('not-a-date')).toBe('not-a-date');
  });

  it('formats date-only values in the requested time zone', () => {
    expect(formatDate('2026-10-20T20:30:00Z', { timeZone: 'Asia/Shanghai' })).toBe('2026-10-21');
    expect(formatDate('2026-10-20T02:56:30Z', { timeZone: 'Asia/Shanghai' })).toBe('2026-10-20');
    expect(formatDate(undefined)).toBe('—');
    expect(formatDate('not-a-date')).toBe('not-a-date');
  });
});
