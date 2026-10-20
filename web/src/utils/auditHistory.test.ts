import { describe, expect, it } from 'vitest';
import {
  buildAuditQuery,
  formatAuditDateTime,
  getAuditBatchRecordSummary,
  getAuditBatchVisibleFields,
  getAuditActionColor,
  getAuditActionLabel,
} from './auditHistory';
import { formatDateTime } from './datetime';

describe('audit history view model', () => {
  it('maps operation types to the labels and colors used by the UI', () => {
    expect(getAuditActionLabel('update')).toBe('修改');
    expect(getAuditActionColor('delete')).toBe('red');
    expect(getAuditActionLabel('custom')).toBe('custom');
    expect(getAuditActionColor('custom')).toBeUndefined();
  });

  it('formats API timestamps for the operation history table', () => {
    expect(formatAuditDateTime('2026-09-05T10:21:45+08:00')).toBe('2026-09-05 10:21');
    const localTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    expect(formatAuditDateTime('2026-10-20T02:56:30Z')).toBe(
      formatDateTime('2026-10-20T02:56:30Z', { timeZone: localTimeZone }),
    );
  });

  it('converts selected local dates to UTC instants for the API filter', () => {
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
      start_at: new Date(2026, 8, 1, 0, 0, 0, 0).toISOString(),
      end_at: new Date(2026, 8, 9, 0, 0, 0, 0).toISOString(),
      management_scope: '指定年级',
      action: 'update',
    });

    expect(buildAuditQuery({ page: 1, pageSize: 20 })).toEqual({ page: 1, page_size: 20 });
  });

  it('uses the next local day as the exclusive end boundary', () => {
    const query = buildAuditQuery({
      page: 1,
      pageSize: 20,
      startDate: '2026-10-20',
      endDate: '2026-10-20',
    });

    expect(query.start_at).toBe(new Date(2026, 9, 20, 0, 0, 0, 0).toISOString());
    expect(query.end_at).toBe(new Date(2026, 9, 21, 0, 0, 0, 0).toISOString());
  });

  it('shows five public batch-import fields without sensitive permission', () => {
    const fields = getAuditBatchVisibleFields(undefined, false);

    expect(fields.map((field) => field.field_name)).toEqual([
      'name',
      'grade',
      'class_name',
      'major',
      'training_mode',
    ]);
  });

  it('adds the five sensitive fields only with sensitive permission', () => {
    const fields = getAuditBatchVisibleFields(undefined, true);

    expect(fields.map((field) => field.field_name)).toEqual([
      'name',
      'grade',
      'class_name',
      'major',
      'training_mode',
      'mobile',
      'email',
      'work_unit',
      'position',
      'mailing_address',
    ]);
  });

  it('reports truncated batch records with the real created total', () => {
    const summary = getAuditBatchRecordSummary({
      batch_created_alumni: new Array(200).fill({}),
      batch_created_alumni_total: 5000,
      batch_created_alumni_truncated: true,
    });

    expect(summary).toEqual({ displayed: 200, total: 5000, truncated: true });
  });

  it('does not report truncation for legacy records without new fields', () => {
    const summary = getAuditBatchRecordSummary({
      batch_created_alumni: new Array(3).fill({}),
    });

    expect(summary).toEqual({ displayed: 3, total: 3, truncated: false });
  });

  it('infers truncation when the total exceeds the returned records', () => {
    const summary = getAuditBatchRecordSummary({
      batch_created_alumni: new Array(2).fill({}),
      batch_created_alumni_total: 10,
    });

    expect(summary).toEqual({ displayed: 2, total: 10, truncated: true });
  });
});
