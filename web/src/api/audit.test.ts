import { describe, expect, it, vi } from 'vitest';
import { auditApi } from './audit';
import { request } from './http';

vi.mock('./http', () => ({ request: vi.fn() }));

describe('audit detail requests', () => {
  it('scopes a request from an alumni record to the selected profile', async () => {
    await auditApi.detail(12, 34);
    expect(request).toHaveBeenLastCalledWith({
      method: 'GET',
      url: '/admin/audit/operations/12',
      params: { target_id: 34 },
    });
  });

  it('keeps a request from global history unscoped', async () => {
    await auditApi.detail(12);
    expect(request).toHaveBeenLastCalledWith({
      method: 'GET',
      url: '/admin/audit/operations/12',
      params: undefined,
    });
  });
});
