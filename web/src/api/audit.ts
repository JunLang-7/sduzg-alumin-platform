import { request } from './http';
import type { PageResult } from '../types/common';
import type { AuditOperation, AuditQuery } from '../types/audit';

export const auditApi = {
  list(params: AuditQuery) {
    return request<PageResult<AuditOperation>>({
      method: 'GET',
      url: '/admin/audit/operations',
      params,
    });
  },

  detail(id: number) {
    return request<AuditOperation>({
      method: 'GET',
      url: `/admin/audit/operations/${id}`,
    });
  },
};
