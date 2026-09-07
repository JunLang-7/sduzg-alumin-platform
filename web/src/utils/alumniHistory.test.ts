import { describe, expect, it } from 'vitest';
import { buildAlumniHistoryQuery } from './auditHistory';

describe('alumni operation history query', () => {
  it('loads the first page of records for the selected alumni profile', () => {
    expect(buildAlumniHistoryQuery(42)).toEqual({
      page: 1,
      page_size: 100,
      target_id: 42,
    });
  });
});
