import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  buildAuditQuery,
  formatAuditDateTime,
  getAuditBatchRecordSummary,
  getAuditBatchVisibleFields,
  getAuditActionColor,
  getAuditActionLabel,
} from './auditHistory';

describe.each([
  {
    zone: 'Asia/Shanghai',
    formatted: '2026-10-20 10:56',
    crossover: '2026-10-21 04:30',
    start: '2026-10-19T16:00:00.000Z',
    end: '2026-10-20T16:00:00.000Z',
  },
  {
    zone: 'UTC',
    formatted: '2026-10-20 02:56',
    crossover: '2026-10-20 20:30',
    start: '2026-10-20T00:00:00.000Z',
    end: '2026-10-21T00:00:00.000Z',
  },
  {
    zone: 'America/New_York',
    formatted: '2026-10-19 22:56',
    crossover: '2026-10-20 16:30',
    start: '2026-10-20T04:00:00.000Z',
    end: '2026-10-21T04:00:00.000Z',
  },
])('audit local time in $zone', ({ zone, formatted, crossover, start, end }) => {
  beforeEach(() => vi.stubEnv('TZ', zone));
  afterEach(() => vi.unstubAllEnvs());

  it('formats API instants using concrete local-time expectations', () => {
    expect(formatAuditDateTime('2026-10-20T02:56:30Z')).toBe(formatted);
    expect(formatAuditDateTime('2026-10-20T20:30:00Z')).toBe(crossover);
    expect(formatAuditDateTime('2026-10-20T10:56:30+08:00')).toBe(formatted);
  });

  it('uses the same local day for filtering and display', () => {
    expect(
      buildAuditQuery({ page: 1, pageSize: 20, startDate: '2026-10-20', endDate: '2026-10-20' }),
    ).toEqual({
      page: 1,
      page_size: 20,
      start_at: start,
      end_at: end,
    });
  });
});

describe('audit daylight-saving boundaries', () => {
  beforeEach(() => vi.stubEnv('TZ', 'America/New_York'));
  afterEach(() => vi.unstubAllEnvs());

  it.each([
    ['2026-03-08', '2026-03-08T05:00:00.000Z', '2026-03-09T04:00:00.000Z'],
    ['2026-11-01', '2026-11-01T04:00:00.000Z', '2026-11-02T05:00:00.000Z'],
  ])('uses calendar midnights, not a fixed 24 hours, for %s', (date, start, end) => {
    const query = buildAuditQuery({ page: 1, pageSize: 20, startDate: date, endDate: date });
    expect(query.start_at).toBe(start);
    expect(query.end_at).toBe(end);
  });
});

describe('audit history view model', () => {
  it('maps operation types to the labels and colors used by the UI', () => {
    expect(getAuditActionLabel('update')).toBe('修改');
    expect(getAuditActionColor('delete')).toBe('red');
    expect(getAuditActionLabel('custom')).toBe('custom');
    expect(getAuditActionColor('custom')).toBeUndefined();
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

  it.each(['not-a-date', '2026-02-30', '2026-13-01'])(
    'preserves invalid date %s for API validation instead of dropping the filter',
    (date) => {
      expect(buildAuditQuery({ page: 1, pageSize: 20, startDate: date, endDate: date })).toEqual({
        page: 1,
        page_size: 20,
        start_date: date,
        end_date: date,
      });
    },
  );

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
