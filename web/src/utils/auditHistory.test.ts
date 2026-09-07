import { describe, expect, it } from 'vitest';
import {
  buildAuditQuery,
  formatAuditDateTime,
  getAuditActionColor,
  getAuditActionLabel,
} from './auditHistory';

describe('audit history view model', () => {
  it('maps operation types to the labels and colors used by the UI', () => {
    expect(getAuditActionLabel('update')).toBe('修改');
    expect(getAuditActionColor('delete')).toBe('red');
    expect(getAuditActionLabel('custom')).toBe('custom');
    expect(getAuditActionColor('custom')).toBeUndefined();
  });

  it('formats API timestamps for the operation history table', () => {
    expect(formatAuditDateTime('2026-09-05T10:21:45+08:00')).toBe('2026-09-05 10:21');
  });

  it('keeps only selected filters in the API query', () => {
    expect(
      buildAuditQuery({
        page: 2,
        pageSize: 20,
        startDate: '2026-09-01',
        endDate: '2026-09-08',
        managementScope: '指定年级',
        action: 'update',
      }),
    ).toEqual({
      page: 2,
      page_size: 20,
      start_date: '2026-09-01',
      end_date: '2026-09-08',
      management_scope: '指定年级',
      action: 'update',
    });

    expect(buildAuditQuery({ page: 1, pageSize: 20 })).toEqual({ page: 1, page_size: 20 });
  });
});
